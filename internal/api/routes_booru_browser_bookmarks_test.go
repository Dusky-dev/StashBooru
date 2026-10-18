package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

func TestBooruBookmarksPersistGalleryAndRejectStaleSource(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	source := booru.Defaults()[0]
	post := publicBooruPost(source, booru.Post{ID: "123", OriginalURL: source.URL + "/one.jpg", Tags: []booru.Tag{{Name: "artist", Category: "artist"}}, Attachments: []booru.Post{{ID: "p" + strings.Repeat("a", 32), OriginalURL: source.URL + "/two.jpg"}}})
	write := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		rs.UpdateBookmarks(w, toolRequest("POST", "/", body, "application/json"))
		return w
	}
	if w := write(`{"action":"favorite","key":"` + post.MediaKey + `","favorite":true}`); w.Code != 200 || strings.Contains(w.Body.String(), "one.jpg") {
		t.Fatal(w.Code, w.Body.String())
	}
	rs = booruBrowserRoutes{configPath: rs.configPath}
	endpoint := "/?source=" + source.ID + "&instance=" + source.Instance() + "&page=1&limit=24"
	w := httptest.NewRecorder()
	rs.Favorites(w, httptest.NewRequest("GET", endpoint, nil))
	var result booru.Result
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Posts) != 1 || len(result.Posts[0].Attachments) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "one.jpg") || strings.Contains(w.Body.String(), "two.jpg") {
		t.Fatal("original URLs leaked")
	}
	_, child, ok := rs.mediaSnapshot(result.Posts[0].Attachments[0].MediaKey)
	if !ok || child.OriginalURL != source.URL+"/two.jpg" || len(child.Tags) != 1 {
		t.Fatal("gallery snapshot lost")
	}
	if w = write(`{"action":"search","name":"Artist","search":"?source=danbooru&q=artist"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	before, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-bookmarks.json"))
	if w = write(`{"action":"search","name":"Broken","search":"https://evil.example.org"}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	after, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-bookmarks.json"))
	if string(before) != string(after) {
		t.Fatal("invalid action damaged bookmarks")
	}
	if w = write(`{"action":"favorite","key":"expired","favorite":true}`); w.Code != 410 {
		t.Fatal(w.Code)
	}
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(`[{"id":"danbooru","disabled":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	rs.Favorites(w, httptest.NewRequest("GET", endpoint, nil))
	if w.Code != 409 {
		t.Fatal("disabled source restored", w.Code)
	}
}
func TestBooruFilenamePublicationAndReceiptAssetBinding(t *testing.T) {
	root := t.TempDir()
	rs := booruBrowserRoutes{configPath: t.TempDir(), libraryRoots: func() []string { return []string{root} }}
	stage := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(stage, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := mediaconvert.MD5(stage)
	if err != nil {
		t.Fatal(err)
	}
	source := booru.Defaults()[0]
	post := booru.Post{ID: "123", OriginalURL: source.URL + "/one.png", Tags: []booru.Tag{{Name: "Artist", Category: "artist"}}}
	format := "%artist%/%id%.%ext%"
	first, err := rs.publishTemplatedBooruFile(context.Background(), stage, root, source, post, hash, "png", format)
	if err != nil || first != filepath.Join(root, "Artist", "123.png") {
		t.Fatal(first, err)
	}
	if err = os.WriteFile(first, []byte("other bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := rs.publishTemplatedBooruFile(context.Background(), stage, root, source, post, hash, "png", format)
	if err != nil || second != filepath.Join(root, "Artist", "123_1.png") {
		t.Fatal(second, err)
	}
	if err = rs.saveDownloadReceipt(booruDownloadReceipt{Source: source.Public(), Post: post, Directory: root, Template: format, OutputMD5: hash, Library: &booruLibraryResult{Path: second}}); err != nil {
		t.Fatal(err)
	}
	if rs.reusableDownloadReceipt(root, source, post, false, format) == nil {
		t.Fatal("nested receipt not reusable")
	}
	post.OriginalURL = source.URL + "/changed.png"
	if rs.reusableDownloadReceipt(root, source, post, false, format) != nil {
		t.Fatal("changed gallery file reused")
	}
	escape := filepath.Join(root, "escape")
	if os.Symlink(t.TempDir(), escape) == nil {
		if _, err = rs.publishTemplatedBooruFile(context.Background(), stage, root, source, post, hash, "png", "escape/%id%.%ext%"); err == nil {
			t.Fatal("symlink template accepted")
		}
	}
}

func TestBooruFavoriteMediaUsesCurrentAllowlist(t *testing.T) {
	source := booru.Defaults()[0]
	source.MediaHosts = nil
	post := publicBooruPost(source, booru.Post{ID: "123", OriginalURL: "https://old-cdn.example.org/full.jpg", PreviewURL: "https://old-cdn.example.org/preview.jpg", DirectPreviewURL: "https://old-cdn.example.org/preview.jpg"})
	if post.DownloadAvailable || post.PreviewURL != "" || post.DirectPreviewURL != "" {
		t.Fatal("obsolete allowlist survived restore", post)
	}
	for _, query := range []string{"?source=../bad&q=cat", "?source=danbooru&q=cat&limit=999", "?source=danbooru&q=cat&p.danbooru=751", "?source=danbooru&q=cat&rating=unsupported"} {
		if validSavedBooruSearch(query) {
			t.Fatal("invalid saved query", query)
		}
	}
}

func TestBooruFilenamePreferencesRejectInvalidReplacement(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	write := func(template string) *httptest.ResponseRecorder {
		body := booruTestJSON(t, map[string]any{"filenameTemplate": template})
		w := httptest.NewRecorder()
		rs.UpdateDownloadSettings(w, toolRequest("POST", "/", string(body), "application/json"))
		return w
	}
	if w := write("%artist%/%md5%.%ext%"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := write("../%id%.%ext%"); w.Code != 400 {
		t.Fatal("invalid template accepted", w.Code)
	}
	settings, err := rs.downloadSettings()
	if err != nil || settings.FilenameTemplate != "%artist%/%md5%.%ext%" {
		t.Fatal("invalid replacement damaged preferences", settings, err)
	}
}
