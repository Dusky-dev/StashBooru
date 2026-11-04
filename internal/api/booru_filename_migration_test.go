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

func TestPlanBooruFilenameMigrationOriginalMediaFormats(t *testing.T) {
	for _, ext := range []string{"jpg", "jpeg", "png", "gif", "webp", "avif", "jxl", "mp4", "webm", "mov"} {
		t.Run(ext, func(t *testing.T) {
			receipt, jpgPath, _ := filenameMigrationFixture(t)
			source := booru.Source{ID: receipt.Source.ID, Name: receipt.Source.Name, URL: receipt.Source.URL}
			oldRel, err := booru.RenderLegacyFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, ext, 0)
			if err != nil {
				t.Fatal(err)
			}
			newRel, err := booru.RenderFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, ext, 0)
			if err != nil {
				t.Fatal(err)
			}
			oldPath := filepath.Join(receipt.Directory, oldRel)
			if err := os.Rename(jpgPath, oldPath); err != nil {
				t.Fatal(err)
			}
			receipt.SavedPath = oldPath
			receipt.Library.Path = oldPath
			receipt.Convert = true // The requested conversion may have been skipped.
			if ext == "mp4" || ext == "webm" || ext == "mov" {
				receipt.Library.Kind = "scene"
			}
			plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
			if plan == nil || plan.Status != "ready" ||
				plan.NewPath != filepath.Join(receipt.Directory, newRel) {
				t.Fatalf("verified original %s must be eligible even when conversion was requested: %+v", ext, plan)
			}
		})
	}
}

func TestConvertedFilenameOriginalTargetPreservesInputFormat(t *testing.T) {
	for _, extension := range []string{".jpg", ".png", ".gif", ".mp4"} {
		oldOriginal := filepath.Join("/media", "a, b"+extension)
		oldOutput := filepath.Join("/media", "a, b.converted-aabbccdd.jxl")
		newOutput := filepath.Join("/media", "a+b.converted-aabbccdd.jxl")
		got, err := convertedFilenameOriginalTarget(oldOriginal, oldOutput, newOutput)
		if err != nil || got != filepath.Join("/media", "a+b"+extension) {
			t.Fatalf("could not reconstruct %s restore destination: %q %v", extension, got, err)
		}
	}
	if _, err := convertedFilenameOriginalTarget("/media/old.jpg", "/media/unrelated.converted-aabbccdd.jxl", "/media/new.converted-aabbccdd.jxl"); err == nil {
		t.Fatal("unmatched original and converted paths cannot be migrated")
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

func TestFilenameMigrationReconcilesOnlyVerifiedAlreadyRenamedFile(t *testing.T) {
	receipt, oldPath, proposed := filenameMigrationFixture(t)
	checksum, err := mediaconvert.MD5(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt.OutputMD5 = checksum
	receipt.Library.ID = "12"
	if err := os.Rename(oldPath, proposed); err != nil {
		t.Fatal(err)
	}
	makePlan := func() *booruFilenameMigrationItem {
		plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
		if plan == nil || plan.Status != "review" || plan.NewPath != proposed {
			t.Fatalf("missing original should be review before identity verification: %+v", plan)
		}
		return plan
	}
	verified := func(path string, library booruLibraryResult) (bool, error) {
		if path != proposed || library.ID != "12" || library.Kind != "image" {
			t.Fatalf("wrong identity candidate: %q %+v", path, library)
		}
		return true, nil
	}
	plan := makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, nil, nil, verified)
	if plan.Status != "ready-receipt" {
		t.Fatalf("verified same-media destination should allow receipt-only repair: %+v", plan)
	}
	plan = makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, nil, nil, func(string, booruLibraryResult) (bool, error) {
		return false, nil
	})
	if plan.Status != "review" {
		t.Fatalf("matching bytes are insufficient without native media identity: %+v", plan)
	}
	plan = makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, map[string]bool{filepath.Clean(oldPath): true}, nil, verified)
	if plan.Status != "review" {
		t.Fatalf("restore journal dependency must forbid receipt-only repair: %+v", plan)
	}
	plan = makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, nil, os.ErrPermission, verified)
	if plan.Status != "review" {
		t.Fatalf("unreadable journals must forbid receipt-only repair: %+v", plan)
	}
	if err := os.WriteFile(proposed, []byte("different content"), 0600); err != nil {
		t.Fatal(err)
	}
	plan = makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, nil, nil, verified)
	if plan.Status != "review" || !strings.Contains(plan.Reason, "checksum") {
		t.Fatalf("destination modified after rename must not be accepted: %+v", plan)
	}
	if err := os.Remove(proposed); err != nil {
		t.Fatal(err)
	}
	plan = makePlan()
	checkFilenameMigrationAlreadyRenamed(plan, receipt, nil, nil, verified)
	if plan.Status != "review" || !strings.Contains(plan.Reason, "both missing") {
		t.Fatalf("missing file cannot be repaired from receipt alone: %+v", plan)
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
		ID:     id,
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
	// An earlier failed attempt at converting the same original must not
	// make a uniquely matched active conversion ambiguous.
	staleID := strings.Repeat("d", 32)
	stale := mediaconvert.Record{ID: staleID, Status: "failed", Before: record.Before}
	staleData, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, staleID+".json"), staleData, 0600); err != nil {
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

func TestFilenameMigrationOffersConvertedVideoWithHistoryMigration(t *testing.T) {
	receipt, previous, _ := filenameMigrationFixture(t)
	source := booru.Source{ID: receipt.Source.ID, Name: receipt.Source.Name, URL: receipt.Source.URL}
	originalName, err := booru.RenderLegacyFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, "mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	nextName, err := booru.RenderFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, "mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(receipt.Directory, originalName)
	if err := os.Rename(previous, original); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("e", 32)
	converted := strings.TrimSuffix(original, ".mp4") + ".converted-" + id[:8] + ".mp4"
	if err := os.Rename(original, converted); err != nil {
		t.Fatal(err)
	}
	checksum, err := mediaconvert.MD5(converted)
	if err != nil {
		t.Fatal(err)
	}
	receipt.SavedPath, receipt.Library.Path = original, converted
	receipt.Library.Kind, receipt.Library.Converted = "scene", true
	receipt.Convert, receipt.OutputMD5 = true, checksum
	config := t.TempDir()
	rs := booruBrowserRoutes{configPath: config}
	if err := rs.saveDownloadReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	record := mediaconvert.Record{
		ID: id, Status: "complete",
		Before: mediaconvert.Snapshot{Video: &models.VideoFile{BaseFile: &models.BaseFile{
			ID: 22, Path: original, Fingerprints: models.Fingerprints{{Type: "md5", Fingerprint: receipt.OriginalMD5}},
		}}},
		After: mediaconvert.Snapshot{Video: &models.VideoFile{BaseFile: &models.BaseFile{
			ID: 22, Path: converted, Fingerprints: models.Fingerprints{{Type: "md5", Fingerprint: checksum}},
		}}},
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
	expected := strings.TrimSuffix(filepath.Join(receipt.Directory, nextName), ".mp4") + ".converted-" + id[:8] + ".mp4"
	if err != nil || report.ConvertedEligible != 1 || len(report.Items) != 1 ||
		report.Items[0].Status != "ready-converted" || report.Items[0].NewPath != expected {
		t.Fatalf("converted MP4 must preserve a verified journal during rename: %+v %v", report, err)
	}
}

func TestFilenameMigrationOffersRestoredOriginalWithJournal(t *testing.T) {
	for _, refreshedReceipt := range []bool{false, true} {
		testName := "stale converted receipt"
		if refreshedReceipt {
			testName = "receipt updated after restore"
		}
		t.Run(testName, func(t *testing.T) {
			receipt, oldOriginal, newOriginal := filenameMigrationFixture(t)
			id := strings.Repeat("f", 32)
			oldConverted := strings.TrimSuffix(oldOriginal, ".jpg") + ".converted-" + id[:8] + ".jxl"
			receipt.Library.Converted = !refreshedReceipt
			receipt.Library.Path = oldConverted
			receipt.Convert = true
			convertedMD5 := strings.Repeat("c", 32)
			receipt.OutputMD5 = convertedMD5
			if refreshedReceipt {
				receipt.Library.Path = oldOriginal
				receipt.OutputMD5 = receipt.OriginalMD5
			}

			config := t.TempDir()
			rs := booruBrowserRoutes{configPath: config}
			if err := rs.saveDownloadReceipt(receipt); err != nil {
				t.Fatal(err)
			}
			record := mediaconvert.Record{
				ID: id, Status: "restored",
				Before: mediaconvert.Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{
					ID: 32, Path: oldOriginal,
					Fingerprints: models.Fingerprints{{Type: "md5", Fingerprint: receipt.OriginalMD5}},
				}}},
				After: mediaconvert.Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{
					ID: 32, Path: oldConverted,
					Fingerprints: models.Fingerprints{{Type: "md5", Fingerprint: convertedMD5}},
				}}},
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
			if err != nil || report.ConvertedEligible != 1 || len(report.Items) != 1 ||
				report.Items[0].Status != "ready-converted" ||
				report.Items[0].OldPath != oldOriginal ||
				report.Items[0].NewPath != newOriginal {
				t.Fatalf("restored original must migrate both history paths: %+v %v", report, err)
			}
		})
	}
}

func TestFilenameMigrationJXLApplyNeedsExplicitRestoreLossConfirmation(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	request := httptest.NewRequest("POST", "/booru-browser/filename-migration",
		strings.NewReader(`{"action":"apply-jxl","receipts":["fixture-source-123.json"]}`))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rs.UpdateFilenameMigration(w, request)
	if w.Code != 400 {
		t.Fatalf("JXL rename must require explicit restore-loss confirmation: %d, %s", w.Code, w.Body.String())
	}
}

func TestFilenameMigrationCompletedReceiptSuppressesFurtherRename(t *testing.T) {
	receipt, _, _ := filenameMigrationFixture(t)
	receipt.LegacyFilenameMigrated = true
	if item := planBooruFilenameMigration("fixture-source-123.json", receipt); item != nil {
		t.Fatalf("completed receipt should not appear again: %+v", item)
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

func TestFilenameMigrationIgnoresFailedOrSkippedHistoryForMP4(t *testing.T) {
	for _, directory := range []string{"media-conversions", "media-upscaling-restores"} {
		for _, status := range []string{"failed", "skipped"} {
			t.Run(directory+"/"+status, func(t *testing.T) {
				receipt, oldJPG, _ := filenameMigrationFixture(t)
				source := booru.Source{ID: receipt.Source.ID, Name: receipt.Source.Name, URL: receipt.Source.URL}
				legacy, err := booru.RenderLegacyFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, "mp4", 0)
				if err != nil {
					t.Fatal(err)
				}
				expected, err := booru.RenderFilename(receipt.Template, source, receipt.Post, receipt.OriginalMD5, "mp4", 0)
				if err != nil {
					t.Fatal(err)
				}
				oldMP4 := filepath.Join(receipt.Directory, legacy)
				if err := os.Rename(oldJPG, oldMP4); err != nil {
					t.Fatal(err)
				}
				receipt.SavedPath, receipt.Library.Path = oldMP4, oldMP4
				receipt.Library.Kind = "scene"
				receipt.Convert = true // A requested conversion can fail or be skipped.
				config := t.TempDir()
				rs := booruBrowserRoutes{configPath: config}
				if err := rs.saveDownloadReceipt(receipt); err != nil {
					t.Fatal(err)
				}
				id := strings.Repeat("e", 32)
				record := mediaconvert.Record{
					ID: id, Status: status,
					Before: mediaconvert.Snapshot{Video: &models.VideoFile{
						BaseFile: &models.BaseFile{ID: 12, Path: oldMP4},
					}},
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
				paths, err := filenameMigrationRestorationPaths(config)
				if err != nil || paths[filepath.Clean(oldMP4)] {
					t.Fatalf("non-restorable historical attempt should not protect MP4: %v %v", paths, err)
				}
				report, err := rs.filenameMigrationReport()
				if err != nil || report.Eligible != 1 || report.Review != 0 ||
					len(report.Items) != 1 || report.Items[0].Status != "ready" ||
					report.Items[0].NewPath != filepath.Join(receipt.Directory, expected) {
					t.Fatalf("unchanged MP4 should be eligible despite old failed/skipped history: %+v %v", report, err)
				}
			})
		}
	}
}

func TestFilenameMigrationEncodingHistoryStillBlocksRename(t *testing.T) {
	receipt, oldPath, _ := filenameMigrationFixture(t)
	config := t.TempDir()
	id := strings.Repeat("f", 32)
	record := mediaconvert.Record{
		ID: id, Status: "encoding",
		Before: mediaconvert.Snapshot{Image: &models.ImageFile{
			BaseFile: &models.BaseFile{ID: 12, Path: oldPath},
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
	paths, err := filenameMigrationRestorationPaths(config)
	if err != nil || !paths[filepath.Clean(oldPath)] {
		t.Fatalf("incomplete encoding is still a recovery dependency: %v %v", paths, err)
	}
	plan := planBooruFilenameMigration("fixture-source-123.json", receipt)
	checkFilenameMigrationRestorationRisk(plan, receipt, paths, nil)
	if plan == nil || plan.Status != "review" {
		t.Fatalf("unrecovered encoding must block rename: %+v", plan)
	}
}

func TestFilenameMigrationDropsUnassignedProviderAnimatedWarning(t *testing.T) {
	tags := []booru.Tag{{Name: "animated", Category: "character"}}
	warnings := legacyCategoryWarning(tags)
	if len(warnings) != 1 {
		t.Fatalf("provider snapshot should prompt assignment verification: %v", warnings)
	}
	if remainder := withoutAnimatedProviderWarning(warnings); len(remainder) != 0 {
		t.Fatalf("verified non-assignment should not leave a metadata warning: %v", remainder)
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
