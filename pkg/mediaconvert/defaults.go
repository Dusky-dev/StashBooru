package mediaconvert

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

type InputFormat struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Family string `json:"family"`
}

type EncodingDefaults struct {
	Quality        float64 `json:"quality"`
	Effort         int     `json:"effort"`
	FasterDecoding *int    `json:"fasterDecoding,omitempty"`
}

func ValidateEncodingDefaults(defaults map[string]EncodingDefaults, backend string) error {
	if backend != "" && backend != "auto" && backend != "local" && backend != "remote" {
		return fmt.Errorf("invalid conversion worker")
	}
	for input, value := range defaults {
		known := false
		for _, f := range InputFormats {
			known = known || f.ID == input
		}
		if !known || math.IsNaN(value.Quality) || math.IsInf(value.Quality, 0) || value.Quality < 0 || value.Quality > 100 || value.Effort < 1 || value.Effort > 9 || (value.FasterDecoding != nil && (*value.FasterDecoding < 0 || *value.FasterDecoding > 4)) {
			return fmt.Errorf("invalid encoding defaults for %s", input)
		}
	}
	return nil
}

func (c Config) DefaultEncoding(input string) EncodingDefaults {
	if value, ok := c.EncodingDefaults[input]; ok {
		if value.FasterDecoding == nil {
			value.FasterDecoding = defaultFasterDecoding(input)
		}
		return value
	}
	fallback := "image"
	for _, f := range InputFormats {
		if f.ID == input && f.Family == "video" {
			fallback = "video"
		}
	}
	if value, ok := c.EncodingDefaults[fallback]; ok {
		if value.FasterDecoding == nil {
			value.FasterDecoding = defaultFasterDecoding(input)
		}
		return value
	}
	quality := 90.0
	if fallback == "video" {
		quality = 80
	}
	return EncodingDefaults{Quality: quality, Effort: 7, FasterDecoding: defaultFasterDecoding(input)}
}

func defaultFasterDecoding(input string) *int {
	value := 0
	for _, format := range InputFormats {
		if format.ID == input && format.Family == "animation" {
			value = 2
			break
		}
	}
	return &value
}

func (e EncodingDefaults) FasterDecodingValue() int {
	if e.FasterDecoding == nil {
		return 0
	}
	return *e.FasterDecoding
}

// Catalogs describe configuration choices independently of installed encoders.
var InputFormats = []InputFormat{
	{"image", "Other images", "image"}, {"video", "Other videos", "video"},
	{"jpeg", "JPEG / JPG", "image"}, {"png", "PNG", "image"},
	{"gif", "GIF", "animation"}, {"apng", "Animated PNG", "animation"},
	{"webp", "WebP", "image"}, {"animated-webp", "Animated WebP", "animation"},
	{"jxl", "JPEG XL", "image"}, {"ajxl", "Animated JPEG XL", "animation"},
	{"avif", "AVIF", "image"}, {"animated-avif", "Animated AVIF", "animation"}, {"tiff", "TIFF", "image"}, {"bmp", "BMP", "image"},
	{"heic", "HEIC / HEIF", "image"}, {"ico", "ICO", "image"}, {"exr", "EXR", "image"},
	{"mp4", "MP4 / M4V", "video"}, {"mkv", "MKV", "video"}, {"webm", "WebM", "video"},
	{"mov", "MOV", "video"}, {"avi", "AVI", "video"}, {"wmv", "WMV / ASF", "video"},
	{"flv", "FLV", "video"}, {"mpeg", "MPEG / MPG", "video"}, {"ts", "TS / M2TS", "video"},
	{"ogv", "OGV / OGG", "video"},
}

var OutputFormats = []Format{
	{ID: "jxl", Label: "JPEG XL", Extension: "jxl", Family: "image"},
	{ID: "ajxl", Label: "Animated JPEG XL (AJXL)", Extension: "jxl", Family: "animation"},
	{ID: "av1-mp4", Label: "AV1 / MP4", Extension: "mp4", Family: "video"},
	{ID: "av1-mkv", Label: "AV1 / MKV", Extension: "mkv", Family: "video"},
	{ID: "av1-webm", Label: "AV1 / WebM", Extension: "webm", Family: "video"},
	{ID: "h264", Label: "H.264 / MP4", Extension: "mp4", Family: "video"},
	{ID: "hevc", Label: "HEVC / MP4", Extension: "mp4", Family: "video"},
	{ID: "vp9", Label: "VP9 / WebM", Extension: "webm", Family: "video"},
	{ID: "mov", Label: "H.264 / MOV", Extension: "mov", Family: "video"},
	{ID: "jpeg", Label: "JPEG", Extension: "jpg", Family: "image"},
	{ID: "png", Label: "PNG", Extension: "png", Family: "image"},
	{ID: "webp", Label: "WebP (still or animated)", Extension: "webp", Family: "animation"},
	{ID: "avif", Label: "AVIF", Extension: "avif", Family: "image"},
	{ID: "gif", Label: "GIF", Extension: "gif", Family: "animation"},
	{ID: "apng", Label: "Animated PNG", Extension: "png", Family: "animation"},
	{ID: "tiff", Label: "TIFF", Extension: "tiff", Family: "image"},
	{ID: "bmp", Label: "BMP", Extension: "bmp", Family: "image"},
}

func DefaultFormatDefaults() map[string]string {
	return map[string]string{
		"image": "jxl", "video": "av1-mp4", "jpeg": "jxl", "png": "jxl",
		"gif": "ajxl", "apng": "ajxl", "webp": "jxl", "animated-webp": "ajxl", "jxl": "jxl", "ajxl": "ajxl", "animated-avif": "ajxl",
		"mp4": "av1-mp4", "mkv": "av1-mkv", "webm": "av1-webm",
	}
}

func ValidateFormatDefaults(defaults map[string]string) error {
	for input, output := range defaults {
		var source InputFormat
		var target Format
		for _, f := range InputFormats {
			if f.ID == input {
				source = f
			}
		}
		for _, f := range OutputFormats {
			if f.ID == output {
				target = f
			}
		}
		if source.ID == "" || target.ID == "" {
			return fmt.Errorf("unknown format default: %s → %s", input, output)
		}
		if source.Family == "video" && target.Family != "video" {
			return fmt.Errorf("%s requires a video output", source.Label)
		}
		if source.Family == "animation" && target.Family == "image" {
			return fmt.Errorf("%s requires an output that preserves animation", source.Label)
		}
	}
	return nil
}

func (c Config) DefaultOutput(input string, video bool) string {
	if output := c.FormatDefaults[input]; output != "" {
		return output
	}
	// New animation subtypes must remain safe with configs saved before those
	// subtype rules existed. They cannot fall through to a still-only fallback.
	if input == "ajxl" || input == "animated-avif" {
		return "ajxl"
	}
	fallback, output := "image", "jxl"
	if video {
		fallback, output = "video", "av1-mp4"
	}
	if configured := c.FormatDefaults[fallback]; configured != "" {
		return configured
	}
	return output
}

// SourceFormat distinguishes animations that share an extension with stills.
// Only container headers are read; decoding/encoding stays on the chosen worker.
func SourceFormat(file models.File) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Base().Path), "."))
	if file.Base().FrameCount > 1 {
		switch ext {
		case "png", "apng":
			return "apng"
		case "webp":
			return "animated-webp"
		case "jxl":
			return "ajxl"
		case "avif", "avifs":
			return "animated-avif"
		}
	}
	if ext == "jxl" && file.Base().FrameCount == 0 {
		return "ajxl" // uninspected JPEG XL must not silently select a still-only output
	}
	switch ext {
	case "jpg", "jfif":
		return "jpeg"
	case "tif":
		return "tiff"
	case "m4v":
		return "mp4"
	case "mpg":
		return "mpeg"
	case "m2ts", "mts":
		return "ts"
	case "asf":
		return "wmv"
	case "ogg":
		return "ogv"
	case "heif":
		return "heic"
	case "png", "webp":
		f, err := os.Open(file.Base().Path)
		if err != nil {
			return ext
		}
		defer f.Close()
		if ext == "webp" {
			var header [21]byte
			if _, err := io.ReadFull(f, header[:]); err == nil && string(header[:4]) == "RIFF" && string(header[8:16]) == "WEBPVP8X" && header[20]&2 != 0 {
				return "animated-webp"
			}
		} else {
			var header [8]byte
			if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:]) != "\x89PNG\r\n\x1a\n" {
				return ext
			}
			for range 512 {
				if _, err := io.ReadFull(f, header[:]); err != nil {
					break
				}
				switch string(header[4:]) {
				case "acTL":
					return "apng"
				case "IDAT", "IEND":
					return ext
				}
				if _, err := f.Seek(int64(binary.BigEndian.Uint32(header[:4]))+4, io.SeekCurrent); err != nil {
					break
				}
			}
		}
	}
	return ext
}

// Matches libjxl's JxlEncoderDistanceFromQuality, including lossless at 100.
// https://github.com/libjxl/libjxl/blob/main/lib/jxl/encode.cc
func JXLDistanceFromQuality(quality float64) float64 {
	switch {
	case quality >= 100:
		return 0
	case quality >= 30:
		return 0.1 + (100-quality)*0.09
	default:
		return 53.0/3000*quality*quality - 23.0/20*quality + 25
	}
}
