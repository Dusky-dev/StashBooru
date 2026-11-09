package manager

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type GenerateImageThumbnailTask struct {
	Image     models.Image
	Overwrite bool
}

func (t *GenerateImageThumbnailTask) GetDescription() string {
	return fmt.Sprintf("Generating Thumbnail for image %s", t.Image.Path)
}

func (t *GenerateImageThumbnailTask) logStderr(err error) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		logger.Debugf("[generator] error output: %s", exitErr.Stderr)
	}
}

func (t *GenerateImageThumbnailTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	mgr := GetInstance()
	f := t.Image.Files.Primary()
	if f == nil {
		return
	}
	animated, err := image.IsAnimatedSource(f)
	if err != nil {
		logger.Errorf("[generator] detecting animation for %s: %v", f.Base().Path, err)
		return
	}
	if animated && !mgr.Config.IsExperimentalAnimatedThumbnailEnabled() {
		return // preserve native animated GIF/WebP behavior
	}
	format := mgr.Config.GetImageThumbnailFormat()
	quality := mgr.Config.GetImageThumbnailQuality()
	width := models.DefaultGthumbWidth
	thumbPath := mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, width, format, quality)
	if mgr.Config.IsExperimentalImageThumbnailEnabled() {
		width = mgr.Config.GetExperimentalThumbnailResolution()
		thumbPath = mgr.Paths.Generated.GetExperimentalThumbnailProfilePath(
			t.Image.Checksum, width, format, quality,
			mgr.Config.GetExperimentalThumbnailMode(),
			mgr.Config.GetExperimentalThumbnailMinQuality(),
			mgr.Config.GetExperimentalThumbnailBudget(),
		)
	}
	if animated {
		format = "webp"
		quality = mgr.Config.GetAnimatedThumbnailQuality()
		width = mgr.Config.GetExperimentalAnimatedThumbnailResolution()
		thumbPath = mgr.Paths.Generated.GetAnimatedThumbnailPathForEncoding(t.Image.Checksum, width, quality)
	}
	path := f.Base().Path

	logger.Debugf("Generating thumbnail for %s", path)

	c := mgr.Config

	clipPreviewOptions := image.ClipPreviewOptions{
		InputArgs:  c.GetTranscodeInputArgs(),
		OutputArgs: c.GetTranscodeOutputArgs(),
		Preset:     c.GetPreviewPreset().String(),
	}

	encoder := image.NewThumbnailEncoder(mgr.FFMpeg, mgr.FFProbe, clipPreviewOptions)
	encoder.ThumbnailFormat = format
	encoder.ThumbnailQuality = quality
	var data []byte
	switch {
	case animated:
		data, err = encoder.GetAnimatedThumbnail(f, width, quality)
	case mgr.Config.IsExperimentalImageThumbnailEnabled() && mgr.Config.GetExperimentalThumbnailMode() == "adaptive":
		nativePath := mgr.Paths.Generated.GetThumbnailPath(t.Image.Checksum, models.DefaultGthumbWidth)
		var choice image.AdaptiveThumbnailChoice
		data, choice, err = encoder.GetAdaptiveThumbnail(
			ctx, f, width, mgr.Config.GetExperimentalThumbnailMinQuality(),
			mgr.Config.GetExperimentalThumbnailBudget(), nativePath,
		)
		if err == nil {
			logger.Debugf("[generator] adaptive thumbnail %s: %dpx Q%d, %d bytes", path, choice.Resolution, choice.Quality, choice.Bytes)
		}
	default:
		data, err = encoder.GetThumbnail(f, width)
	}
	if err != nil {
		// Unsupported animated images are expected; selected JXL encoder errors
		// are not, and must remain visible instead of being silently replaced.
		if format == "jxl" || !errors.Is(err, image.ErrNotSupportedForThumbnail) {
			logger.Errorf("[generator] getting thumbnail for image %s: %s", path, err.Error())
			t.logStderr(err)
		}
		return
	}

	err = fsutil.WriteFile(thumbPath, data)
	if err != nil {
		logger.Errorf("[generator] writing thumbnail for image %s: %s", path, err.Error())
		return
	}
}

func (t *GenerateImageThumbnailTask) required() bool {
	f := t.Image.Files.Primary()
	if f == nil {
		return false
	}
	animated, err := image.IsAnimatedSource(f)
	if err != nil {
		logger.Errorf("[generator] determining animation: %v", err)
		return false
	}
	mgr := GetInstance()
	if animated && !mgr.Config.IsExperimentalAnimatedThumbnailEnabled() {
		return false
	}
	vf, ok := f.(models.VisualFile)
	if !ok {
		return false
	}

	// Native mode keeps Stash's no-reencode behavior for small images.
	// Explicit experimental codecs rebuild every eligible source regardless
	// of size so the user-selected cache is actually populated.
	if vf.GetHeight() <= models.DefaultGthumbWidth && vf.GetWidth() <= models.DefaultGthumbWidth {
		// Native mode does not need resized thumbnails for small sources,
		// while an explicitly enabled codec still rebuilds them.
		if animated && !mgr.Config.IsExperimentalAnimatedThumbnailEnabled() {
			return false
		}
		if !animated && !mgr.Config.IsExperimentalImageThumbnailEnabled() {
			return false
		}
	}

	if t.Overwrite {
		return true
	}

	format := mgr.Config.GetImageThumbnailFormat()
	quality := mgr.Config.GetImageThumbnailQuality()
	width := models.DefaultGthumbWidth
	thumbPath := mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, width, format, quality)
	if mgr.Config.IsExperimentalImageThumbnailEnabled() {
		width = mgr.Config.GetExperimentalThumbnailResolution()
		thumbPath = mgr.Paths.Generated.GetExperimentalThumbnailProfilePath(
			t.Image.Checksum, width, format, quality,
			mgr.Config.GetExperimentalThumbnailMode(),
			mgr.Config.GetExperimentalThumbnailMinQuality(),
			mgr.Config.GetExperimentalThumbnailBudget(),
		)
	}
	if animated {
		quality = mgr.Config.GetAnimatedThumbnailQuality()
		width = mgr.Config.GetExperimentalAnimatedThumbnailResolution()
		thumbPath = mgr.Paths.Generated.GetAnimatedThumbnailPathForEncoding(t.Image.Checksum, width, quality)
	}
	exists, _ := fsutil.FileExists(thumbPath)
	return !exists
}
