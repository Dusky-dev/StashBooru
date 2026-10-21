package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
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

// The maximum API page has 50 posts; archive posts can have up to 200
// attachments each. Keep enough media keys to access an entire page, not
// only the first few hundred media files. Oldest keys expire first.
const booruSnapshotCapacity = 16384
const booruSnapshotTTL = 30 * time.Minute

var booruSnapshots = struct {
	sync.Mutex
	entries map[string]booruMediaSnapshot
	order   []string
	start   int
}{entries: map[string]booruMediaSnapshot{}}

func rememberBooruPost(source booru.Source, post booru.Post) string {
	// Keep a bounded taxonomy projection for filenames and saved favorites.
	core := []booru.Tag{}
	for _, tag := range post.Tags {
		if len(core) >= 64 {
			break
		}
		if (tag.Category == "artist" || tag.Category == "character" || tag.Category == "copyright") && len(tag.Name) <= 200 && len(tag.Label) <= 200 {
			core = append(core, tag)
		}
	}
	post.Tags = core
	refs := []booru.Post{}
	for _, asset := range post.Attachments {
		refs = append(refs, booru.Post{ID: asset.ID, MediaKey: asset.MediaKey})
	}
	post.Attachments = refs
	post.MediaKey = ""
	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	token := hex.EncodeToString(key)
	booruSnapshots.Lock()
	defer booruSnapshots.Unlock()
	now := time.Now()
	// FIFO eviction avoids scanning thousands of media keys for every
	// attachment on a large Pawchive or Kemono page.
	for booruSnapshots.start < len(booruSnapshots.order) {
		first := booruSnapshots.order[booruSnapshots.start]
		item, ok := booruSnapshots.entries[first]
		if ok && now.Sub(item.Created) <= booruSnapshotTTL && len(booruSnapshots.entries) < booruSnapshotCapacity {
			break
		}
		delete(booruSnapshots.entries, first)
		booruSnapshots.start++
	}
	if booruSnapshots.start > 4096 && booruSnapshots.start*2 >= len(booruSnapshots.order) {
		booruSnapshots.order = append([]string(nil), booruSnapshots.order[booruSnapshots.start:]...)
		booruSnapshots.start = 0
	}
	booruSnapshots.entries[token] = booruMediaSnapshot{source.ID, source.Instance(), post, now}
	booruSnapshots.order = append(booruSnapshots.order, token)
	return token
}

func (rs booruBrowserRoutes) mediaSnapshot(key string) (booru.Source, booru.Post, bool) {
	booruSnapshots.Lock()
	item, ok := booruSnapshots.entries[key]
	booruSnapshots.Unlock()
	if !ok || time.Since(item.Created) > booruSnapshotTTL {
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

// Public responses contain opaque keys and public image fallbacks, never originals.
func publicBooruPost(source booru.Source, post booru.Post) booru.Post {
	post.PreviewURL = source.ApprovedMediaURL(post.PreviewURL)
	post.SampleURL = source.ApprovedMediaURL(post.SampleURL)
	post.OriginalURL = source.ApprovedMediaURL(post.OriginalURL)
	post.DownloadAvailable = post.OriginalURL != ""
	post.DirectPreviewURL, post.DirectSampleURL = "", ""

	for i, asset := range post.Attachments {
		if len(asset.Tags) == 0 {
			asset.Tags = post.Tags
		}
		post.Attachments[i] = publicBooruPost(source, asset)
	}
	post.MediaKey = rememberBooruPost(source, post)
	if u, err := url.Parse(post.PreviewURL); err == nil && u.Scheme == "https" && u.RawQuery == "" {
		post.DirectPreviewURL = u.String()
	}
	// Samples promoted from originals must remain behind the bounded
	// server proxy. Do not retry original media directly from the browser
	// after an oversized or rejected proxy request.
	if post.SampleURL != "" && post.SampleURL != post.OriginalURL {
		if u, err := url.Parse(post.SampleURL); err == nil && u.Scheme == "https" && u.RawQuery == "" {
			post.DirectSampleURL = u.String()
		}
	}
	if post.PreviewURL != "" {
		post.PreviewURL = "booru-browser/media?key=" + post.MediaKey + "&kind=preview"
	}
	if post.SampleURL != "" {
		post.SampleURL = "booru-browser/media?key=" + post.MediaKey + "&kind=sample"
	}
	return post
}
