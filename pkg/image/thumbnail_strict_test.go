package image

import "testing"

func TestJXLSelectionFailsWithoutEncoderRatherThanFallingBack(t *testing.T) {
	e := ThumbnailEncoder{ThumbnailFormat: "jxl", ThumbnailQuality: 70}
	options, err := e.ffmpegThumbnailOptions(320)
	if err == nil {
		t.Fatalf("expected unsupported JXL encoder error, got options %+v", options)
	}
}
