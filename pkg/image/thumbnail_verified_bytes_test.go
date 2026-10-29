package image

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifiedThumbnailBytesRejectUnsafeOrUnsupportedInputs(t *testing.T) {
	encoder := &ThumbnailEncoder{}
	for _, test := range []struct {
		label  string
		data   []byte
		format string
		size   int
	}{
		{label: "empty", data: nil, format: "png", size: 320},
		{label: "zero-size", data: []byte("x"), format: "png", size: 0},
		{label: "oversize-target", data: []byte("x"), format: "png", size: 513},
		{label: "oversize-source", data: []byte(strings.Repeat("z", 64*1024*1024+1)), format: "png", size: 320},
		{label: "unsupported-jxl", data: []byte("x"), format: "jxl", size: 320},
		{label: "unsupported-avif", data: []byte("x"), format: "avif", size: 320},
		{label: "animated-gif", data: []byte("GIF89a"), format: "gif", size: 320},
		{label: "unsupported-html", data: []byte("x"), format: "html", size: 320},
	} {
		t.Run(test.label, func(t *testing.T) {
			_, err := encoder.GetThumbnailFromVerifiedBytes(test.data, test.format, test.size)
			if err == nil {
				t.Fatal("unsafe or unsupported verified thumbnail accepted")
			}
			if (test.format == "gif" || test.format == "avif" || test.format == "jxl" || test.format == "html") &&
				!errors.Is(err, ErrNotSupportedForThumbnail) {
				t.Fatalf("expected unsupported native thumbnail classification, got %v", err)
			}
		})
	}
}

func TestVerifiedThumbnailExplicitlyRejectsAnimatedWebP(t *testing.T) {
	encoder := &ThumbnailEncoder{}
	// Minimal RIFF/WEBP container carrying a VP8X animation flag.
	animated := []byte{
		'R', 'I', 'F', 'F', 22, 0, 0, 0, 'W', 'E', 'B', 'P',
		'V', 'P', '8', 'X', 10, 0, 0, 0,
		0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	if _, err := encoder.GetThumbnailFromVerifiedBytes(animated, "webp", 320); err == nil {
		t.Fatal("animated WEBP should never enter the static native encoder")
	}
}
