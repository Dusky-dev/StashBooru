package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodingPathsKeepLegacyDefaultsAndSeparateVariants(t *testing.T) {
	p := newGeneratedPaths(t.TempDir())
	sum := strings.Repeat("a", 64)
	if got, want := p.GetThumbnailPathForEncoding(sum, 320, "jpg", 70), p.GetThumbnailPath(sum, 320); got != want {
		t.Fatalf("legacy thumbnail path changed: got %q want %q", got, want)
	}
	if got, want := p.GetClipPreviewPathForEncoding(sum, 320, "webm", 75), p.GetClipPreviewPath(sum, 320); got != want {
		t.Fatalf("legacy clip path changed: got %q want %q", got, want)
	}
	a := p.GetThumbnailPathForEncoding(sum, 320, "jxl", 55)
	avif := p.GetThumbnailPathForEncoding(sum, 320, "avif", 55)
	b := p.GetThumbnailPathForEncoding(sum, 320, "webp", 55)
	c := p.GetThumbnailPathForEncoding(sum, 320, "webp", 70)
	if a == b || b == c || a == avif || b == avif || filepath.Ext(avif) != ".avif" || filepath.Ext(a) != ".jxl" || filepath.Ext(b) != ".webp" {
		t.Fatalf("format/quality cache collisions: %q %q %q", a, b, c)
	}
	if got := p.GetClipPreviewPathForEncoding(sum, 320, "mp4", 55); filepath.Ext(got) != ".mp4" {
		t.Fatalf("MP4 clip path extension: %q", got)
	}
}

func TestExperimentalPathsNeverOverwriteNativeCache(t *testing.T) {
	p := newGeneratedPaths(t.TempDir())
	checksum := strings.Repeat("a", 32)
	for _, tc := range []struct {
		experiment string
		native     string
	}{
		{p.GetExperimentalThumbnailPath(checksum, 320, "jpg", 70), p.GetThumbnailPath(checksum, 320)},
		{p.GetExperimentalThumbnailPath(checksum, 320, "jxl", 60), p.GetThumbnailPath(checksum, 320)},
		{p.GetExperimentalThumbnailPath(checksum, 320, "avif", 60), p.GetThumbnailPath(checksum, 320)},
		{p.GetExperimentalClipPreviewPath(checksum, 320, "webm", 75), p.GetClipPreviewPath(checksum, 320)},
		{p.GetExperimentalClipPreviewPath(checksum, 320, "mp4", 60), p.GetClipPreviewPath(checksum, 320)},
	} {
		if tc.experiment == tc.native || !strings.Contains(filepath.Base(tc.experiment), "_exp_q") {
			t.Fatalf("experimental cache must be isolated: %q and %q", tc.experiment, tc.native)
		}
	}
}
