package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"image/png"
	stdexec "os/exec"
	"strings"

	"github.com/stashapp/stash/pkg/exec"
	"github.com/stashapp/stash/pkg/logger"
)

type vipsEncoder string

func (e *vipsEncoder) ImageThumbnail(image *bytes.Buffer, maxSize int) ([]byte, error) {
	return e.ImageThumbnailWithOptions(image, maxSize, "jpg", defaultThumbnailQuality)
}

func vipsThumbnailOutput(format string, quality int) string {
	switch format {
	case "webp", "jxl", "avif", "png":
	default:
		format = "jpg"
	}
	if quality < 10 || quality > 100 {
		quality = defaultThumbnailQuality
	}
	if format == "png" {
		// Lossless intermediary when an image decoder exists in libvips but
		// not in FFmpeg. Never use a JPEG intermediary for AVIF.
		return ".png[strip]"
	}
	if format == "avif" {
		// At thumbnail sizes, 4:4:4 preserves colored outlines in artwork.
		// Effort 4 avoids the exceptionally slow AV1 high-effort presets.
		return fmt.Sprintf(".avif[Q=%d,effort=4,subsample_mode=off,bitdepth=8,strip]", quality)
	}
	return fmt.Sprintf(".%s[Q=%d,strip]", format, quality)
}

func (e *vipsEncoder) ImageThumbnailWithOptions(image *bytes.Buffer, maxSize int, format string, quality int) ([]byte, error) {
	args := []string{
		"thumbnail_source",
		"[descriptor=0]",
		vipsThumbnailOutput(format, quality),
		fmt.Sprint(maxSize),
		"--size", "down",
	}
	data, err := e.run(args, image)

	return []byte(data), err
}

// ImageThumbnailPath generates a thumbnail from a file path instead of stdin.
// This is required for formats like AVIF that need random file access (seeking)
// which stdin cannot provide.
func (e *vipsEncoder) ImageThumbnailPath(path string, maxSize int) ([]byte, error) {
	return e.ImageThumbnailPathWithOptions(path, maxSize, "jpg", defaultThumbnailQuality)
}

func (e *vipsEncoder) ImageThumbnailPathWithOptions(path string, maxSize int, format string, quality int) ([]byte, error) {
	// vips thumbnail syntax: thumbnail input output width [options]
	// Using .jpg[Q=70,strip] as output writes to stdout
	args := []string{
		"thumbnail",
		path,
		vipsThumbnailOutput(format, quality),
		fmt.Sprint(maxSize),
		"--size", "down",
	}

	cmd := exec.Command(string(*e), args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		logger.Errorf("image encoder error when running command <%s>: %s", strings.Join(cmd.Args, " "), stderr.String())
		return nil, err
	}

	return stdout.Bytes(), nil
}

func (e *vipsEncoder) run(args []string, stdin *bytes.Buffer) (string, error) {
	cmd := exec.Command(string(*e), args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	if err := cmd.Start(); err != nil {
		return "", err
	}

	err := cmd.Wait()

	if err != nil {
		// error message should be in the stderr stream
		logger.Errorf("image encoder error when running command <%s>: %s", strings.Join(cmd.Args, " "), stderr.String())
		return stdout.String(), err
	}

	return stdout.String(), nil
}

// PreflightAVIFThumbnail proves this libvips installation has a working AV1
// encoder, not merely the heifsave operation (which can support HEIC only).
// Nothing is written to the generated thumbnail cache.
func PreflightAVIFThumbnail(ctx context.Context) error {
	vips := GetVipsPath()
	if vips == "" {
		return fmt.Errorf("AVIF thumbnail encoding requires libvips")
	}
	var input bytes.Buffer
	sample := stdimage.NewRGBA(stdimage.Rect(0, 0, 16, 16))
	for i := 3; i < len(sample.Pix); i += 4 {
		sample.Pix[i] = 255 // probe an opaque thumbnail, not alpha-only AVIF support
	}
	if err := png.Encode(&input, sample); err != nil {
		return fmt.Errorf("preparing AVIF encoder test: %w", err)
	}
	cmd := stdexec.CommandContext(ctx, vips,
		"thumbnail_source", "[descriptor=0]",
		vipsThumbnailOutput("avif", 60), "16", "--size", "down")
	cmd.Stdin = &input
	out, err := cmd.Output()
	if err != nil {
		var exit *stdexec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("AVIF encoder unavailable (%s); thumbnails unchanged: %w", strings.TrimSpace(string(exit.Stderr)), err)
		}
		return fmt.Errorf("AVIF encoder unavailable; thumbnails unchanged: %w", err)
	}
	// The AVIF brand is mandatory for a still AVIF bitstream, even when
	// the system has a separate HEIF/HEIC encoder installed.
	if len(out) < 16 || !bytes.Contains(out[:min(len(out), 64)], []byte("ftypavif")) {
		return fmt.Errorf("AVIF encoder returned non-AVIF output; thumbnails unchanged")
	}
	return nil
}
