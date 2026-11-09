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
	format := mgr.Config.GetImageThumbnailFormat()
	quality := mgr.Config.GetImageThumbnailQuality()
	thumbPath := mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	f := t.Image.Files.Primary()
	if f == nil {
		return
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
	data, err := encoder.GetThumbnail(f, models.DefaultGthumbWidth)
	if err != nil && format == "jxl" {
		logger.Warnf("[generator] JXL thumbnail unavailable for %s, using WebP: %v", path, err)
		encoder.ThumbnailFormat = "webp"
		thumbPath = mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, "webp", quality)
		data, err = encoder.GetThumbnail(f, models.DefaultGthumbWidth)
	}

	if err != nil {
		// don't log for animated images
		if !errors.Is(err, image.ErrNotSupportedForThumbnail) {
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
	vf, ok := t.Image.Files.Primary().(models.VisualFile)
	if !ok {
		return false
	}

	if vf.GetHeight() <= models.DefaultGthumbWidth && vf.GetWidth() <= models.DefaultGthumbWidth {
		return false
	}

	if t.Overwrite {
		return true
	}

	mgr := GetInstance()
	format := mgr.Config.GetImageThumbnailFormat()
	quality := mgr.Config.GetImageThumbnailQuality()
	thumbPath := mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	exists, _ := fsutil.FileExists(thumbPath)
	if !exists && format == "jxl" {
		fallback := mgr.Paths.Generated.GetThumbnailPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, "webp", quality)
		exists, _ = fsutil.FileExists(fallback)
	}
	return !exists
}
