package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThumbnailCacheMatchesIsolated(t *testing.T) {
	prefix := strings.Repeat("a", 32) + "_320"
	cases := []struct {
		name, category string
		want           bool
	}{
		{prefix + ".jpg", "image", true},
		{prefix + "_q60.jxl", "image", true},
		{prefix + "_exp_q60.jxl", "image", true},
		{prefix + "_exp_q45.jpg", "image", true},
		{prefix + "_exp_aq75_m55_b100.jxl", "image", true},
		{prefix + "_exp_aq75_m55_b100.jpg", "image", true},
		{prefix + "_q45.webp", "image", true},
		{prefix + ".webm", "image", false},
		{prefix + "_anim_q65.webp", "image", false},
		{prefix + "_anim_q65.webp", "animated", true},
		{prefix + "_q65.webp", "animated", false},
		{prefix + ".webm", "clip", true},
		{prefix + "_q55.mp4", "clip", true},
		{prefix + "_exp_q75.webm", "clip", true},
		{prefix + "_exp_q55.mp4", "clip", true},
		{prefix + ".jpg", "clip", false},
		{"notes.webp", "image", false},
		{"../../bad.jpg", "image", false},
	}
	for _, tc := range cases {
		if got := thumbnailCacheMatches(tc.name, tc.category); got != tc.want {
			t.Errorf("%s %s: %t; want %t", tc.name, tc.category, got, tc.want)
		}
	}
}

func TestRemoveThumbnailCategoryLeavesOtherCachesAndUnknownFiles(t *testing.T) {
	dir := t.TempDir()
	hash := strings.Repeat("a", 32)
	root := filepath.Join(dir, "thumbnails")
	inner := filepath.Join(root, "aa", "aa")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	names := []string{hash + "_320.jpg", hash + "_320_q45.jxl", hash + "_320_anim_q75.webp", hash + "_320.webm", "notes.txt"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(inner, name), []byte("cache"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := removeThumbnailCategory(context.Background(), root, "image")
	if err != nil || removed != 2 {
		t.Fatalf("removed %d, err %v; want 2", removed, err)
	}
	for _, name := range names[:2] {
		if _, err := os.Stat(filepath.Join(inner, name)); !os.IsNotExist(err) {
			t.Errorf("expected removed cache: %s", name)
		}
	}
	for _, name := range names[2:] {
		if _, err := os.Stat(filepath.Join(inner, name)); err != nil {
			t.Errorf("unrelated cache removed: %s: %v", name, err)
		}
	}
	if _, err := removeThumbnailCategory(context.Background(), root, "bogus"); err == nil {
		t.Fatal("invalid category accepted")
	}
}

// Existing native thumbnails are not touched when keeping the old cache.
// A separate deletion must remove only those legacy paths, not experimental
// files or animated media.
func TestKeepOldAndDeleteOldAreCategoryScoped(t *testing.T) {
	root := filepath.Join(t.TempDir(), "thumbnails")
	sum := strings.Repeat("b", 32)
	folder := filepath.Join(root, "bb", "bb")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{
		"_320.jpg", "_320_exp_q70.jpg", "_320_exp_q55.webp",
		"_320.webm", "_320_exp_q50.mp4", "_320_anim_q70.webp",
	} {
		if err := os.WriteFile(filepath.Join(folder, sum+suffix), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	n, err := removeThumbnailCategoryPreservingOld(context.Background(), root, "image")
	if err != nil || n != 2 {
		t.Fatalf("image cleanup removed %d, err %v", n, err)
	}
	for _, suffix := range []string{"_320.jpg", "_320.webm", "_320_exp_q50.mp4", "_320_anim_q70.webp"} {
		if _, err := os.Stat(filepath.Join(folder, sum+suffix)); err != nil {
			t.Errorf("unrelated/old variant lost: %s: %v", suffix, err)
		}
	}
	n, err = RemoveOldThumbnailCategory(context.Background(), root, "image")
	if err != nil || n != 1 {
		t.Fatalf("old-image cleanup removed %d, err %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(folder, sum+"_320.webm")); err != nil {
		t.Fatal("video clip native cache must survive still-image cleanup", err)
	}
	n, err = removeThumbnailCategoryPreservingOld(context.Background(), root, "clip")
	if err != nil || n != 1 {
		t.Fatalf("clip cleanup removed %d, err %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(folder, sum+"_320.webm")); err != nil {
		t.Fatal("old clip cache must remain:", err)
	}
	n, err = RemoveOldThumbnailCategory(context.Background(), root, "clip")
	if err != nil || n != 1 {
		t.Fatalf("old-clip cleanup removed %d, err %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(folder, sum+"_320_anim_q70.webp")); err != nil {
		t.Fatal("animated thumbnail cache must survive:", err)
	}
}
