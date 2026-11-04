package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/txn"
)

// Legacy category separator renames use verified receipts and native file IDs.
// For converted JXL, both journal paths are migrated with the native file and
// receipt, preserving the original-format restore path.
const booruFilenameMigrationVersion = "category-separator-v1"
const booruFilenameMigrationMaxReceipts = 2000

var booruFilenameMigrationMu sync.Mutex

type booruFilenameMigrationItem struct {
	Receipt  string   `json:"receipt"`
	OldPath  string   `json:"oldPath"`
	NewPath  string   `json:"newPath,omitempty"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type booruFilenameMigrationReport struct {
	Version           string                       `json:"version"`
	Dismissed         bool                         `json:"dismissed"`
	Eligible          int                          `json:"eligible"`
	ConvertedEligible int                          `json:"convertedEligible"`
	Review            int                          `json:"review"`
	Truncated         bool                         `json:"truncated"`
	Items             []booruFilenameMigrationItem `json:"items"`
}

type booruFilenameMigrationRequest struct {
	Action            string   `json:"action"`
	Receipts          []string `json:"receipts,omitempty"`
	AcceptHistoryMigration bool     `json:"acceptHistoryMigration,omitempty"`
}

func (rs booruBrowserRoutes) filenameMigrationDismissed() bool {
	data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-filename-migration-v1.json"))
	if err != nil || len(data) > 512 {
		return false
	}
	var state struct {
		Version   string `json:"version"`
		Dismissed bool   `json:"dismissed"`
	}
	return json.Unmarshal(data, &state) == nil && state.Version == booruFilenameMigrationVersion && state.Dismissed
}

func validBooruMigrationReceiptName(name string) bool {
	return name != "" && name == filepath.Base(name) &&
		!strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".json") &&
		!strings.ContainsAny(name, "/\\") && len(name) <= 180
}

func (rs booruBrowserRoutes) readFilenameMigrationReceipt(name string) (*booruDownloadReceipt, error) {
	if !validBooruMigrationReceiptName(name) {
		return nil, fmt.Errorf("invalid receipt name")
	}
	path := filepath.Join(rs.configPath, "booru-download-history", name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, fmt.Errorf("invalid receipt file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var receipt booruDownloadReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	if receipt.Source.Instance == "" || receipt.Post.ID == "" ||
		receiptName(receipt.Source.Instance, receipt.Post.ID) != name {
		return nil, fmt.Errorf("receipt identity mismatch")
	}
	return &receipt, nil
}

func legacyCategoryWarning(tags []booru.Tag) []string {
	var warnings []string
	for _, tag := range tags {
		if tag.Category == "character" && strings.EqualFold(tag.Name, "animated") {
			warnings = append(warnings, "Provider recorded animated as a Character; inspect the media's assigned entities")
			break
		}
	}
	return warnings
}

// The receipt describes what the provider sent, not necessarily the entities
// the scanner assigned. Inspect the linked native Image or Scene explicitly;
// the audit is read-only and never recategorizes a Character automatically.
func (rs booruBrowserRoutes) auditFilenameAssignedCharacters(item *booruFilenameMigrationItem, receipt booruDownloadReceipt) {
	if item == nil || len(item.Warnings) == 0 {
		return
	}
	suspicious := false
	for _, tag := range receipt.Post.Tags {
		if tag.Category == "character" && strings.EqualFold(tag.Name, "animated") {
			suspicious = true
			break
		}
	}
	if !suspicious {
		return
	}
	if receipt.Library == nil || receipt.Library.ID == "" || rs.repository.TxnManager == nil ||
		rs.repository.Image == nil || rs.repository.Scene == nil || rs.repository.Performer == nil {
		item.Warnings = append(item.Warnings, "Native media assignments could not be inspected; missing linked library identity")
		return
	}
	id, err := strconv.Atoi(receipt.Library.ID)
	if err != nil || id <= 0 {
		item.Warnings = append(item.Warnings, "Native media assignments could not be inspected; invalid linked media ID")
		return
	}
	matchedIDs := []int{}
	err = txn.WithReadTxn(context.Background(), rs.repository.TxnManager, func(ctx context.Context) error {
		var performerIDs []int
		switch receipt.Library.Kind {
		case "image":
			media, err := rs.repository.Image.Find(ctx, id)
			if err != nil {
				return err
			}
			if media == nil {
				return fmt.Errorf("linked image not found")
			}
			if err := media.LoadPerformerIDs(ctx, rs.repository.Image); err != nil {
				return err
			}
			performerIDs = media.PerformerIDs.List()
		case "scene":
			media, err := rs.repository.Scene.Find(ctx, id)
			if err != nil {
				return err
			}
			if media == nil {
				return fmt.Errorf("linked scene not found")
			}
			if err := media.LoadPerformerIDs(ctx, rs.repository.Scene); err != nil {
				return err
			}
			performerIDs = media.PerformerIDs.List()
		default:
			return fmt.Errorf("unsupported linked media kind")
		}
		for _, performerID := range performerIDs {
			character, err := rs.repository.Performer.Find(ctx, performerID)
			if err != nil {
				return err
			}
			if character != nil && strings.EqualFold(strings.TrimSpace(character.Name), "animated") {
				matchedIDs = append(matchedIDs, performerID)
			}
		}
		return nil
	})
	if err != nil {
		item.Warnings = append(item.Warnings, "Native Character assignments could not be verified: "+err.Error())
		return
	}
	if len(matchedIDs) == 0 {
		item.Warnings = append(item.Warnings, "No Character named animated is currently assigned to this media; the warning comes from the provider snapshot")
		return
	}
	for _, id := range matchedIDs {
		item.Warnings = append(item.Warnings, fmt.Sprintf("Linked Character animated (ID %d) is actually assigned to this media; review manually", id))
	}
}

// A plan is calculated from the per-download saved template and tag snapshot,
// not the user's current filename settings. Only exact legacy-path matches are
// candidates. This also avoids renaming arbitrary files merely containing a comma.
// convertedBooruFilenamePreview only predicts an active converted filename if the
// converter's old basename and eight-hex-ID suffix can be verified exactly.
// It never edits an active conversion or its restoration journal.
func convertedBooruFilenamePreview(oldOriginal, newOriginal, activePath string) (string, bool) {
	if filepath.Dir(oldOriginal) != filepath.Dir(activePath) ||
		filepath.Dir(newOriginal) != filepath.Dir(activePath) {
		return "", false
	}
	active := filepath.Base(activePath)
	index := strings.LastIndex(active, ".converted-")
	if index < 1 {
		return "", false
	}
	suffix := active[index:]
	if len(suffix) < len(".converted-")+8+2 {
		return "", false
	}
	idAndExt := strings.TrimPrefix(suffix, ".converted-")
	dot := strings.IndexByte(idAndExt, '.')
	if dot != 8 || dot == len(idAndExt)-1 {
		return "", false
	}
	for _, c := range idAndExt[:8] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return "", false
		}
	}
	oldStem := strings.TrimSuffix(filepath.Base(oldOriginal), filepath.Ext(oldOriginal))
	newStem := strings.TrimSuffix(filepath.Base(newOriginal), filepath.Ext(newOriginal))
	// A truncated converter basename cannot be verified from the receipt alone.
	// The actual conversion journal must be consulted to safely propose that case.
	maxStem := 255 - len(suffix)
	if maxStem <= 0 || len(oldStem) > maxStem || len(newStem) > maxStem ||
		active[:index] != oldStem {
		return "", false
	}
	return filepath.Join(filepath.Dir(activePath), newStem+suffix), true
}

func planBooruFilenameMigration(name string, receipt booruDownloadReceipt) *booruFilenameMigrationItem {
	item := &booruFilenameMigrationItem{
		Receipt:  name,
		OldPath:  receipt.SavedPath,
		Warnings: legacyCategoryWarning(receipt.Post.Tags),
		Status:   "audit",
	}
	if receipt.Library != nil && receipt.Library.Path != "" {
		item.OldPath = receipt.Library.Path
	}
	if receipt.LegacyFilenameMigrated {
		if len(item.Warnings) == 0 {
			return nil
		}
		item.Reason = "Filename already migrated; metadata review only"
		return item
	}
	hasMulti := false
	for _, category := range []string{"artist", "copyright", "character"} {
		if !strings.Contains(receipt.Template, "%"+category+"%") {
			continue
		}
		count := 0
		for _, tag := range receipt.Post.Tags {
			if tag.Category == category {
				count++
			}
		}
		hasMulti = hasMulti || count > 1
	}
	if !hasMulti || receipt.Template == "" || receipt.SavedPath == "" ||
		receipt.Directory == "" || receipt.OriginalMD5 == "" {
		if len(item.Warnings) == 0 {
			return nil
		}
		item.Reason = "Metadata audit only; no filename migration needed"
		return item
	}
	ext := strings.TrimPrefix(filepath.Ext(receipt.SavedPath), ".")
	source := booru.Source{
		ID: receipt.Source.ID, Name: receipt.Source.Name,
		Engine: receipt.Source.Engine, URL: receipt.Source.URL,
	}
	var changedPath string
	matched := false
	for n := 0; n < 100; n++ {
		oldRel, err := booru.RenderLegacyFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, ext, n)
		if err != nil {
			break
		}
		newRel, err := booru.RenderFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, ext, n)
		if err != nil {
			break
		}
		if n > 0 && !strings.Contains(receipt.Template, "%num%") {
			oldRel = strings.TrimSuffix(oldRel, "."+ext) + fmt.Sprintf("_%d.%s", n, ext)
			newRel = strings.TrimSuffix(newRel, "."+ext) + fmt.Sprintf("_%d.%s", n, ext)
		}
		expectedOld := filepath.Join(receipt.Directory, filepath.FromSlash(oldRel))
		if filepath.Clean(expectedOld) == filepath.Clean(receipt.SavedPath) {
			changedPath = filepath.Join(receipt.Directory, filepath.FromSlash(newRel))
			matched = true
			break
		}
	}
	if !matched {
		item.Status = "review"
		item.Reason = "Saved filename does not match the original template and tag snapshot"
		return item
	}
	if changedPath == receipt.SavedPath {
		if len(item.Warnings) == 0 {
			return nil
		}
		item.Reason = "No separator change; metadata review only"
		return item
	}
	item.NewPath = changedPath
	if filepath.Dir(changedPath) != filepath.Dir(receipt.SavedPath) {
		item.Status = "review"
		item.Reason = "Filename change would move a directory; manual review required"
		return item
	}
	if receipt.Library == nil || receipt.Library.Path == "" {
		item.Status = "review"
		item.Reason = "Native library file is not registered"
		return item
	}
	if receipt.Library.Converted || receipt.Library.Path != receipt.SavedPath {
		item.Status = "review"
		item.NewPath = "" // Never propose an original path for any active converted output.
		if receipt.Library.Converted {
			if convertedPath, ok := convertedBooruFilenamePreview(receipt.SavedPath, changedPath, receipt.Library.Path); ok {
				item.NewPath = convertedPath
			}
		}
		item.Reason = "Converted or restored media: conversion history must be migrated together"
		return item
	}
	if receipt.OutputMD5 == "" {
		item.Status = "review"
		item.Reason = "Saved download has no output checksum for byte verification"
		return item
	}
	if stat, err := os.Lstat(receipt.Library.Path); err != nil || !stat.Mode().IsRegular() {
		item.Status, item.Reason = "review", "Saved file is missing or not a regular file"
		return item
	}
	if _, err := os.Lstat(changedPath); err == nil {
		item.Status, item.Reason = "review", "Destination already exists; never overwrite"
		return item
	} else if !errors.Is(err, os.ErrNotExist) {
		item.Status, item.Reason = "review", "Destination could not be checked"
		return item
	}
	item.Status = "ready"
	return item
}

// Conversion and upscaling restore journals retain original paths even when
// their active file has been switched back. Renaming one of those paths would
// invalidate a future ToggleRestore, so they are always review-only.
func filenameMigrationRestorationPaths(configPath string) (map[string]bool, error) {
	paths := make(map[string]bool)
	for _, directory := range []string{"media-conversions", "media-upscaling-restores"} {
		store := mediaconvert.Store{Root: filepath.Join(configPath, directory)}
		history, err := store.History()
		if err != nil {
			return nil, fmt.Errorf("reading %s restore history: %w", directory, err)
		}
		for _, record := range history {
			for _, snapshot := range []mediaconvert.Snapshot{record.Before, record.After} {
				if media := snapshot.File(); media != nil && media.Base().Path != "" {
					paths[filepath.Clean(media.Base().Path)] = true
				}
			}
		}
	}
	return paths, nil
}

func checkFilenameMigrationRestorationRisk(item *booruFilenameMigrationItem, receipt booruDownloadReceipt, paths map[string]bool, historyErr error) {
	if item == nil || item.Status != "ready" {
		return
	}
	switch {
	case historyErr != nil:
		item.Status = "review"
		item.Reason = "Saved conversion history could not be verified; rename blocked"
	case paths[filepath.Clean(receipt.SavedPath)] || paths[filepath.Clean(receipt.Library.Path)]:
		item.Status = "review"
		item.Reason = "Conversion or upscaling restore history references this file; rename would break recovery"
	}
}

// Converted output renames require a unique, completed conversion journal. The original
// and active output paths will be rewritten together, preserving restoration.
func filenameMigrationActiveConvertedRecords(configPath string) (map[string]*mediaconvert.Record, error) {
	records := make(map[string]*mediaconvert.Record)
	references := make(map[string]int)
	for _, directory := range []string{"media-conversions", "media-upscaling-restores"} {
		store := mediaconvert.Store{Root: filepath.Join(configPath, directory)}
		history, err := store.History()
		if err != nil {
			return nil, fmt.Errorf("reading %s conversion history: %w", directory, err)
		}
		for _, record := range history {
			before, after := record.Before.File(), record.After.File()
			if before != nil {
				references[filepath.Clean(before.Base().Path)]++
			}
			if after != nil {
				references[filepath.Clean(after.Base().Path)]++
				if directory == "media-conversions" && (record.Status == "complete" || record.Status == "restored") &&
					filepath.Ext(after.Base().Path) != "" && strings.Contains(filepath.Base(after.Base().Path), ".converted-") {
					path := filepath.Clean(after.Base().Path)
					records[path] = record
					// Restored media may have refreshed download receipts that
					// already point back to the original (Before) path.
					if record.Status == "restored" && before != nil {
						records[filepath.Clean(before.Base().Path)] = record
					}
				}
			}
		}
	}
	for path, record := range records {
		if references[path] != 1 || references[filepath.Clean(record.Before.File().Base().Path)] != 1 {
			delete(records, path)
		}
	}
	return records, nil
}

// Converted files use a distinct action requiring the conversion journal to
// be migrated with the active file; regular renames never bypass this guard.
func checkFilenameMigrationConvertedEligibility(item *booruFilenameMigrationItem, receipt booruDownloadReceipt, history map[string]bool, records map[string]*mediaconvert.Record, historyErr error) {
	if item == nil || receipt.Library == nil || item.Status != "review" || item.NewPath == "" ||
		historyErr != nil || receipt.OriginalMD5 == "" || receipt.OutputMD5 == "" {
		return
	}
	record := records[filepath.Clean(receipt.Library.Path)]
	if record == nil || record.Before.File() == nil || record.After.File() == nil ||
		(record.Status != "complete" && record.Status != "restored") || len(record.ID) < 8 {
		return
	}
	before, after := record.Before.File().Base(), record.After.File().Base()
	oldOriginal, oldOutput := filepath.Clean(before.Path), filepath.Clean(after.Path)
	restored := record.Status == "restored"
	receiptRefreshed := !receipt.Library.Converted && restored &&
		filepath.Clean(receipt.Library.Path) == oldOriginal
	if receipt.Library.Converted {
		if filepath.Clean(receipt.Library.Path) != oldOutput {
			return
		}
	} else if !receiptRefreshed {
		return
	}
	if filepath.Clean(receipt.SavedPath) != oldOriginal ||
		before.Fingerprints.GetString("md5") != receipt.OriginalMD5 ||
		(!receiptRefreshed && after.Fingerprints.GetString("md5") != receipt.OutputMD5) ||
		(receiptRefreshed && receipt.OutputMD5 != receipt.OriginalMD5) ||
		!strings.HasSuffix(strings.ToLower(filepath.Base(oldOutput)), ".converted-"+record.ID[:8]+strings.ToLower(filepath.Ext(oldOutput))) {
		return
	}

	newOriginal, newOutput := "", ""
	if receiptRefreshed {
		newOriginal = item.NewPath
		var ok bool
		newOutput, ok = convertedBooruFilenamePreview(oldOriginal, newOriginal, oldOutput)
		if !ok {
			return
		}
	} else {
		newOutput = item.NewPath
		var err error
		newOriginal, err = convertedFilenameOriginalTarget(oldOriginal, oldOutput, newOutput)
		if err != nil {
			return
		}
	}
	if filepath.Dir(oldOriginal) != filepath.Dir(oldOutput) ||
		filepath.Dir(newOriginal) != filepath.Dir(oldOriginal) ||
		filepath.Dir(newOutput) != filepath.Dir(oldOutput) ||
		!strings.EqualFold(filepath.Ext(newOriginal), filepath.Ext(oldOriginal)) ||
		!strings.EqualFold(filepath.Ext(newOutput), filepath.Ext(oldOutput)) ||
		newOriginal == newOutput || history[filepath.Clean(newOriginal)] ||
		history[filepath.Clean(newOutput)] {
		return
	}
	source, target := oldOutput, newOutput
	if restored {
		source, target = oldOriginal, newOriginal
		// The inactive converted path must not be occupied by a stale file.
		if _, err := os.Lstat(oldOutput); !errors.Is(err, os.ErrNotExist) {
			item.Reason = "Restored conversion has an occupied old output path"
			return
		}
	}
	if stat, err := os.Lstat(source); err != nil || !stat.Mode().IsRegular() {
		item.Reason = "Converted or restored file is missing or is not regular"
		return
	}
	for _, candidate := range []string{newOriginal, newOutput} {
		if _, err := os.Lstat(candidate); err == nil {
			item.Reason = "Destination already exists; never overwrite"
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			item.Reason = "Destination could not be checked"
			return
		}
	}
	item.OldPath, item.NewPath = source, target
	item.Status = "ready-converted"
	if restored {
		item.Reason = "Restored original: rename both conversion history paths while preserving reapply"
	} else {
		item.Reason = "Converted media: original and output history paths will be migrated together"
	}
}

func (rs booruBrowserRoutes) filenameMigrationReport() (booruFilenameMigrationReport, error) {
	report := booruFilenameMigrationReport{
		Version: booruFilenameMigrationVersion, Dismissed: rs.filenameMigrationDismissed(),
		Items: []booruFilenameMigrationItem{},
	}
	dir := filepath.Join(rs.configPath, "booru-download-history")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	restorationPaths, historyErr := filenameMigrationRestorationPaths(rs.configPath)
	convertedRecords, convertedErr := filenameMigrationActiveConvertedRecords(rs.configPath)
	if historyErr == nil {
		historyErr = convertedErr
	}
	for i, entry := range entries {
		if i >= booruFilenameMigrationMaxReceipts {
			report.Truncated = true
			break
		}
		if entry.IsDir() || !validBooruMigrationReceiptName(entry.Name()) {
			continue
		}
		receipt, err := rs.readFilenameMigrationReceipt(entry.Name())
		if err != nil {
			continue
		}
		item := planBooruFilenameMigration(entry.Name(), *receipt)
		if item == nil {
			continue
		}
		rs.auditFilenameAssignedCharacters(item, *receipt)
		checkFilenameMigrationRestorationRisk(item, *receipt, restorationPaths, historyErr)
		checkFilenameMigrationConvertedEligibility(item, *receipt, restorationPaths, convertedRecords, historyErr)
		report.Items = append(report.Items, *item)
		switch item.Status {
		case "ready":
			report.Eligible++
		case "ready-converted":
			report.ConvertedEligible++
		default:
			report.Review++
		}
	}
	return report, nil
}

func (rs booruBrowserRoutes) FilenameMigration(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	booruFilenameMigrationMu.Lock()
	defer booruFilenameMigrationMu.Unlock()
	report, err := rs.filenameMigrationReport()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "migration", "Cannot inspect saved download receipts.")
		return
	}
	writeVisualSimilarityJSON(w, report)
}

// convertedFilenameOriginalTarget recovers the proposed original-format name
// from the verified converted output suffix; no extension is guessed.
func convertedFilenameOriginalTarget(oldOriginal, oldOutput, newOutput string) (string, error) {
	oldStem := strings.TrimSuffix(filepath.Base(oldOriginal), filepath.Ext(oldOriginal))
	suffix := strings.TrimPrefix(filepath.Base(oldOutput), oldStem)
	if oldStem == "" || !strings.HasPrefix(suffix, ".converted-") ||
		!strings.HasSuffix(filepath.Base(newOutput), suffix) {
		return "", fmt.Errorf("converted filename suffix does not match the receipt")
	}
	newStem := strings.TrimSuffix(filepath.Base(newOutput), suffix)
	if newStem == "" || filepath.Dir(oldOriginal) != filepath.Dir(newOutput) {
		return "", fmt.Errorf("invalid proposed original-format filename")
	}
	return filepath.Join(filepath.Dir(oldOriginal), newStem+filepath.Ext(oldOriginal)), nil
}

func (rs booruBrowserRoutes) applyBooruFilenameMigration(ctx context.Context, name string, restorationPaths map[string]bool, convertedRecords map[string]*mediaconvert.Record, allowConverted bool) error {
	receipt, err := rs.readFilenameMigrationReceipt(name)
	if err != nil {
		return err
	}
	plan := planBooruFilenameMigration(name, *receipt)
	if plan == nil {
		return fmt.Errorf("download is not eligible for filename migration")
	}
	checkFilenameMigrationRestorationRisk(plan, *receipt, restorationPaths, nil)
	checkFilenameMigrationConvertedEligibility(plan, *receipt, restorationPaths, convertedRecords, nil)
	if allowConverted {
		if plan.Status != "ready-converted" {
			return fmt.Errorf("download is not eligible for coordinated conversion history migration")
		}
	} else if plan.Status != "ready" {
		return fmt.Errorf("download is not eligible for safe filename migration")
	}
	if rs.repository.TxnManager == nil || rs.libraryRoots == nil {
		return fmt.Errorf("native library is unavailable")
	}

	original := *receipt
	originalLibrary := *receipt.Library
	original.Library = &originalLibrary // Keep a separate copy for receipt and journal rollback.

	oldPath, newPath := receipt.Library.Path, plan.NewPath
	expectedMD5 := receipt.OutputMD5
	var record *mediaconvert.Record
	restored := false
	if allowConverted {
		record = convertedRecords[filepath.Clean(receipt.Library.Path)]
		if record == nil {
			return fmt.Errorf("conversion journal was not uniquely verified")
		}
		restored = record.Status == "restored"
		if restored {
			// Whether or not the download receipt was refreshed, native restore
			// places the original-format bytes at the Before path.
			oldPath = receipt.SavedPath
			expectedMD5 = receipt.OriginalMD5
		}
	}
	oldDir := filepath.Dir(oldPath)
	valid, err := rs.destination(oldDir)
	if err != nil || valid != oldDir {
		return fmt.Errorf("download is outside a selected library root")
	}
	if filepath.Dir(newPath) != oldDir || filepath.Base(newPath) == "." {
		return fmt.Errorf("migration cannot change folder layout")
	}
	md5, err := mediaconvert.MD5(oldPath)
	if err != nil || md5 != expectedMD5 {
		return fmt.Errorf("media checksum changed since the conversion snapshot; skipping")
	}
	if _, err := os.Lstat(newPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("target is occupied or cannot be inspected")
	}

	newOriginal, newOutput := newPath, newPath
	journalID := ""
	if allowConverted {
		journalID = record.ID
		if restored {
			var ok bool
			newOutput, ok = convertedBooruFilenamePreview(receipt.SavedPath, newPath, record.After.File().Base().Path)
			if !ok {
				return fmt.Errorf("converted restore path cannot be reconstructed exactly")
			}
		} else {
			newOriginal, err = convertedFilenameOriginalTarget(receipt.SavedPath, oldPath, newPath)
			if err != nil {
				return err
			}
		}
		for _, candidate := range []string{newOriginal, newOutput} {
			if filepath.Dir(candidate) != oldDir {
				return fmt.Errorf("conversion history cannot change folder layout")
			}
			if candidate != newPath {
				if _, err := os.Lstat(candidate); !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("conversion restore target is occupied or cannot be inspected")
				}
			}
			if restorationPaths[filepath.Clean(candidate)] {
				return fmt.Errorf("conversion restore target is referenced by another journal")
			}
		}
	}

	receiptUpdated := false
	journalUpdated := false
	err = rs.repository.WithTxn(ctx, func(ctx context.Context) error {
		existing, err := rs.repository.File.FindByPath(ctx, oldPath, true)
		if err != nil || existing == nil {
			return fmt.Errorf("cannot locate the registered native file: %v", err)
		}
		if allowConverted && existing.Base().ID != record.Before.File().Base().ID {
			return fmt.Errorf("registered media identity does not match the conversion journal")
		}
		folder, err := rs.repository.Folder.Find(ctx, existing.Base().ParentFolderID)
		if err != nil || folder == nil || folder.Path != oldDir {
			return fmt.Errorf("cannot locate registered parent folder")
		}
		mover := file.NewMover(rs.repository.File, rs.repository.Folder, rs.libraryRoots())
		mover.RegisterHooks(ctx)
		if err := mover.Move(ctx, existing, folder, filepath.Base(newPath)); err != nil {
			return err
		}
		if allowConverted {
			store := mediaconvert.Store{Root: filepath.Join(rs.configPath, "media-conversions")}
			if err := store.RenameRestorablePaths(journalID, original.SavedPath, newOriginal, record.After.File().Base().Path, newOutput); err != nil {
				return fmt.Errorf("cannot migrate conversion history: %w", err)
			}
			journalUpdated = true
		}
		receipt.SavedPath = newOriginal
		receipt.Library.Path = newPath
		if restored {
			receipt.Library.Converted = false
			receipt.OutputMD5 = receipt.OriginalMD5
		}
		receipt.LegacyFilenameMigrated = true
		if err := rs.saveDownloadReceipt(*receipt); err != nil {
			return err
		}
		receiptUpdated = true
		return nil
	})
	if err != nil {
		if journalUpdated {
			store := mediaconvert.Store{Root: filepath.Join(rs.configPath, "media-conversions")}
			if rollbackErr := store.RenameRestorablePaths(journalID, newOriginal, original.SavedPath, newOutput, record.After.File().Base().Path); rollbackErr != nil {
				return fmt.Errorf("%w; conversion history rollback failed: %v", err, rollbackErr)
			}
		}
		if receiptUpdated {
			if recoveryErr := rs.saveDownloadReceipt(original); recoveryErr != nil {
				return fmt.Errorf("%w; receipt rollback failed: %v", err, recoveryErr)
			}
		}
	}
	return err
}

func (rs booruBrowserRoutes) UpdateFilenameMigration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || !booruSameOrigin(r) {
		writeBooruError(w, http.StatusForbidden, "migration", "Migration requires a same-origin JSON request.")
		return
	}
	var input booruFilenameMigrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
		writeBooruError(w, http.StatusBadRequest, "migration", "Invalid migration request.")
		return
	}
	booruFilenameMigrationMu.Lock()
	defer booruFilenameMigrationMu.Unlock()
	switch input.Action {
	case "dismiss":
		data, err := json.Marshal(struct {
			Version   string `json:"version"`
			Dismissed bool   `json:"dismissed"`
		}{Version: booruFilenameMigrationVersion, Dismissed: true})
		if err != nil {
			writeBooruError(w, 500, "migration", "Cannot encode migration preference.")
			return
		}
		if err := writeBooruAtomic(rs.configPath, "booru-filename-migration-v1.json", data); err != nil {
			writeBooruError(w, 500, "migration", "Cannot save migration preference.")
			return
		}
		writeVisualSimilarityJSON(w, map[string]bool{"dismissed": true})
	case "apply", "apply-converted", "apply-jxl":
		if input.Action != "apply" && !input.AcceptHistoryMigration {
			writeBooruError(w, http.StatusBadRequest, "migration", "Confirm the coordinated converted-media filename and conversion-history migration.")
			return
		}
		if len(input.Receipts) == 0 || len(input.Receipts) > 20 {
			writeBooruError(w, 400, "migration", "Choose between 1 and 20 verified receipts per batch.")
			return
		}
		// Serialize against conversions while checking journals and renaming.
		conversionMutations.Lock()
		defer conversionMutations.Unlock()
		restorationPaths, err := filenameMigrationRestorationPaths(rs.configPath)
		if err != nil {
			writeBooruError(w, http.StatusConflict, "migration", "Cannot verify saved conversion history; no files renamed.")
			return
		}
		convertedRecords, err := filenameMigrationActiveConvertedRecords(rs.configPath)
		if err != nil {
			writeBooruError(w, http.StatusConflict, "migration", "Cannot verify saved conversion journal; no files renamed.")
			return
		}
		applied := []string{}
		failures := map[string]string{}
		for _, name := range input.Receipts {
			if !validBooruMigrationReceiptName(name) {
				writeBooruError(w, 400, "migration", "Invalid receipt identifier.")
				return
			}
			if err := rs.applyBooruFilenameMigration(r.Context(), name, restorationPaths, convertedRecords, input.Action != "apply"); err != nil {
				failures[name] = err.Error()
			} else {
				applied = append(applied, name)
			}
		}
		writeVisualSimilarityJSON(w, map[string]interface{}{"applied": applied, "failures": failures})
	default:
		writeBooruError(w, 400, "migration", "Unknown migration action.")
	}
}
