package image

import (
	"os"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

// CaptureDate reads when a photo was taken from its EXIF DateTimeOriginal
// tag (not DateTime, which editors rewrite). It returns false when the file
// has no such tag or the value is unusable. The day is the camera's own,
// with no time-zone conversion.
func CaptureDate(path string, now time.Time) (time.Time, bool) {
	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer file.Close()
	x, err := exif.Decode(file)
	if err != nil {
		return time.Time{}, false
	}
	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil || tag.Format() != tiff.StringVal {
		return time.Time{}, false
	}
	return parseCaptureDate(string(tag.Val), now)
}

func parseCaptureDate(raw string, now time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(strings.TrimRight(raw, "\x00"))
	at, err := time.Parse("2006:01:02 15:04:05", raw)
	if err != nil || at.Year() < 1900 || at.After(now.Add(24*time.Hour)) {
		return time.Time{}, false
	}
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), true
}
