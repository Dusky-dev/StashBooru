package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMediaBackupContains(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "media")
	if !mediaBackupContains(parent, parent) {
		t.Fatal("expected root to contain itself")
	}
	if !mediaBackupContains(parent, filepath.Join(parent, "sub", "image.png")) {
		t.Fatal("expected nested image to be inside root")
	}
	if mediaBackupContains(parent, parent+"-backup") {
		t.Fatal("sibling with common prefix is not inside root")
	}
}

func TestMediaBackupDestinationValidation(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "media")
	store := filepath.Join(base, "separate-disk")
	for _, dir := range []string{root, store} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := validateBackupDestination(store, []string{root}); err != nil {
		t.Fatalf("separate destination rejected: %v", err)
	}
	if _, err := validateBackupDestination(root, []string{root}); err == nil {
		t.Fatal("allowed backup over source root")
	}
	if _, err := validateBackupDestination(filepath.Join(root, "missing"), []string{root}); err == nil {
		t.Fatal("allowed nonexistent backup destination")
	}
	inside := filepath.Join(root, "backup")
	if err := os.Mkdir(inside, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := validateBackupDestination(inside, []string{root}); err == nil {
		t.Fatal("allowed nested backup destination")
	}
	if _, err := validateBackupDestination(base, []string{root}); err == nil {
		t.Fatal("allowed parent backup destination")
	}
	if _, err := validateBackupDestination("relative", []string{root}); err == nil {
		t.Fatal("allowed relative backup destination")
	}
}

func TestMediaBackupDestinationSymlinkAlias(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "media")
	alias := filepath.Join(base, "media-alias")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := validateBackupDestination(alias, []string{root}); err == nil {
		t.Fatal("symlink alias bypassed overlap validation")
	}
}
