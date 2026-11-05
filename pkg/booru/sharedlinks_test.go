package booru

import (
	"fmt"
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
		{"https://mega.nz/folder/abcDEF123#abcdef1234567890", "folder"},
		{"https://mega.nz/#F!abcDEF123!abcdef1234567890", "folder"},
	}
	for _, tc := range cases {
		post, err := SharedLinkPost(tc.url)
		if err != nil {
			t.Fatalf("shared link %q: %v", tc.url, err)
		}
		if post.Format != tc.format || post.DownloadAvailable != (tc.format != "folder") ||
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
		"https://mega.nz/folder/abcDEF123",
		"https://mega.nz/folder/abcDEF123#not-a*key",
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

func TestLinkedMediaFromPawchiveContent(t *testing.T) {
	content := `<p>Files <a href="https://www.dropbox.com/scl/fi/abcDEF123/cat.png?rlkey=abcdef123456&amp;dl=0">Dropbox</a>
	<a href="https://mega.nz/file/abcDEF123#abcdef1234567890">MEGA</a></p>
	Backup https://www.dropbox.com/scl/fi/abcDEF123/cat.png?rlkey=abcdef123456&dl=1
	Folder https://mega.nz/folder/abcDEF123#abcdef1234567890
	Legacy https://mega.nz/#F!abcDEF123!abcdef1234567890
	Unsafe https://dropbox.com.bad.test/s/abcDEF123/evil.jpg
	Invalid https://mega.nz/file/abcDEF123#not*valid
	`
	links, folders := LinkedMediaFromPawchiveContent(content, "https://pawchive.pw/patreon/user/42/post/31")
	if len(links) != 4 || folders != 2 {
		t.Fatalf("expected deduplicated Dropbox + MEGA files and two folder links, got %d and %d: %+v", len(links), folders, links)
	}
	for _, item := range links {
		if item.PostURL != "https://pawchive.pw/patreon/user/42/post/31" ||
			item.SourceID != "shared-links" || item.OriginalURL == "" ||
			!ValidPostID(item.ID) {
			t.Fatalf("bad linked-file provenance or identity: %+v", item)
		}
	}
	if links[0].Format != "png" || links[1].Format != "" ||
		links[2].Format != "folder" || links[3].Format != "folder" ||
		links[2].DownloadAvailable || links[3].DownloadAvailable {
		t.Fatalf("wrong linked media formats: %+v", links)
	}
}

func TestLinkedMediaFromPawchiveContentBounded(t *testing.T) {
	content := ""
	for i := 0; i < 30; i++ {
		content += fmt.Sprintf("https://www.dropbox.com/s/abcDEF%06d/file.png ", i)
	}
	links, folders := LinkedMediaFromPawchiveContent(content, "https://pawchive.pw/post")
	if len(links) != maxLinkedMedia || folders != 0 {
		t.Fatalf("unbounded linked-media results: %d, %d", len(links), folders)
	}
}

func TestSharedLinkProviderRedirectAllowsOnlyValidatedServerAssets(t *testing.T) {
	if !ValidMegaSharedFolderURL("https://mega.nz/folder/abcDEF123#abcdef1234567890") {
		t.Fatal("valid folder share rejected")
	}
	for _, link := range []string{
		"https://mega.nz/folder/abcDEF123",
		"https://mega.nz/folder/abcDEF123#not*valid",
		"https://mega.nz.evil.example/folder/abcDEF123#abcdef1234567890",
		"https://mega.nz@127.0.0.1/folder/abcDEF123#abcdef1234567890",
	} {
		if ValidMegaSharedFolderURL(link) {
			t.Fatalf("unsafe folder accepted: %q", link)
		}
	}
}
