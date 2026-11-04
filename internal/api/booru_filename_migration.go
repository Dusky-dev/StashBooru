package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

// The v1 upgrader handles verified same-folder, non-converted moves. Active
// converted JXL can be renamed only with a separate explicit acknowledgement:
// its original conversion journal is intentionally not rewritten and future
// conversion restoration will no longer work.
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
	Version   string                       `json:"version"`
	Dismissed bool                         `json:"dismissed"`
	Eligible  int                          `json:"eligible"`
	ConvertedEligible int                 `json:"convertedEligible"`
	Review    int                          `json:"review"`
	Truncated bool                         `json:"truncated"`
	Items     []booruFilenameMigrationItem `json:"items"`
}

type booruFilenameMigrationRequest struct {
	Action   string   `json:"action"`
	Receipts []string `json:"receipts,omitempty"`
	AcceptRestoreLoss bool `json:"acceptRestoreLoss,omitempty"`
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

// A plan is calculated from the per-download saved template and tag snapshot,
// not the user's current filename settings. Only exact legacy-path matches are
// candidates. This also avoids renaming arbitrary files merely containing a comma.
// convertedBooruFilenamePreview only predicts the active JXL filename if the
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
	if receipt.Library.Converted || receipt.Convert || receipt.Library.Path != receipt.SavedPath {
		item.Status = "review"
		item.NewPath = "" // Never propose the original .jpg path for an active .jxl.
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

// A JXL rename does not re-encode media. The existing conversion journal
// remains intact, but it will not be able to restore by its original output
// path. Only an unambiguous, completed conversion for this exact native file
// can be offered under a separate irreversible-restore confirmation.
func filenameMigrationActiveJXLRecords(configPath string) (map[string]*mediaconvert.Record, error) {
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
			for _, media := range []interface{ Base() *models.BaseFile }{} {
				_ = media
			}
			if before != nil {
				references[filepath.Clean(before.Base().Path)]++
			}
			if after != nil {
				references[filepath.Clean(after.Base().Path)]++
				if directory == "media-conversions" && record.Status == "complete" &&
					strings.EqualFold(filepath.Ext(after.Base().Path), ".jxl") {
					path := filepath.Clean(after.Base().Path)
					records[path] = record
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

// Converted files are never enabled by the regular safe-rename path. Their
// separate action acknowledges that the untouched conversion journal loses
// its restore capability after the move.
func checkFilenameMigrationJXLEligibility(item *booruFilenameMigrationItem, receipt booruDownloadReceipt, history map[string]bool, records map[string]*mediaconvert.Record, historyErr error) {
	if item == nil || receipt.Library == nil || !receipt.Library.Converted ||
		!strings.EqualFold(filepath.Ext(receipt.Library.Path), ".jxl") ||
		item.Status != "review" || item.NewPath == "" {
		return
	}
	if historyErr != nil || receipt.OutputMD5 == "" || !strings.EqualFold(filepath.Ext(item.NewPath), ".jxl") {
		return
	}
	oldPath := filepath.Clean(receipt.Library.Path)
	record := records[oldPath]
	if record == nil || record.Before.File() == nil || record.After.File() == nil ||
		record.Status != "complete" ||
		filepath.Clean(record.Before.File().Base().Path) != filepath.Clean(receipt.SavedPath) ||
		filepath.Clean(record.After.File().Base().Path) != oldPath ||
		record.Before.File().Base().Fingerprints.GetString("md5") != receipt.OriginalMD5 ||
		record.After.File().Base().Fingerprints.GetString("md5") != receipt.OutputMD5 ||
		filepath.Dir(item.NewPath) != filepath.Dir(oldPath) ||
		history[filepath.Clean(item.NewPath)] {
		return
	}
	if stat, err := os.Lstat(oldPath); err != nil || !stat.Mode().IsRegular() {
		item.Reason = "Converted file is missing or is not a regular file"
		return
	}
	if _, err := os.Lstat(item.NewPath); err == nil {
		item.Reason = "Destination already exists; never overwrite"
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		item.Reason = "Destination could not be checked"
		return
	}
	item.Status = "ready-converted"
	item.Reason = "JXL rename only: restoring the original through conversion history will no longer work"
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
	convertedRecords, convertedErr := filenameMigrationActiveJXLRecords(rs.configPath)
	if historyErr == nil { historyErr = convertedErr }
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
		checkFilenameMigrationRestorationRisk(item, *receipt, restorationPaths, historyErr)
		checkFilenameMigrationJXLEligibility(item, *receipt, restorationPaths, convertedRecords, historyErr)
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
	checkFilenameMigrationJXLEligibility(plan, *receipt, restorationPaths, convertedRecords, nil)
	if allowConverted {
		if plan.Status != "ready-converted" {
			return fmt.Errorf("download is not an eligible converted JXL")
		}
	} else if plan.Status != "ready" {
		return fmt.Errorf("download is not eligible for safe filename migration")
	}
	if rs.repository.TxnManager == nil || rs.libraryRoots == nil {
		return fmt.Errorf("native library is unavailable")
	}
	oldPath, newPath := receipt.Library.Path, plan.NewPath
	oldDir := filepath.Dir(oldPath)
	valid, err := rs.destination(oldDir)
	if err != nil || valid != oldDir {
		return fmt.Errorf("download is outside a selected library root")
	}
	if filepath.Dir(newPath) != oldDir || filepath.Base(newPath) == "." {
		return fmt.Errorf("migration cannot change folder layout")
	}
	md5, err := mediaconvert.MD5(oldPath)
	if err != nil || md5 != receipt.OutputMD5 {
		return fmt.Errorf("file checksum changed since download; skipping")
	}
	if _, err := os.Lstat(newPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("target is occupied or cannot be inspected")
	}
	original := *receipt
	movedReceipt := false
	err = rs.repository.WithTxn(ctx, func(ctx context.Context) error {
		existing, err := rs.repository.File.FindByPath(ctx, oldPath, true)
		if err != nil || existing == nil {
			return fmt.Errorf("cannot locate the registered native file: %v", err)
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
		receipt.SavedPath = newPath
		receipt.Library.Path = newPath
		if err := rs.saveDownloadReceipt(*receipt); err != nil {
			return err
		}
		movedReceipt = true
		return nil
	})
	if err != nil && movedReceipt {
		if recoveryErr := rs.saveDownloadReceipt(original); recoveryErr != nil {
			return fmt.Errorf("%w; receipt rollback failed: %v", err, recoveryErr)
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
	case "apply", "apply-jxl":
		if input.Action == "apply-jxl" && !input.AcceptRestoreLoss {
			writeBooruError(w, http.StatusBadRequest, "migration", "Confirm that conversion restore will no longer work after renaming JXL files.")
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
		convertedRecords, err := filenameMigrationActiveJXLRecords(rs.configPath)
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
			if err := rs.applyBooruFilenameMigration(r.Context(), name, restorationPaths, convertedRecords, input.Action == "apply-jxl"); err != nil {
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
