package image

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

var avifVipsProbe struct {
	once sync.Once
	err  error
}

// A valid libvips binary need not have libheif or an AV1 encoder installed.
// Probe the real output once and cache the result for normal thumb generation.
func vipsAVIFSupported() bool {
	avifVipsProbe.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		avifVipsProbe.err = PreflightAVIFThumbnail(ctx)
	})
	return avifVipsProbe.err == nil
}

func avifQualityCRF(quality int) int {
	if quality < 10 || quality > 100 {
		quality = 70
	}
	// AV1 CRF uses a different scale than libvips Q. 0 is best; ~50 is
	// aggressive compression. Preserve the user's monotonic quality control.
	return (100 - quality) * 55 / 100
}

func isPNGEncoded(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
}

func isJXLSource(f models.File) bool {
	if img, ok := f.(*models.ImageFile); ok {
		if strings.EqualFold(img.Format, "jpegxl") || strings.EqualFold(img.Format, "jxl") {
			return true
		}
	}
	return strings.EqualFold(filepath.Ext(f.Base().Path), ".jxl")
}

func isAVIFEncoded(data []byte) bool {
	// ISO BMFF files identify their format near the start of the ftyp box.
	return len(data) >= 16 && bytes.Contains(data[:min(len(data), 64)], []byte("ftypavif"))
}

// ffmpegAVIFThumbnail creates a temporary *seekable* file: FFmpeg's AVIF
// muxer rejects pipe:1. The temporary file is distinct from the generated
// thumbnail cache and removed whether encoding succeeds or fails.
func (e *ThumbnailEncoder) ffmpegAVIFThumbnail(
	ctx context.Context, input io.Reader, inputPath string, maxSize int,
) ([]byte, error) {
	if e.FFMpeg == nil {
		return nil, fmt.Errorf("FFmpeg unavailable for AVIF encoding")
	}
	if maxSize < 1 || maxSize > 2048 {
		return nil, fmt.Errorf("invalid AVIF thumbnail size %d", maxSize)
	}
	tmp, err := os.CreateTemp("", "stash-avif-thumbnail-*.avif")
	if err != nil {
		return nil, fmt.Errorf("creating temporary AVIF: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("closing temporary AVIF: %w", err)
	}

	source := "pipe:0"
	if inputPath != "" {
		source = inputPath
	}
	var scale ffmpeg.VideoFilter
	scale = scale.ScaleMaxSize(maxSize)
	args := ffmpeg.Args{"-hide_banner", "-loglevel", "error", "-y", "-i", source}
	args = args.VideoFilter(scale)
	args = append(args,
		"-frames:v", "1",
		"-c:v", "libaom-av1",
		"-cpu-used", "4",
		"-still-picture", "1",
		"-crf", strconv.Itoa(avifQualityCRF(e.ThumbnailQuality)),
		"-b:v", "0",
		"-pix_fmt", "yuv444p",
		"-f", "avif",
		path,
	)
	cmd := e.FFMpeg.Command(ctx, args)
	cmd.Stdin = input
	result, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(result))
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return nil, fmt.Errorf("FFmpeg AVIF encoding failed: %s: %w", msg, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading temporary AVIF: %w", err)
	}
	if !isAVIFEncoded(data) {
		return nil, fmt.Errorf("FFmpeg did not generate a valid AVIF image")
	}
	return data, nil
}

// PreflightAVIFWithFFmpeg tests the same AVIF output path used by the
// thumbnail encoder. Neither a libvips operation name nor the presence of
// an FFmpeg codec alone proves that an AVIF image can be generated.
func PreflightAVIFWithFFmpeg(ctx context.Context, ff *ffmpeg.FFMpeg) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if vipsAVIFSupported() {
		return nil
	}

	// An opaque PNG probe is independent of user media and the cache.
	var src bytes.Buffer
	pixel := stdimage.NewRGBA(stdimage.Rect(0, 0, 16, 16))
	for i := 3; i < len(pixel.Pix); i += 4 {
		pixel.Pix[i] = 255
	}
	if err := png.Encode(&src, pixel); err != nil {
		return fmt.Errorf("creating AVIF preflight input: %w", err)
	}
	encoder := ThumbnailEncoder{FFMpeg: ff, ThumbnailQuality: 60}
	_, err := encoder.ffmpegAVIFThumbnail(ctx, bytes.NewReader(src.Bytes()), "", 16)
	if err != nil {
		return fmt.Errorf("AVIF is unavailable: libvips cannot encode AVIF and FFmpeg with libaom-av1 + AVIF muxer also failed (%v); thumbnails unchanged", err)
	}
	return nil
}

// ffmpegAVIFViaVips decodes and downsizes an input using libvips to a
// lossless PNG, then uses FFmpeg only for AV1 encoding. This covers sources
// such as JPEG XL when FFmpeg was built without a corresponding decoder.
// It deliberately never uses a pre-existing thumbnail or lossy JPEG.
func (e *ThumbnailEncoder) ffmpegAVIFViaVips(f models.File, original []byte, maxSize int) ([]byte, error) {
	if e.vips == nil {
		return nil, fmt.Errorf("libvips is required to decode the source image when FFmpeg lacks its decoder")
	}
	var pngData []byte
	var err error
	if runtime.GOOS == "windows" && f.Base().ZipFileID == nil {
		// vips can't reliably use stdin for thumbnailing on Windows.
		pngData, err = e.vips.ImageThumbnailPathWithOptions(f.Base().Path, maxSize, "png", 100)
	} else {
		pngData, err = e.vips.ImageThumbnailWithOptions(bytes.NewBuffer(original), maxSize, "png", 100)
	}
	if err != nil {
		return nil, fmt.Errorf("libvips could not decode source image for AVIF: %w", err)
	}
	if !isPNGEncoded(pngData) {
		return nil, fmt.Errorf("libvips returned non-PNG data for AVIF fallback")
	}
	return e.ffmpegAVIFThumbnail(context.TODO(), bytes.NewReader(pngData), "", maxSize)
}

// getAVIFThumbnail prefers libvips when it supports AVIF (including alpha),
// otherwise it uses a native AVIF encode from the original media via FFmpeg.
// It never re-encodes an existing JPEG thumbnail.
func (e *ThumbnailEncoder) getAVIFThumbnail(f models.File, maxSize int) ([]byte, error) {
	preferVips := e.vips != nil && vipsAVIFSupported()
	if _, isVideo := f.(*models.VideoFile); isVideo {
		if preferVips {
			poster, err := e.ffmpegLosslessPosterPath(f.Base().Path, maxSize)
			if err == nil {
				out, saveErr := e.vips.ImageThumbnailWithOptions(bytes.NewBuffer(poster), maxSize, "avif", e.ThumbnailQuality)
				if saveErr == nil && isAVIFEncoded(out) {
					return out, nil
				}
			}
		}
		return e.ffmpegAVIFThumbnail(context.TODO(), nil, f.Base().Path, maxSize)
	}

	animated, err := IsAnimatedSource(f)
	if err != nil {
		return nil, err
	}
	if animated {
		return nil, fmt.Errorf("%w: animated sources are not still AVIF thumbnails", ErrNotSupportedForThumbnail)
	}
	if img, ok := f.(*models.ImageFile); ok && img.Format == "avif" {
		if f.Base().ZipFileID != nil {
			return nil, fmt.Errorf("%w: AVIF input in ZIP", ErrNotSupportedForThumbnail)
		}
		if preferVips {
			data, err := e.vips.ImageThumbnailPathWithOptions(f.Base().Path, maxSize, "avif", e.ThumbnailQuality)
			if err == nil && isAVIFEncoded(data) {
				return data, nil
			}
		}
		return e.ffmpegAVIFThumbnail(context.TODO(), nil, f.Base().Path, maxSize)
	}

	reader, err := f.Open(&file.OsFS{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	if preferVips {
		if runtime.GOOS != "windows" {
			data, err := e.vips.ImageThumbnailWithOptions(bytes.NewBuffer(buf.Bytes()), maxSize, "avif", e.ThumbnailQuality)
			if err == nil && isAVIFEncoded(data) {
				return data, nil
			}
		} else if f.Base().ZipFileID == nil {
			data, err := e.vips.ImageThumbnailPathWithOptions(f.Base().Path, maxSize, "avif", e.ThumbnailQuality)
			if err == nil && isAVIFEncoded(data) {
				return data, nil
			}
		}
	}
	// JPEG XL may be supported by libvips even when FFmpeg has no decoder.
	// Skip a doomed FFmpeg decode attempt for this known source type.
	if isJXLSource(f) {
		return e.ffmpegAVIFViaVips(f, buf.Bytes(), maxSize)
	}

	out, encodeErr := e.ffmpegAVIFThumbnail(context.TODO(), bytes.NewReader(buf.Bytes()), "", maxSize)
	if encodeErr == nil {
		return out, nil
	}
	if e.vips == nil {
		return nil, encodeErr
	}
	// A different source codec may also be missing from FFmpeg; try the
	// libvips lossless decode bridge before returning a codec error.
	out, decodeErr := e.ffmpegAVIFViaVips(f, buf.Bytes(), maxSize)
	if decodeErr != nil {
		return nil, fmt.Errorf("AVIF thumbnail failed via FFmpeg (%v) and libvips PNG bridge: %w", encodeErr, decodeErr)
	}
	return out, nil
}
