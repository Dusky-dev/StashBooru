package transcoder

import (
	"errors"
	"strconv"

	"github.com/stashapp/stash/pkg/ffmpeg"
)

var ErrUnsupportedFormat = errors.New("unsupported image format")

type ImageThumbnailOptions struct {
	InputFormat   ffmpeg.ImageFormat
	OutputFormat  ffmpeg.ImageFormat
	OutputPath    string
	MaxDimensions int
	Quality       int
}

func ImageThumbnail(input string, options ImageThumbnailOptions) ffmpeg.Args {
	var videoFilter ffmpeg.VideoFilter
	videoFilter = videoFilter.ScaleMaxSize(options.MaxDimensions)

	var args ffmpeg.Args
	args = append(args, "-hide_banner")
	args = args.LogLevel(ffmpeg.LogLevelError)

	args = args.Overwrite().
		ImageFormat(options.InputFormat).
		Input(input).
		VideoFilter(videoFilter)

	if options.OutputFormat == ffmpeg.ImageFormatWebp {
		args = args.VideoCodec(ffmpeg.VideoCodecLibWebP)
	} else {
		args = args.VideoCodec(ffmpeg.VideoCodecMJpeg)
	}

	args = append(args, "-frames:v", "1")

	if options.Quality > 0 {
		if options.OutputFormat == ffmpeg.ImageFormatWebp {
			args = append(args, "-q:v", strconv.Itoa(options.Quality))
		} else {
			args = args.FixedQualityScaleVideo(options.Quality)
		}
	}

	args = args.ImageFormat(ffmpeg.ImageFormatImage2Pipe).
		Output(options.OutputPath).
		ImageFormat(options.OutputFormat)

	return args
}
