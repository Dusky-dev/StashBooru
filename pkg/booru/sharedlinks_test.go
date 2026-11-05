package booru

import (
	"strings"
	"testing"
)

func TestSharedLinkPostSupported(t *testing.T) {
	cases := []struct {
		url, format string
	}{
		{"https://www.dropbox.com/scl/fi/abcDEF123/scene.mp4?rlkey=abcdef123456&dl=0", "mp4"},
		{"https://dropbox.com/s/abcDEF123/photo.png?dl=0", "png"},
		{"https://www.dropbox.com/scl/fo/abcDEF123/folder?rlkey=abcdef123456", "zip"},
		{"https://mega.nz/file/abcDEF123#abcdef1234567890", ""},
		{"https://mega.nz/#!abcDEF123!abcdef1234567890", ""},
	}
	for _, tc := range cases {
		post, err := SharedLinkPost(tc.url)
		if err != nil {
			t.Fatalf("shared link %q: %v", tc.url, err)
		}
		if post.Format != tc.format || !post.DownloadAvailable ||
			post.SourceID != SharedLinkSource().ID || post.OriginalURL == "" ||
			!ValidPostID(post.ID) || strings.Contains(post.PostURL, "#") ||
			strings.Contains(post.PostURL, "rlkey=") {
			t.Errorf("invalid safe shared post for %q: %+v", tc.url, post)
		}
	}
}

func TestSharedLinkPostRejectsInvalidAndPrivateURLs(t *testing.T) {
	cases := []string{
		"http://www.dropbox.com/s/abcDEF123/scene.mp4",
		"https://dropbox.com.evil.test/s/abcDEF123/scene.mp4",
		"https://www.dropbox.com/scl/fi/abcDEF123/a.jpg?token=secret",
		"https://www.dropbox.com/s/abcDEF123/a.jpg#fragment",
		"https://www.dropbox.com/s/abcDEF123/malware.exe",
		"https://mega.nz/folder/abcDEF123#abcdef1234567890",
		"https://mega.nz/file/abcDEF123",
		"https://mega.nz/file/abcDEF123#not*base64",
		"https://mega.nz:8443/file/abcDEF123#abcdef1234567890",
		"https://127.0.0.1/file/abcDEF123#abcdef1234567890",
		"https://evil.test/anything",
	}
	for _, raw := range cases {
		if _, err := SharedLinkPost(raw); err == nil {
			t.Errorf("expected rejected share URL: %q", raw)
		}
	}
}
