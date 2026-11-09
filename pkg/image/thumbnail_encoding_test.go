package image

import "testing"

func TestVipsThumbnailOutputFormats(t *testing.T) {
	for _, tc := range []struct{ format, expected string }{
		{"jpg", ".jpg[Q=70,strip]"},
		{"webp", ".webp[Q=70,strip]"},
		{"jxl", ".jxl[Q=70,strip]"},
		{"bad", ".jpg[Q=70,strip]"},
	} {
		if got := vipsThumbnailOutput(tc.format, 70); got != tc.expected {
			t.Errorf("%s: got %q want %q", tc.format, got, tc.expected)
		}
	}
}
