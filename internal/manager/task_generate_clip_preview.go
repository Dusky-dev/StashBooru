package manager

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type GenerateClipPreviewTask struct {
	Image     models.Image
	Overwrite bool
}

func (t *GenerateClipPreviewTask) GetDescription() string {
	return fmt.Sprintf("Generating Preview for image Clip %s", t.Image.Path)
}

func (t *GenerateClipPreviewTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	mgr := GetInstance()
	format := mgr.Config.GetClipPreviewFormat()
	quality := mgr.Config.GetClipPreviewQuality()
	prevPath := mgr.Paths.Generated.GetClipPreviewPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	if mgr.Config.IsExperimentalClipPreviewEnabled() {
		prevPath = mgr.Paths.Generated.GetExperimentalClipPreviewPath(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	}
	f := t.Image.Files.Primary()
	if f == nil {
		return
	}
	filePath := f.Base().Path

	clipPreviewOptions := image.ClipPreviewOptions{
		InputArgs:  GetInstance().Config.GetTranscodeInputArgs(),
		OutputArgs: GetInstance().Config.GetTranscodeOutputArgs(),
		Preset:     GetInstance().Config.GetPreviewPreset().String(),
		Format:     format,
		Quality:    quality,
	}

	encoder := image.NewThumbnailEncoder(GetInstance().FFMpeg, GetInstance().FFProbe, clipPreviewOptions)
	err := encoder.GetPreview(filePath, prevPath, models.DefaultGthumbWidth)
	if err != nil {
		logger.Errorf("getting preview for image %s: %w", filePath, err)
		return
	}

}

func (t *GenerateClipPreviewTask) required() bool {
	_, ok := t.Image.Files.Primary().(*models.VideoFile)
	if !ok {
		return false
	}

	if t.Overwrite {
		return true
	}

	mgr := GetInstance()
	format := mgr.Config.GetClipPreviewFormat()
	quality := mgr.Config.GetClipPreviewQuality()
	prevPath := mgr.Paths.Generated.GetClipPreviewPathForEncoding(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	if mgr.Config.IsExperimentalClipPreviewEnabled() {
		prevPath = mgr.Paths.Generated.GetExperimentalClipPreviewPath(t.Image.Checksum, models.DefaultGthumbWidth, format, quality)
	}
	if exists, _ := fsutil.FileExists(prevPath); exists {
		return false
	}

	return true
}
