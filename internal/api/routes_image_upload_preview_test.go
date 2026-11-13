package api

import (
	"net/http/httptest"
	"testing"
)

func TestUploadPreviewJPEGDoesNotChangeNormalThumbnailPreferences(t *testing.T) {
	for _, tc := range []struct {
		name, rawURL, format string
		quality              int
		wantFormat           string
		wantQuality          int
		wantPreview          bool
	}{
		{"configured JXL stays JXL", "/image/7/thumbnail", "jxl", 90, "jxl", 90, false},
		{"configured AVIF stays AVIF", "/image/7/thumbnail?uploadPreview=other", "avif", 80, "avif", 80, false},
		{"queue forces JPEG from JXL", "/image/7/thumbnail?uploadPreview=jpeg", "jxl", 90, "jpg", 70, true},
		{"queue forces JPEG from animated WebP", "/image/7/thumbnail?uploadPreview=jpeg", "webp", 75, "jpg", 70, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", tc.rawURL, nil)
			format, quality, preview := thumbnailEncodingForUploadPreview(request, tc.format, tc.quality)
			if format != tc.wantFormat || quality != tc.wantQuality || preview != tc.wantPreview {
				t.Fatalf("got %q/%d preview=%v, want %q/%d preview=%v", format, quality, preview, tc.wantFormat, tc.wantQuality, tc.wantPreview)
			}
		})
	}
}
