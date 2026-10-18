package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

type booruMediaSnapshot struct {
	Source, Instance string
	Post             booru.Post
	Created          time.Time
}

var booruSnapshots = struct {
	sync.Mutex
	entries map[string]booruMediaSnapshot
}{entries: map[string]booruMediaSnapshot{}}

func rememberBooruPost(source booru.Source, post booru.Post) string {
	post.Tags = nil // Asset snapshots do not retain the potentially large tag catalogue.
	post.MediaKey = ""
	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	token := hex.EncodeToString(key)
	booruSnapshots.Lock()
	defer booruSnapshots.Unlock()
	now := time.Now()
	oldest, keyOld := now, ""
	for key, item := range booruSnapshots.entries {
		if now.Sub(item.Created) > 30*time.Minute {
			delete(booruSnapshots.entries, key)
			continue
		}
		if item.Created.Before(oldest) {
			oldest, keyOld = item.Created, key
		}
	}
	if len(booruSnapshots.entries) >= 4096 {
		delete(booruSnapshots.entries, keyOld)
	}
	booruSnapshots.entries[token] = booruMediaSnapshot{source.ID, source.Instance(), post, now}
	return token
}

func (rs booruBrowserRoutes) mediaSnapshot(key string) (booru.Source, booru.Post, bool) {
	booruSnapshots.Lock()
	item, ok := booruSnapshots.entries[key]
	booruSnapshots.Unlock()
	if !ok || time.Since(item.Created) > 30*time.Minute {
		return booru.Source{}, booru.Post{}, false
	}
	sources, err := rs.sources()
	if err != nil {
		return booru.Source{}, booru.Post{}, false
	}
	for _, source := range sources {
		if source.ID == item.Source && source.Instance() == item.Instance {
			return source, item.Post, true
		}
	}
	return booru.Source{}, booru.Post{}, false
}

func (rs booruBrowserRoutes) Media(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	source, post, ok := rs.mediaSnapshot(r.URL.Query().Get("key"))
	if !ok {
		writeBooruError(w, 410, "expired", "Preview expired or its source changed. Refresh the search.")
		return
	}
	asset := post.PreviewURL
	limit := int64(8 << 20)
	imagesOnly := true
	switch r.URL.Query().Get("kind") {
	case "preview":
	case "sample":
		asset, limit = post.SampleURL, 24<<20
	case "motion":
		switch strings.ToLower(post.Format) {
		case "mp4", "webm", "gif", "apng":
		default:
			writeBooruError(w, 400, "media", "No supported animated preview is available.")
			return
		}
		asset, limit, imagesOnly = post.OriginalURL, 24<<20, false
		if asset == "" || post.Bytes > limit {
			writeBooruError(w, 413, "media", "Animated preview unavailable or over 24 MiB. Use Download or open the remote post.")
			return
		}
	default:
		writeBooruError(w, 400, "media", "Invalid preview kind.")
		return
	}
	var buf bytes.Buffer
	info, err := rs.client.FetchMedia(r.Context(), source, booru.MediaRequest{URL: asset, Limit: limit, ImagesOnly: imagesOnly}, &buf)
	if err != nil {
		writeBooruError(w, 502, "media", err.Error())
		return
	}
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "preview", time.Time{}, bytes.NewReader(buf.Bytes()))
}
