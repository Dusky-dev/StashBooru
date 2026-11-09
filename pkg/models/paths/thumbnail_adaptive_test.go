package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExperimentalAdaptiveProfileCacheKeys(t *testing.T) {
	p := newGeneratedPaths(t.TempDir())
	sum := strings.Repeat("e", 32)
	fixed := p.GetExperimentalThumbnailProfilePath(sum, 480, "jxl", 75, "fixed", 55, 100)
	adaptive := p.GetExperimentalThumbnailProfilePath(sum, 480, "jxl", 75, "adaptive", 55, 100)
	if fixed == adaptive || filepath.Ext(adaptive) != ".jxl" {
		t.Fatalf("fixed/adaptive collision: %s vs %s", fixed, adaptive)
	}
	if adaptive == p.GetExperimentalThumbnailProfilePath(sum, 640, "jxl", 75, "adaptive", 55, 100) ||
		adaptive == p.GetExperimentalThumbnailProfilePath(sum, 480, "jxl", 75, "adaptive", 60, 100) ||
		adaptive == p.GetExperimentalThumbnailProfilePath(sum, 480, "jxl", 75, "adaptive", 55, 110) {
		t.Fatal("adaptive profile settings must create distinct cache keys")
	}
}

func TestExperimentalAdaptiveAVIFProfilePath(t *testing.T) {
	p := newGeneratedPaths(t.TempDir())
	sum := strings.Repeat("b", 32)
	fixed := p.GetExperimentalThumbnailProfilePath(sum, 320, "avif", 60, "fixed", 50, 100)
	adaptive := p.GetExperimentalThumbnailProfilePath(sum, 320, "avif", 60, "adaptive", 50, 100)
	jxl := p.GetExperimentalThumbnailProfilePath(sum, 320, "jxl", 60, "adaptive", 50, 100)
	if fixed == adaptive || adaptive == jxl || filepath.Ext(adaptive) != ".avif" {
		t.Fatalf("AVIF profile collides with other caches: %q / %q / %q", fixed, adaptive, jxl)
	}
}
