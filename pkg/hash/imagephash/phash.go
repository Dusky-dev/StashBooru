package imagephash

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/corona10/goimagehash"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

// Generate computes a perceptual hash for an image file.
func Generate(encoder *ffmpeg.FFMpeg, imageFile *models.ImageFile) (*uint64, error) {
	img, err := loadImage(encoder, imageFile)
	if err != nil {
		return nil, fmt.Errorf("loading image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("computing phash from image: %w", err)
	}

	hashValue := hash.GetHash()
	return &hashValue, nil
}

// loadImage loads an image from disk and decodes it.
// Where Go has no built-in decoder for a specific format, ffmpeg is used to convert to BMP first.
func loadImage(encoder *ffmpeg.FFMpeg, imageFile *models.ImageFile) (image.Image, error) {
	// try to load with Go's built-in decoders first for better performance
	reader, err := imageFile.Open(&file.OsFS{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if errors.Is(err, image.ErrFormat) {
		// try ffmpeg as a fallback for unsupported formats
		// ffmpeg cannot read files inside zips
		if imageFile.Base().ZipFileID != nil {
			return nil, fmt.Errorf("ffmpeg fallback unsupported for images in zip files")
		}
		return loadImageExternal(encoder, imageFile.Path)
	}

	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	return img, nil
}

func loadImageExternal(encoder *ffmpeg.FFMpeg, path string) (image.Image, error) {
	img, ffmpegErr := loadImageFFmpeg(encoder, path)
	if ffmpegErr == nil {
		return img, nil
	}

	// Some ffmpeg builds can identify JPEG XL but are compiled without a
	// jpegxl decoder. Use the reference decoder when it is available.
	if strings.EqualFold(filepath.Ext(path), ".jxl") {
		img, jxlErr := loadImageJPEGXL(path)
		if jxlErr == nil {
			return img, nil
		}
		return nil, fmt.Errorf("converting image with ffmpeg: %v; JPEG XL fallback: %w", ffmpegErr, jxlErr)
	}

	return nil, fmt.Errorf("converting image with ffmpeg: %w", ffmpegErr)
}

// loadImageFFmpeg uses ffmpeg to convert an image to BMP and then decodes it.
func loadImageFFmpeg(encoder *ffmpeg.FFMpeg, path string) (image.Image, error) {
	options := transcoder.ScreenshotOptions{
		OutputPath: "-",
		OutputType: transcoder.ScreenshotOutputTypeBMP,
	}

	args := transcoder.ScreenshotTime(path, 0, options)
	data, err := encoder.GenerateOutput(context.Background(), args, nil)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding ffmpeg output: %w", err)
	}

	return img, nil
}

// loadImageJPEGXL uses the reference JPEG XL decoder to produce a temporary
// PNG, then lets Go decode that PNG for pHash generation.
func loadImageJPEGXL(path string) (image.Image, error) {
	temp, err := os.CreateTemp("", "stash-jxl-*.png")
	if err != nil {
		return nil, fmt.Errorf("creating temporary JPEG XL output: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("closing temporary JPEG XL output: %w", err)
	}
	defer os.Remove(tempPath)

	command := exec.CommandContext(context.Background(), "djxl", path, tempPath)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return nil, fmt.Errorf("running djxl: %w: %s", err, message)
		}
		return nil, fmt.Errorf("running djxl: %w", err)
	}

	reader, err := os.Open(tempPath)
	if err != nil {
		return nil, fmt.Errorf("opening decoded JPEG XL output: %w", err)
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decoding JPEG XL fallback output: %w", err)
	}
	return img, nil
}
