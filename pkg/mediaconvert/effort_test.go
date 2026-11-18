package mediaconvert

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

func imageFile(path string, width, height, frames int) *models.ImageFile {
	return &models.ImageFile{BaseFile: &models.BaseFile{Path: path, FrameCount: frames}, Width: width, Height: height}
}

func TestAutoJXLEffort(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     models.File
		distance float64
		scale    int
		want     int
	}{
		{"small lossy still", imageFile("a.png", 1000, 1000, 1), 1, 0, 9},
		{"medium lossy still", imageFile("a.png", 2000, 3000, 1), 1, 0, 8},
		{"large lossy still", imageFile("a.png", 4000, 6000, 1), 1, 0, 7},
		{"huge lossy still", imageFile("a.png", 8000, 8000, 1), 1, 0, 5},
		{"upscale counts output pixels", imageFile("a.png", 1000, 1000, 1), 1, 2, 8},
		{"lossless still", imageFile("a.png", 2000, 2000, 1), 0, 0, 7},
		{"huge lossless still", imageFile("a.png", 5000, 5000, 1), 0, 0, 5},
		{"JPEG recompression", imageFile("a.jpg", 2000, 3000, 1), 0, 0, 9},
		{"short animation", imageFile("a.gif", 480, 270, 60), 1, 0, 7},
		{"long animation", imageFile("a.gif", 1920, 1080, 120), 1, 0, 5},
		{"video frames from duration", &models.VideoFile{BaseFile: &models.BaseFile{Path: "a.mp4"}, Width: 640, Height: 360, Duration: 2, FrameRate: 30}, 1, 0, 7},
	} {
		got, ok := AutoJXLEffort(tc.file, tc.distance, tc.scale)
		if !ok || got != tc.want {
			t.Errorf("%s: got %d (ok=%v), want %d", tc.name, got, ok, tc.want)
		}
	}
	if _, ok := AutoJXLEffort(imageFile("a.png", 1000, 1000, 0), 1, 0); ok {
		t.Error("an uninspected frame count must keep the manual effort")
	}
}

func TestApplyAutoEffortKeepsManualEffortOutsideJXL(t *testing.T) {
	small := imageFile("a.png", 1000, 1000, 1)
	options := Options{Format: "webp", Effort: 4, Distance: 1}
	ApplyAutoEffort(&options, small, true)
	if options.Effort != 4 {
		t.Fatalf("WebP effort changed to %d", options.Effort)
	}
	options.Format = "jxl"
	ApplyAutoEffort(&options, small, false)
	if options.Effort != 4 {
		t.Fatalf("manual JXL effort changed to %d", options.Effort)
	}
	ApplyAutoEffort(&options, imageFile("a.png", 1000, 1000, 0), true)
	if options.Effort != 4 {
		t.Fatalf("unknown size changed effort to %d", options.Effort)
	}
	ApplyAutoEffort(&options, small, true)
	if options.Effort != 9 {
		t.Fatalf("auto JXL effort = %d, want 9", options.Effort)
	}
}
