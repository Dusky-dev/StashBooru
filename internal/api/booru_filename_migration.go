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

// The v1 upgrader handles verified same-folder, non-converted moves only.
// Converted assets need coordinated updates of the restoration journal; never
// offer a destructive file rename without that guarantee.
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
	Review    int                          `json:"review"`
	Truncated bool                         `json:"truncated"`
	Items     []booruFilenameMigrationItem `json:"items"`
}

type booruFilenameMigrationRequest struct {
	Action   string   `json:"action"`
	Receipts []string `json:"receipts,omitempty"`
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
func planBooruFilenameMigration(name string, receipt booruDownloadReceipt) *booruFilenameMigrationItem {
	item := &booruFilenameMigrationItem{
		Receipt: name, OldPath: receipt.SavedPath,
		Warnings: legacyCategoryWarning(receipt.Post.Tags),
		Status: "audit",
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
		item.Reason = "Converted or restored media: conversion history must be migrated together"
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
		report.Items = append(report.Items, *item)
		if item.Status == "ready" {
			report.Eligible++
		} else {
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

func (rs booruBrowserRoutes) applyBooruFilenameMigration(ctx context.Context, name string) error {
	receipt, err := rs.readFilenameMigrationReceipt(name)
	if err != nil {
		return err
	}
	plan := planBooruFilenameMigration(name, *receipt)
	if plan == nil || plan.Status != "ready" {
		return fmt.Errorf("download is not eligible for safe filename migration")
	}
	if rs.repository == nil || rs.libraryRoots == nil {
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
			return fmt.Errorf("cannot locate the registered native file: %w", err)
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
		data, _ := json.Marshal(map[string]interface{}{"version": booruFilenameMigrationVersion, "dismissed": true})
		if err := writeBooruAtomic(rs.configPath, "booru-filename-migration-v1.json", data); err != nil {
			writeBooruError(w, 500, "migration", "Cannot save migration preference.")
			return
		}
		writeVisualSimilarityJSON(w, map[string]bool{"dismissed": true})
	case "apply":
		if len(input.Receipts) == 0 || len(input.Receipts) > 20 {
			writeBooruError(w, 400, "migration", "Choose between 1 and 20 verified receipts per batch.")
			return
		}
		applied := []string{}
		failures := map[string]string{}
		for _, name := range input.Receipts {
			if !validBooruMigrationReceiptName(name) {
				writeBooruError(w, 400, "migration", "Invalid receipt identifier.")
				return
			}
			if err := rs.applyBooruFilenameMigration(r.Context(), name); err != nil {
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
