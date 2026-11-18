package mediaconvert

import (
	"math"
	"path/filepath"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

// AutoJXLEffort picks a cjxl effort (1–9) for one file from its pixel work
// (width × height × frames, after upscaling) and whether the encode is
// lossless. ok is false when the file's size is unknown; callers then keep the
// manual effort.
//
// Thresholds come from cjxl 0.7 timings on 4 threads (docs/media-converter.md):
// lossy stills gained 3–10% at effort 8–9 for 5–10× the time, lossless effort 9
// took 15× longer than 7 for 2%, animations gained nothing above 7, and JPEG
// recompression was under a second at any effort.
func AutoJXLEffort(f models.File, distance float64, upscaleScale int) (effort int, ok bool) {
	width, height, frames := 0, 0, f.Base().FrameCount
	switch file := f.(type) {
	case *models.ImageFile:
		width, height = file.Width, file.Height
	case *models.VideoFile:
		width, height = file.Width, file.Height
		if frames <= 0 && file.Duration > 0 && file.FrameRate > 0 {
			frames = int(math.Round(file.Duration * file.FrameRate))
		}
	}
	if frames <= 0 && !mayAnimate(f.Base().Path) {
		// Files scanned before frame inspection store 0; still-only formats
		// such as JPEG have exactly one frame.
		frames = 1
	}
	if width <= 0 || height <= 0 || frames <= 0 {
		return 0, false
	}
	scale := float64(1)
	if upscaleScale > 1 {
		scale = float64(upscaleScale * upscaleScale)
	}
	const megapixel = 1_000_000
	still := float64(width) * float64(height) * scale / megapixel
	work := still * float64(frames)
	switch {
	case frames > 1:
		if work <= 16 {
			return 7, true
		}
		return 5, true
	case distance == 0 && SourceFormat(f) == "jpeg":
		// Lossless JPEG recompression is cheap; spend effort on size.
		if work <= 8 {
			return 9, true
		}
		return 7, true
	case distance == 0:
		if work <= 16 {
			return 7, true
		}
		return 5, true
	case work <= 2:
		return 9, true
	case work <= 8:
		return 8, true
	case work <= 32:
		return 7, true
	default:
		return 5, true
	}
}

// ApplyAutoEffort replaces options.Effort with the automatic JPEG XL effort
// when auto is set and the output is JPEG XL. Other formats and files of
// unknown size keep their manual effort, so the worker always receives 1–9.
func ApplyAutoEffort(options *Options, f models.File, auto bool) {
	if !auto || f == nil || (options.Format != "jxl" && options.Format != "ajxl") {
		return
	}
	if effort, ok := AutoJXLEffort(f, options.Distance, options.UpscaleScale); ok {
		options.Effort = effort
	}
}

// mayAnimate matches the containers animation.Count inspects; any other image
// extension is a single frame.
func mayAnimate(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gif", ".png", ".apng", ".webp", ".jxl", ".avif", ".avifs", ".heic", ".heif":
		return true
	}
	return false
}
