package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
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

func TestPlanBooruFilenameMigrationFlagsSuspiciousCharacterCategory(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	receipt.Post.Tags = []booru.Tag{{Name: "animated", Category: "character"}}
	item := planBooruFilenameMigration("fixture-source-123.json", receipt)
	if item == nil || item.Status != "audit" || len(item.Warnings) != 1 {
		t.Fatalf("expected independent, read-only metadata warning: %+v", item)
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
