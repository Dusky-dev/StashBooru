package mediaconvert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

func TestRenameCompletedPathsPreservesRestoreSnapshots(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	id := strings.Repeat("a", 32)
	oldOriginal := filepath.Join(root, "artist, second.jpg")
	newOriginal := filepath.Join(root, "artist+second.jpg")
	oldOutput := filepath.Join(root, "artist, second.converted-aaaaaaaa.jxl")
	newOutput := filepath.Join(root, "artist+second.converted-aaaaaaaa.jxl")
	record := &Record{
		ID: id,
		Status: "complete",
		Cached: true,
		Before: Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{
			ID: 7, Path: oldOriginal, Basename: filepath.Base(oldOriginal),
		}}},
		After: Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{
			ID: 7, Path: oldOutput, Basename: filepath.Base(oldOutput),
		}}},
	}
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	if err := store.RenameCompletedPaths(id, oldOriginal, newOriginal, oldOutput, newOutput); err != nil {
		t.Fatal(err)
	}
	renamed, err := store.Record(id)
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Before.File().Base().Path != newOriginal ||
		renamed.After.File().Base().Path != newOutput ||
		renamed.Before.File().Base().Basename != filepath.Base(newOriginal) ||
		renamed.After.File().Base().Basename != filepath.Base(newOutput) ||
		!renamed.Cached || renamed.Status != "complete" {
		t.Fatalf("journal snapshots no longer match the renamed files: %+v", renamed)
	}
	if err := store.RenameCompletedPaths(id, newOriginal, oldOriginal, newOutput, oldOutput); err != nil {
		t.Fatal(err)
	}
	reverted, err := store.Record(id)
	if err != nil || reverted.Before.File().Base().Path != oldOriginal ||
		reverted.After.File().Base().Path != oldOutput {
		t.Fatalf("journal rollback did not restore both paths: %+v %v", reverted, err)
	}
}

func TestRenameCompletedPathsRejectsAmbiguousOrIncompleteJournal(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	id := strings.Repeat("b", 32)
	before := filepath.Join(root, "old.gif")
	after := filepath.Join(root, "old.converted-bbbbbbbb.jxl")
	record := &Record{
		ID: id, Status: "complete",
		Before: Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{ID: 1, Path: before}}},
		After: Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{ID: 1, Path: after}}},
	}
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, original, output string }{
		{"mismatched original", filepath.Join(root, "someone-else.gif"), filepath.Join(root, "new.converted-bbbbbbbb.jxl")},
		{"cross directory", filepath.Join(root, "sub", "new.gif"), filepath.Join(root, "new.converted-bbbbbbbb.jxl")},
		{"changed output type", filepath.Join(root, "new.gif"), filepath.Join(root, "new.converted-bbbbbbbb.png")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.RenameCompletedPaths(id, tc.original, filepath.Join(root, "new.gif"), after, tc.output); err == nil {
				t.Fatal("unsafe journal rewrite must be rejected")
			}
		})
	}
	record.Status = "restored"
	if err := store.save(record); err != nil {
		t.Fatal(err)
	}
	if err := store.RenameCompletedPaths(id, before, filepath.Join(root, "new.gif"), after, filepath.Join(root, "new.converted-bbbbbbbb.jxl")); err == nil {
		t.Fatal("restored journal needs a separate verified migration")
	}
	// The journal must never be modified by a rejected request.
	b, err := os.ReadFile(filepath.Join(root, id+".json"))
	if err != nil || !strings.Contains(string(b), "old.gif") {
		t.Fatalf("rejected rewrite unexpectedly changed journal: %v", err)
	}
}
