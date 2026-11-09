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
