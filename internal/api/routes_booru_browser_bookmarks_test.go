package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

func TestBooruSavedBlankSearchesRemainExecutable(t *testing.T) {
	for _, search := range []string{"?source=danbooru", "?source=danbooru&q=", "?source=danbooru&q=+++", "?source=pixiv", "?source=pixiv&q="} {
		t.Run(search, func(t *testing.T) {
			rs := booruBrowserRoutes{configPath: t.TempDir()}
			body := booruTestJSON(t, map[string]string{"action": "search", "name": "Recent", "search": search})
			w := httptest.NewRecorder()
			rs.UpdateBookmarks(w, toolRequest("POST", "/", string(body), "application/json"))
			var bookmarks booruBookmarks
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &bookmarks) != nil || len(bookmarks.Searches) != 1 {
				t.Fatal(w.Code, w.Body.String())
			}
			params, err := url.ParseQuery(strings.TrimPrefix(bookmarks.Searches[0].Search, "?"))
			if err != nil || !params.Has("q") || params.Get("q") != "" {
				t.Fatal("saved filter cannot execute", bookmarks, err)
			}
			data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-bookmarks.json"))
			if err != nil || !strings.Contains(string(data), "q=") {
				t.Fatal("explicit blank query not persisted", string(data), err)
			}
			// Simulate a bookmark accepted by the previous version and a restart.
			legacy := booruTestJSON(t, booruBookmarks{Searches: []booruSavedSearch{{Name: "Recent", Search: search}}})
			if err := os.WriteFile(filepath.Join(rs.configPath, "booru-bookmarks.json"), legacy, 0600); err != nil {
				t.Fatal(err)
			}
			restarted := booruBrowserRoutes{configPath: rs.configPath}
			w = httptest.NewRecorder()
			restarted.Bookmarks(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &bookmarks) != nil || bookmarks.Searches[0].Search != "?q=&source="+params.Get("source") {
				t.Fatal("legacy filter lacks an explicit blank query", w.Code, w.Body.String())
			}
		})
	}
	for _, search := range []string{"?source=danbooru&q=&q=cat", "?source=danbooru&q=cat&q=", "?q=%0A", "?q=%00", "?q=" + strings.Repeat("+", 1025)} {
		if _, ok := normalizeSavedBooruSearch(search); ok {
			t.Fatal("invalid query normalized before validation", search)
		}
	}
	search, ok := normalizeSavedBooruSearch("?source=danbooru&favorites=1")
	if !ok || strings.Contains(search, "q=") {
		t.Fatal("favorites changed into a provider query", search)
	}
}

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
		if _, ok := normalizeSavedBooruSearch(query); ok {
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
