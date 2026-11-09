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
	b := p.GetThumbnailPathForEncoding(sum, 320, "webp", 55)
	c := p.GetThumbnailPathForEncoding(sum, 320, "webp", 70)
	if a == b || b == c || filepath.Ext(a) != ".jxl" || filepath.Ext(b) != ".webp" {
		t.Fatalf("format/quality cache collisions: %q %q %q", a, b, c)
	}
	if got := p.GetClipPreviewPathForEncoding(sum, 320, "mp4", 55); filepath.Ext(got) != ".mp4" {
		t.Fatalf("MP4 clip path extension: %q", got)
	}
}
