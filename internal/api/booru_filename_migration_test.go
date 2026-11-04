package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
)

func filenameMigrationFixture(t *testing.T) (booruDownloadReceipt, string, string) {
	t.Helper()
	dir := t.TempDir()
	hash := strings.Repeat("a", 32)
	source := booru.Source{ID: "fixture", Name: "Fixture", URL: "https://board.example.org"}
	post := booru.Post{ID: "123", Tags: []booru.Tag{
		{Name: "bossy", Category: "artist"}, {Name: "vestartist", Category: "artist"},
		{Name: "chainsaw_man", Category: "copyright"}, {Name: "denji_(chainsaw_man)", Category: "character"},
	}}
	format := "[%artist%](%copyright%).%character%_%md5%.%ext%"
	old, err := booru.RenderLegacyFilename(format, source, post, hash, "jpg", 0)
	if err != nil {
		t.Fatal(err)
	}
	want, err := booru.RenderFilename(format, source, post, hash, "jpg", 0)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, old)
	newPath := filepath.Join(dir, want)
	if err := os.WriteFile(oldPath, []byte("synthetic image fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	receipt := booruDownloadReceipt{
		Source:      booru.PublicSource{ID: source.ID, Name: source.Name, URL: source.URL, Instance: "fixture-source"},
		Post:        post,
		Directory:   dir,
		SavedPath:   oldPath,
		OriginalMD5: hash,
		OutputMD5:   strings.Repeat("b", 32),
		Template:    format,
		Library:     &booruLibraryResult{Kind: "image", Path: oldPath},
	}
	return receipt, oldPath, newPath
}

func TestPlanBooruFilenameMigrationUsesReceiptAndPreservesLayout(t *testing.T) {
	receipt, old, newName := filenameMigrationFixture(t)
	plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
	if plan == nil || plan.Status != "ready" || plan.OldPath != old || plan.NewPath != newName {
		t.Fatalf("expected verified same-folder rename: %+v", plan)
	}
	// A user's subsequent preference cannot affect a saved receipt.
	receipt.Template = "%md5%.%ext%"
	if plan := planBooruFilenameMigration("fixture-source-123.json", receipt); plan != nil {
		t.Fatalf("template without multi-tag categories should not migrate: %+v", plan)
	}
}

func TestPlanBooruFilenameMigrationSkipsConversionAndConflicts(t *testing.T) {
	receipt, _, destination := filenameMigrationFixture(t)
	receipt.Library.Converted = true
	if plan := planBooruFilenameMigration("fixture-source-123.json", receipt); plan == nil || plan.Status != "review" {
		t.Fatalf("converted asset must require review: %+v", plan)
	}
	receipt.Library.Converted = false
	if err := os.WriteFile(destination, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if plan := planBooruFilenameMigration("fixture-source-123.json", receipt); plan == nil || plan.Status != "review" {
		t.Fatalf("occupied target must not be overwritten: %+v", plan)
	}
	receipt.SavedPath = filepath.Join(receipt.Directory, "some-other-layout.jpg")
	if plan := planBooruFilenameMigration("fixture-source-123.json", receipt); plan == nil || plan.Status != "review" {
		t.Fatalf("nonmatching old name must not be guessed: %+v", plan)
	}
}

func TestPlanBooruFilenameMigrationRequiresVerifiedOutputChecksum(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	receipt.OutputMD5 = ""
	plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
	if plan == nil || plan.Status != "review" {
		t.Fatalf("missing checksum cannot be offered as a safe rename: %+v", plan)
	}
}

func TestConvertedBooruFilenamePreviewPreservesJXLSuffix(t *testing.T) {
	receipt, oldPath, newOriginal := filenameMigrationFixture(t)
	receipt.Library.Converted = true
	receipt.Convert = true
	receipt.Library.Path = strings.TrimSuffix(oldPath, ".jpg") + ".converted-170c91a2.jxl"
	plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
	want := strings.TrimSuffix(newOriginal, ".jpg") + ".converted-170c91a2.jxl"
	if plan == nil || plan.Status != "review" || plan.NewPath != want {
		t.Fatalf("converted media must only preview the active JXL destination: %+v; want %q", plan, want)
	}
	if _, ok := convertedBooruFilenamePreview(oldPath, newOriginal,
		filepath.Join(receipt.Directory, "unrelated.converted-170c91a2.jxl")); ok {
		t.Fatal("unrelated converted filenames cannot be inferred")
	}
}

func TestFilenameMigrationOffersAcknowledgedJXLRename(t *testing.T) {
	receipt, oldOriginal, newOriginal := filenameMigrationFixture(t)
	id := strings.Repeat("c", 32)
	active := strings.TrimSuffix(oldOriginal, ".jpg") + ".converted-" + id[:8] + ".jxl"
	wanted := strings.TrimSuffix(newOriginal, ".jpg") + ".converted-" + id[:8] + ".jxl"
	if err := os.WriteFile(active, []byte("synthetic converted bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(oldOriginal); err != nil {
		t.Fatal(err)
	}
	hash, err := mediaconvert.MD5(active)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Library.Converted = true
	receipt.Library.Path = active
	receipt.Convert = true
	receipt.OutputMD5 = hash
	config := t.TempDir()
	rs := booruBrowserRoutes{configPath: config}
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	record := mediaconvert.Record{
		ID: id,
		Status: "complete",
		Before: mediaconvert.Snapshot{Image: &models.ImageFile{
			BaseFile: &models.BaseFile{ID: 12, Path: oldOriginal, Fingerprints: models.Fingerprints{
				{Type: "md5", Fingerprint: receipt.OriginalMD5},
			}},
		}},
		After: mediaconvert.Snapshot{Image: &models.ImageFile{
			BaseFile: &models.BaseFile{ID: 12, Path: active, Fingerprints: models.Fingerprints{
				{Type: "md5", Fingerprint: hash},
			}},
		}},
	}
	root := filepath.Join(config, "media-conversions")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	report, err := rs.filenameMigrationReport()
	if err != nil || report.ConvertedEligible != 1 || report.Eligible != 0 ||
		len(report.Items) != 1 || report.Items[0].Status != "ready-converted" ||
		report.Items[0].NewPath != wanted {
		t.Fatalf("matching completed JXL needs explicit restore-loss choice: %+v: %v", report, err)
	}
	// A checksum mismatch must never present a converted rename action.
	receipt.OutputMD5 = strings.Repeat("b", 32)
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	report, err = rs.filenameMigrationReport()
	if err != nil || report.ConvertedEligible != 0 || len(report.Items) != 1 ||
		report.Items[0].Status != "review" {
		t.Fatalf("unverified converted output must remain review-only: %+v: %v", report, err)
	}
}

func TestFilenameMigrationRejectsConvertedWithoutJournal(t *testing.T) {
	receipt, oldOriginal, _ := filenameMigrationFixture(t)
	receipt.Library.Converted = true
	receipt.Convert = true
	receipt.Library.Path = strings.TrimSuffix(oldOriginal, ".jpg") + ".converted-170c91a2.jxl"
	config := t.TempDir()
	rs := booruBrowserRoutes{configPath: config}
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	report, err := rs.filenameMigrationReport()
	if err != nil || report.ConvertedEligible != 0 || len(report.Items) != 1 ||
		report.Items[0].Status != "review" {
		t.Fatalf("untracked JXL must remain review-only: %+v: %v", report, err)
	}
}

func TestPlanBooruFilenameMigrationFlagsSuspiciousCharacterCategory(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	receipt.Post.Tags = []booru.Tag{{Name: "animated", Category: "character"}}
	item := planBooruFilenameMigration("fixture-source-123.json", receipt)
	if item == nil || item.Status != "audit" || len(item.Warnings) != 1 {
		t.Fatalf("expected independent, read-only metadata warning: %+v", item)
	}
}

func writeFilenameMigrationJournalFixture(t *testing.T, config, directory, originalPath string) {
	t.Helper()
	id := strings.Repeat("a", 32)
	afterPath := strings.TrimSuffix(originalPath, filepath.Ext(originalPath)) + ".converted-" + id[:8] + ".jxl"
	record := mediaconvert.Record{
		ID:     id,
		Status: "restored",
		Before: mediaconvert.Snapshot{
			Image: &models.ImageFile{BaseFile: &models.BaseFile{ID: 12, Path: originalPath}},
		},
		After: mediaconvert.Snapshot{
			Image: &models.ImageFile{BaseFile: &models.BaseFile{ID: 12, Path: afterPath}},
		},
	}
	root := filepath.Join(config, directory)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFilenameMigrationBlocksRestorationJournalPaths(t *testing.T) {
	for _, directory := range []string{"media-conversions", "media-upscaling-restores"} {
		t.Run(directory, func(t *testing.T) {
			receipt, oldPath, _ := filenameMigrationFixture(t)
			config := t.TempDir()
			rs := booruBrowserRoutes{configPath: config}
			if err := rs.saveDownloadReceipt(receipt); err != nil {
				t.Fatal(err)
			}
			writeFilenameMigrationJournalFixture(t, config, directory, oldPath)
			history, err := filenameMigrationRestorationPaths(config)
			if err != nil || !history[filepath.Clean(oldPath)] {
				t.Fatalf("restore journal must protect the old path: %v, %v", history, err)
			}
			report, err := rs.filenameMigrationReport()
			if err != nil || report.Eligible != 0 || report.Review != 1 ||
				len(report.Items) != 1 || report.Items[0].Status != "review" {
				t.Fatalf("a restored file must not be renamable: %+v, %v", report, err)
			}
		})
	}
}

func TestFilenameMigrationRejectsUnreadableRestorationHistory(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	config := t.TempDir()
	rs := booruBrowserRoutes{configPath: config}
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(config, "media-conversions")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, strings.Repeat("b", 32)+".json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := rs.filenameMigrationReport()
	if err != nil || report.Eligible != 0 || report.Review != 1 ||
		len(report.Items) != 1 || report.Items[0].Status != "review" {
		t.Fatalf("unreadable conversion journal must block renames: %+v, %v", report, err)
	}
}

func TestFilenameMigrationPreviewAndDismiss(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	config := t.TempDir()
	rs := booruBrowserRoutes{configPath: config}
	if err := os.MkdirAll(filepath.Join(config, "booru-download-history"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	rs.FilenameMigration(w, httptest.NewRequest("GET", "/booru-browser/filename-migration", nil))
	var report booruFilenameMigrationReport
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.Eligible != 1 || report.Dismissed {
		t.Fatalf("invalid preview %d: %s", w.Code, w.Body.String())
	}
	request := httptest.NewRequest("POST", "/booru-browser/filename-migration", strings.NewReader(`{"action":"dismiss"}`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	rs.UpdateFilenameMigration(w, request)
	if w.Code != 200 || !rs.filenameMigrationDismissed() {
		t.Fatalf("cannot dismiss optional migration: %d %s", w.Code, w.Body.String())
	}
}
