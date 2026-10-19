package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

type booruSavedSearch struct {
	Name   string `json:"name"`
	Search string `json:"search"`
}
type booruFavorite struct {
	Instance string          `json:"instance"`
	Post     booru.Post      `json:"post"`
	Original string          `json:"original"`
	Children []booruFavorite `json:"children,omitempty"`
	SavedAt  time.Time       `json:"savedAt"`
}
type booruBookmarks struct {
	Favorites []booruFavorite    `json:"favorites"`
	Searches  []booruSavedSearch `json:"searches"`
}

var booruBookmarksMu sync.Mutex
var booruSavedSourceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func (rs booruBrowserRoutes) readBookmarks() (booruBookmarks, error) {
	value := booruBookmarks{Favorites: []booruFavorite{}, Searches: []booruSavedSearch{}}
	file, err := os.Open(filepath.Join(rs.configPath, "booru-bookmarks.json"))
	if os.IsNotExist(err) {
		return value, nil
	}
	if err != nil {
		return value, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return value, fmt.Errorf("bookmark storage exceeds its bounds")
	}
	err = json.Unmarshal(data, &value)
	// Repair previously accepted blank filters when loading them as well.
	for i := range value.Searches {
		if search, ok := normalizeSavedBooruSearch(value.Searches[i].Search); ok {
			value.Searches[i].Search = search
		}
	}
	return value, err
}
func publicBookmarks(value booruBookmarks) any {
	refs := []map[string]string{}
	for _, f := range value.Favorites {
		refs = append(refs, map[string]string{"instance": f.Instance, "id": f.Post.ID})
	}
	return map[string]any{"favorites": refs, "searches": value.Searches}
}
func (rs booruBrowserRoutes) Bookmarks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	booruBookmarksMu.Lock()
	defer booruBookmarksMu.Unlock()
	value, err := rs.readBookmarks()
	if err != nil {
		writeBooruError(w, 500, "bookmarks", "Cannot read saved favorites/searches.")
		return
	}
	writeVisualSimilarityJSON(w, publicBookmarks(value))
}
func normalizeSavedBooruSearch(search string) (string, bool) {
	if len(search) > 8192 || !strings.HasPrefix(search, "?") || strings.ContainsAny(search, "\r\n\x00#") {
		return "", false
	}
	params, err := url.ParseQuery(strings.TrimPrefix(search, "?"))
	if err != nil || len(params["source"]) > 8 {
		return "", false
	}
	query := params.Get("q")
	if len(query) > 1024 || strings.ContainsAny(query, "\x00\r\n") {
		return "", false
	}
	if strings.TrimSpace(query) == "" {
		query = "*"
	}
	if booru.ValidateQuery(query, 1, 24) != nil {
		return "", false
	}
	for _, id := range params["source"] {
		if !booruSavedSourceID.MatchString(id) {
			return "", false
		}
	}
	for key, values := range params {
		switch key {
		case "source", "q", "inspect":
		case "sort":
			if !((booru.SearchOptions{Sort: params.Get(key)}).Validate()) {
				return "", false
			}
		case "rating":
			if !((booru.SearchOptions{Rating: params.Get(key)}).Validate()) {
				return "", false
			}
		case "media":
			if params.Get(key) != "images" && params.Get(key) != "videos" {
				return "", false
			}
		case "favorites", "ignored":
			if params.Get(key) != "1" {
				return "", false
			}
		case "ai":
			if params.Get(key) != "hide" {
				return "", false
			}
		case "limit":
			n, e := strconv.Atoi(params.Get(key))
			if e != nil || n < 1 || n > 50 {
				return "", false
			}
		default:
			if !strings.HasPrefix(key, "p.") {
				return "", false
			}
			n, e := strconv.Atoi(params.Get(key))
			if e != nil || n < 1 || n > 750 {
				return "", false
			}
		}
		if key != "source" && len(values) != 1 {
			return "", false
		}
	}
	if strings.TrimSpace(params.Get("q")) == "" && params.Get("favorites") != "1" {
		params.Set("q", "")
	}
	normalized := "?" + params.Encode()
	if len(normalized) > 8192 {
		return "", false
	}
	return normalized, true
}
func (rs booruBrowserRoutes) saveFavorite(key string) (booruFavorite, error) {
	source, post, ok := rs.mediaSnapshot(key)
	if !ok {
		return booruFavorite{}, fmt.Errorf("post expired; refresh before saving it")
	}
	value := booruFavorite{Instance: source.Instance(), Post: post, Original: post.OriginalURL, SavedAt: time.Now().UTC()}
	value.Post.MediaKey = ""
	value.Post.Attachments = nil
	for _, ref := range post.Attachments {
		_, asset, ok := rs.mediaSnapshot(ref.MediaKey)
		if !ok {
			return value, fmt.Errorf("gallery expired; refresh before saving it")
		}
		asset.Attachments = nil
		asset.MediaKey = ""
		value.Children = append(value.Children, booruFavorite{Instance: value.Instance, Post: asset, Original: asset.OriginalURL})
	}
	return value, nil
}
func (rs booruBrowserRoutes) UpdateBookmarks(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action   string `json:"action"`
		Key      string `json:"key"`
		Favorite bool   `json:"favorite"`
		Name     string `json:"name"`
		Search   string `json:"search"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	booruBookmarksMu.Lock()
	defer booruBookmarksMu.Unlock()
	value, err := rs.readBookmarks()
	if err != nil {
		writeBooruError(w, 500, "bookmarks", "Cannot read saved favorites/searches.")
		return
	}
	switch input.Action {
	case "favorite":
		var favorite booruFavorite
		if input.Favorite {
			favorite, err = rs.saveFavorite(input.Key)
		} else {
			source, post, ok := rs.mediaSnapshot(input.Key)
			if !ok {
				err = fmt.Errorf("post expired; refresh favorites")
			} else {
				favorite = booruFavorite{Instance: source.Instance(), Post: post}
			}
		}
		if err != nil {
			writeBooruError(w, 410, "expired", err.Error())
			return
		}
		next := []booruFavorite{}
		for _, f := range value.Favorites {
			if f.Instance != favorite.Instance || f.Post.ID != favorite.Post.ID {
				next = append(next, f)
			}
		}
		if input.Favorite {
			next = append([]booruFavorite{favorite}, next...)
		}
		if len(next) > 500 {
			writeBooruError(w, 400, "bookmarks", "At most 500 favorite posts can be saved.")
			return
		}
		value.Favorites = next
	case "search", "remove-search":
		input.Name = strings.TrimSpace(input.Name)
		search, validSearch := normalizeSavedBooruSearch(input.Search)
		if input.Name == "" || len(input.Name) > 120 || strings.ContainsAny(input.Name, "\r\n\x00") || (input.Action == "search" && !validSearch) {
			writeBooruError(w, 400, "bookmarks", "Enter a name up to 120 bytes and a valid booru search.")
			return
		}
		next := []booruSavedSearch{}
		for _, s := range value.Searches {
			if !strings.EqualFold(s.Name, input.Name) {
				next = append(next, s)
			}
		}
		if input.Action == "search" {
			next = append(next, booruSavedSearch{Name: input.Name, Search: search})
		}
		if len(next) > 100 {
			writeBooruError(w, 400, "bookmarks", "At most 100 search filters can be saved.")
			return
		}
		value.Searches = next
	default:
		writeBooruError(w, 400, "bookmarks", "Invalid bookmark action.")
		return
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > 8<<20 {
		writeBooruError(w, 400, "bookmarks", "Saved favorites exceed the 8 MiB metadata limit.")
		return
	}
	if err = writeBooruAtomic(rs.configPath, "booru-bookmarks.json", data); err != nil {
		writeBooruError(w, 500, "bookmarks", "Cannot save favorites/searches.")
		return
	}
	writeVisualSimilarityJSON(w, publicBookmarks(value))
}
func restoreFavorite(source booru.Source, value booruFavorite) booru.Post {
	post := value.Post
	post.SourceID = source.ID
	post.OriginalURL = value.Original
	post.Favorite = true
	post.Attachments = nil
	for _, child := range value.Children {
		post.Attachments = append(post.Attachments, restoreFavorite(source, child))
	}
	return post
}
func (rs booruBrowserRoutes) Favorites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources.")
		return
	}
	if r.URL.Query().Get("rating") != "" || r.URL.Query().Get("sort") != "" {
		writeBooruError(w, 400, "favorites", "Favorites use saved order and All ratings; clear rating/sort controls.")
		return
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if r.URL.Query().Get("page") == "" {
		page = 1
	} else if err != nil {
		page = 0
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if r.URL.Query().Get("limit") == "" {
		limit = 24
	} else if err != nil {
		limit = 0
	}
	if booru.ValidateQuery("*", page, limit) != nil {
		writeBooruError(w, 400, "favorites", "Invalid favorites page.")
		return
	}
	booruBookmarksMu.Lock()
	value, err := rs.readBookmarks()
	booruBookmarksMu.Unlock()
	if err != nil {
		writeBooruError(w, 500, "bookmarks", "Cannot read favorites.")
		return
	}
	posts := []booru.Post{}
	for _, favorite := range value.Favorites {
		if favorite.Instance == source.Instance() {
			posts = append(posts, restoreFavorite(source, favorite))
		}
	}
	total := int64(len(posts))
	start := min((page-1)*limit, len(posts))
	end := min(start+limit, len(posts))
	assets := end - start
	for _, post := range posts[start:end] {
		assets += len(post.Attachments)
	}
	if assets > 256 {
		writeBooruError(w, 400, "favorites", "Favorite page exceeds 256 media files. Reduce posts per page.")
		return
	}
	items := []booru.Post{}
	for _, post := range posts[start:end] {
		items = append(items, publicBooruPost(source, post))
	}
	writeVisualSimilarityJSON(w, booru.Result{SourceID: source.ID, Page: page, Limit: limit, Posts: items, Total: &total, HasMore: end < len(posts), FetchedAt: time.Now().UTC()})
}
