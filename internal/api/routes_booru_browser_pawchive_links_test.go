package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestPawchiveSharedLinksUseSeparatePrivateSnapshots(t *testing.T) {
	original := "https://www.dropbox.com/scl/fi/abcDEF123/cat.png?rlkey=abcdef123456&dl=0"
	mega := "https://mega.nz/file/abcDEF123#abcdef1234567890"
	links, _ := booru.LinkedMediaFromPawchiveContent(original+" "+mega, "https://pawchive.pw/patreon/user/42/post/31")
	if len(links) != 2 {
		t.Fatalf("links missing: %d", len(links))
	}
	root := booru.Post{
		ID: "a12345678901234567890123456789012", SourceID: "pawchive",
		PostURL: "https://pawchive.pw/patreon/user/42/post/31", Tags: []booru.Tag{},
		Attachments: links,
	}
	var source booru.Source
	for _, item := range booru.Defaults() {
		if item.ID == "pawchive" {
			source = item
		}
	}
	pub := publicBooruPost(source, root)
	data, err := json.Marshal(pub)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"abcdef123456", "abcdef1234567890", "mega.nz/file/", "rlkey"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("shared-link secret leaked into response: %q", secret)
		}
	}
	if len(pub.Attachments) != 2 {
		t.Fatalf("shared links disappeared: %+v", pub)
	}
	for _, asset := range pub.Attachments {
		if asset.MediaKey == "" || asset.SourceID != "shared-links" {
			t.Fatalf("linked asset not given server media key: %+v", asset)
		}
		resolvedSource, resolved, ok := (booruBrowserRoutes{}).mediaSnapshot(asset.MediaKey)
		if !ok || resolvedSource.ID != "shared-links" || resolved.OriginalURL == "" {
			t.Fatalf("linked source cannot be resolved by server: %+v %+v", resolvedSource, resolved)
		}
	}
	if pub.Attachments[0].DirectPreviewURL != "" || pub.Attachments[0].DirectSampleURL != "" {
		t.Fatal("direct share links must not be sent to the browser")
	}
}
