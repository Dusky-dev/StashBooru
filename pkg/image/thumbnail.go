package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/models"
)

const defaultThumbnailQuality = 70

var vipsPath string
var once sync.Once

// ErrNotSupportedForThumbnail is returned if the image format is not supported for thumbnail generation
var ErrNotSupportedForThumbnail = errors.New("unsupported image format for thumbnail")

type ThumbnailEncoder struct {
	FFMpeg             *ffmpeg.FFMpeg
	FFProbe            *ffmpeg.FFProbe
	ClipPreviewOptions ClipPreviewOptions
	ThumbnailFormat    string
	ThumbnailQuality   int
	vips               *vipsEncoder
}

type ClipPreviewOptions struct {
	InputArgs  []string
	OutputArgs []string
	Preset     string
	Format     string
	Quality    int
}

func GetVipsPath() string {
	once.Do(func() {
		vipsPath, _ = exec.LookPath("vips")
	})
	return vipsPath
}

func NewThumbnailEncoder(ffmpegEncoder *ffmpeg.FFMpeg, ffProbe *ffmpeg.FFProbe, clipPreviewOptions ClipPreviewOptions) ThumbnailEncoder {
	ret := ThumbnailEncoder{
		FFMpeg:             ffmpegEncoder,
		FFProbe:            ffProbe,
		ClipPreviewOptions: clipPreviewOptions,
		ThumbnailFormat:    "jpg",
		ThumbnailQuality:   defaultThumbnailQuality,
	}

	vipsPath := GetVipsPath()
	if vipsPath != "" {
		vipsEncoder := vipsEncoder(vipsPath)
		ret.vips = &vipsEncoder
	}

	return ret
}

// IsAnimatedSource identifies GIF and animated WebP without reading an
// entire source image into memory. It does not alter source files.
func IsAnimatedSource(f models.File) (bool, error) {
	imageFile, ok := f.(*models.ImageFile)
	if !ok {
		return false, nil
	}
	if imageFile.Format == formatGif {
		return true, nil
	}
	if imageFile.Format != formatWebP {
		return false, nil
	}
	reader, err := f.Open(&file.OsFS{})
	if err != nil {
		return false, err
	}
	defer reader.Close()
	buf := make([]byte, 48)
	n, err := io.ReadFull(reader, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	return isWebPAnimated(buf[:n]), nil
}

// GetAnimatedThumbnail generates a reduced-frame-rate animated WebP preview.
// It never writes or replaces the original GIF/WebP source.
func (e *ThumbnailEncoder) GetAnimatedThumbnail(f models.File, maxSize, quality int) ([]byte, error) {
	animated, err := IsAnimatedSource(f)
	if err != nil {
		return nil, err
	}
	if !animated {
		return nil, fmt.Errorf("%w: source is not animated GIF/WebP", ErrNotSupportedForThumbnail)
	}
	if quality < 10 || quality > 100 {
		quality = 70
	}
	reader, err := f.Open(&file.OsFS{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if e.FFMpeg == nil {
		return nil, fmt.Errorf("FFmpeg is required for animated WebP thumbnails")
	}
	args := ffmpeg.Args{
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("fps=12,scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease:flags=lanczos", maxSize, maxSize),
		"-an", "-c:v", "libwebp_anim", "-quality", fmt.Sprint(quality),
		"-loop", "0", "-f", "webp", "pipe:1",
	}
	return e.FFMpeg.GenerateOutput(context.TODO(), args, reader)
}

// GetThumbnail returns the thumbnail image of the provided image resized to
// the provided max size. It resizes based on the largest X/Y direction.
// It returns nil and an error if an error occurs reading, decoding or encoding
// the image, or if the image is not suitable for thumbnails.
func (e *ThumbnailEncoder) GetThumbnail(f models.File, maxSize int) ([]byte, error) {
	// Video-backed image clips can be large, so avoid buffering the whole file.
	if _, ok := f.(*models.VideoFile); ok {
		if e.ThumbnailFormat == "jxl" || e.ThumbnailFormat == "avif" {
			// Extract a lossless PNG frame, avoiding a lossy JPEG intermediate
			// before encoding JXL/AVIF poster thumbnails for video-backed Images.
			if e.vips == nil {
				return nil, fmt.Errorf("%w: %s poster frame encoding requires libvips", ErrNotSupportedForThumbnail, e.ThumbnailFormat)
			}
			poster, err := e.ffmpegLosslessPosterPath(f.Base().Path, maxSize)
			if err != nil {
				return nil, err
			}
			return e.vips.ImageThumbnailWithOptions(bytes.NewBuffer(poster), maxSize, e.ThumbnailFormat, e.ThumbnailQuality)
		}
		return e.ffmpegImageThumbnailPath(f.Base().Path, maxSize)
	}

	reader, err := f.Open(&file.OsFS{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}

	data := buf.Bytes()

	format := ""
	if imageFile, ok := f.(*models.ImageFile); ok {
		format = imageFile.Format
		animated := imageFile.Format == formatGif

		// #2266 - if image is webp, then determine if it is animated
		if format == formatWebP {
			animated = isWebPAnimated(data)
		}

		// #2266 - don't generate a thumbnail for animated images
		if animated {
			return nil, fmt.Errorf("%w: %s", ErrNotSupportedForThumbnail, format)
		}

		// AVIF cannot be read from stdin, must use file path
		// AVIF in zip files is not supported
		// Note: No Windows check needed here since we use file path, not stdin
		if format == "avif" {
			if f.Base().ZipFileID != nil {
				return nil, fmt.Errorf("%w: AVIF in zip file", ErrNotSupportedForThumbnail)
			}
			if e.vips != nil {
				return e.vips.ImageThumbnailPathWithOptions(f.Base().Path, maxSize, e.ThumbnailFormat, e.ThumbnailQuality)
			}
			return e.ffmpegImageThumbnailPath(f.Base().Path, maxSize)
		}
	}

	// vips has issues loading files from stdin on Windows
	if e.vips != nil {
		if runtime.GOOS == "windows" && f.Base().ZipFileID == nil {
			return e.vips.ImageThumbnailPathWithOptions(f.Base().Path, maxSize, e.ThumbnailFormat, e.ThumbnailQuality)
		}
		if runtime.GOOS != "windows" {
			return e.vips.ImageThumbnailWithOptions(buf, maxSize, e.ThumbnailFormat, e.ThumbnailQuality)
		}
	}
	return e.ffmpegImageThumbnail(buf, maxSize)
}

// GetThumbnailFromVerifiedBytes runs the same vips/FFmpeg thumbnail backends
// against already-verified static image bytes, without touching any source or
// generated cache file. This is only used by the isolated experimental shadow
// probe, not by any of the normal native thumbnail routes.
//
// Animated GIF/WebP, AVIF (which needs seekable path access), and unsupported
// formats remain explicitly excluded, matching native thumbnail limitations.
func (e *ThumbnailEncoder) GetThumbnailFromVerifiedBytes(data []byte, format string, maxSize int) ([]byte, error) {
	if len(data) == 0 || len(data) > 64*1024*1024 || maxSize <= 0 || maxSize > 512 {
		return nil, fmt.Errorf("invalid bounded thumbnail input")
	}
	if format == formatGif || (format == formatWebP && isWebPAnimated(data)) ||
		format == "avif" || format == "jxl" {
		return nil, fmt.Errorf("%w: %s", ErrNotSupportedForThumbnail, format)
	}
	switch format {
	case "png", "jpg", "jpeg", "webp", "bmp", "tiff":
	default:
		return nil, fmt.Errorf("%w: %s", ErrNotSupportedForThumbnail, format)
	}
	input := bytes.NewBuffer(data)
	if e.vips != nil && runtime.GOOS != "windows" {
		return e.vips.ImageThumbnail(input, maxSize)
	}
	return e.ffmpegImageThumbnail(input, maxSize)
}

// GetPreview returns the preview clip of the provided image clip resized to
// the provided max size. It resizes based on the largest X/Y direction.
// It is hardcoded to 30 seconds maximum right now
func (e *ThumbnailEncoder) GetPreview(inPath string, outPath string, maxSize int) error {
	fileData, err := e.FFProbe.NewVideoFile(inPath)
	if err != nil {
		return err
	}
	if fileData.Width <= maxSize {
		maxSize = fileData.Width
	}
	clipDuration := fileData.VideoStreamDuration
	if clipDuration > 30.0 {
		clipDuration = 30.0
	}
	return e.getClipPreview(inPath, outPath, maxSize, clipDuration, fileData.FrameRate)
}

func (e *ThumbnailEncoder) ffmpegThumbnailOptions(maxSize int) (transcoder.ImageThumbnailOptions, error) {
	format, quality := e.ThumbnailFormat, e.ThumbnailQuality
	if quality < 10 || quality > 100 {
		quality = defaultThumbnailQuality
	}
	if format == "jxl" || format == "avif" {
		return transcoder.ImageThumbnailOptions{}, fmt.Errorf("%w: %s encoding requires libvips", ErrNotSupportedForThumbnail, format)
	}
	opts := transcoder.ImageThumbnailOptions{
		OutputFormat:  ffmpeg.ImageFormatJpeg,
		OutputPath:    "-",
		MaxDimensions: maxSize,
		Quality:       2 + (100-quality)*(100-quality)/300,
	}
	if opts.Quality > 31 {
		opts.Quality = 31
	}
	if format == "webp" {
		opts.OutputFormat = ffmpeg.ImageFormatWebp
		opts.Quality = quality
	}
	return opts, nil
}

func (e *ThumbnailEncoder) ffmpegImageThumbnail(image *bytes.Buffer, maxSize int) ([]byte, error) {
	options, err := e.ffmpegThumbnailOptions(maxSize)
	if err != nil {
		return nil, err
	}
	return e.FFMpeg.GenerateOutput(context.TODO(), transcoder.ImageThumbnail("-", options), image)
}

// ffmpegLosslessPosterPath extracts a single frame at thumbnail dimensions
// without introducing JPEG artifacts before libvips AVIF/JXL encoding.
func (e *ThumbnailEncoder) ffmpegLosslessPosterPath(inputPath string, maxSize int) ([]byte, error) {
	var scale ffmpeg.VideoFilter
	scale = scale.ScaleMaxSize(maxSize)
	args := ffmpeg.Args{"-hide_banner", "-loglevel", "error", "-i", inputPath}
	args = args.VideoFilter(scale)
	args = append(args, "-frames:v", "1", "-c:v", "png", "-f", "image2pipe", "pipe:1")
	return e.FFMpeg.GenerateOutput(context.TODO(), args, nil)
}

func (e *ThumbnailEncoder) ffmpegImageThumbnailPath(inputPath string, maxSize int) ([]byte, error) {
	options, err := e.ffmpegThumbnailOptions(maxSize)
	if err != nil {
		return nil, err
	}
	return e.FFMpeg.GenerateOutput(context.TODO(), transcoder.ImageThumbnail(inputPath, options), nil)
}

func (e *ThumbnailEncoder) getClipPreview(inPath string, outPath string, maxSize int, clipDuration float64, frameRate float64) error {
	var thumbFilter ffmpeg.VideoFilter
	thumbFilter = thumbFilter.ScaleMaxSize(maxSize)

	var thumbArgs ffmpeg.Args
	thumbArgs = thumbArgs.VideoFilter(thumbFilter)

	o := e.ClipPreviewOptions

	format := o.Format
	if format != "mp4" {
		format = "webm"
	}
	quality := o.Quality
	if quality < 10 || quality > 100 {
		quality = 75
	}
	crf := 25 + (75-quality)/3
	if crf < 15 {
		crf = 15
	}
	if crf > 45 {
		crf = 45
	}
	codec := ffmpeg.VideoCodecVP9
	if format == "mp4" {
		codec = ffmpeg.VideoCodecLibX264
	}
	thumbArgs = append(thumbArgs,
		"-pix_fmt", "yuv420p",
		"-preset", o.Preset,
		"-crf", fmt.Sprint(crf),
		"-threads", "4",
		"-strict", "-2",
		"-f", format,
	)

	if frameRate <= 0.01 {
		thumbArgs = append(thumbArgs, "-vsync", "2")
	}

	thumbOptions := transcoder.TranscodeOptions{
		OutputPath: outPath,
		StartTime:  0,
		Duration:   clipDuration,

		XError:   true,
		SlowSeek: false,

		VideoCodec: codec,
		VideoArgs:  thumbArgs,

		ExtraInputArgs:  o.InputArgs,
		ExtraOutputArgs: o.OutputArgs,
	}

	if err := fsutil.EnsureDirAll(filepath.Dir(outPath)); err != nil {
		return err
	}
	args := transcoder.Transcode(inPath, thumbOptions)
	return e.FFMpeg.Generate(context.TODO(), args)
}
