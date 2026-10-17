package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

type booruTagDatabase struct {
	index    *booru.TagIndex
	revision string
	mod      time.Time
	size     int64
}

var booruTagDatabases = struct {
	sync.Mutex
	entries map[string]booruTagDatabase
}{entries: map[string]booruTagDatabase{}}

func (rs booruBrowserRoutes) tagSource(r *http.Request) (booru.Source, bool) {
	sources, err := rs.sources()
	if err != nil {
		return booru.Source{}, false
	}
	for _, s := range sources {
		if s.ID == r.URL.Query().Get("source") && s.Instance() == r.URL.Query().Get("instance") {
			return s, true
		}
	}
	return booru.Source{}, false
}
func (rs booruBrowserRoutes) Tags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources before using its tag database.")
		return
	}
	path := filepath.Join(rs.configPath, "booru-tags", source.Instance()+".csv")
	// Serialize loads/imports to prevent cache/file revisions from diverging. An
	// immutable index can be searched concurrently after releasing this lock.
	booruTagDatabases.Lock()
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		delete(booruTagDatabases.entries, path)
		booruTagDatabases.Unlock()
		writeVisualSimilarityJSON(w, map[string]any{"count": 0, "revision": "", "suggestions": []booru.Suggestion{}})
		return
	}
	if err != nil || stat.Size() > booru.MaxTagCSVBytes {
		booruTagDatabases.Unlock()
		writeBooruError(w, 500, "tags", "Cannot read tag database.")
		return
	}
	db, cached := booruTagDatabases.entries[path]
	if !cached || db.size != stat.Size() || !db.mod.Equal(stat.ModTime()) {
		file, openErr := os.Open(path)
		if openErr != nil {
			booruTagDatabases.Unlock()
			writeBooruError(w, 500, "tags", "Cannot open tag database.")
			return
		}
		h := sha256.New()
		index, readErr := booru.ReadTagCSV(io.TeeReader(file, h))
		file.Close()
		if readErr != nil {
			booruTagDatabases.Unlock()
			writeBooruError(w, 500, "tags", "Invalid tag database. Upload a valid CSV to replace it.")
			return
		}
		db = booruTagDatabase{index, hex.EncodeToString(h.Sum(nil)), stat.ModTime(), stat.Size()}
		// The cache holds at most four databases; disk persistence is independent.
		if len(booruTagDatabases.entries) >= 4 {
			for key := range booruTagDatabases.entries {
				delete(booruTagDatabases.entries, key)
				break
			}
		}
		booruTagDatabases.entries[path] = db
	}
	booruTagDatabases.Unlock()
	writeVisualSimilarityJSON(w, map[string]any{"count": db.index.Count(), "revision": db.revision, "suggestions": db.index.Suggest(r.Context(), r.URL.Query().Get("q"))})
}
func booruSameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
	}
	return true
}
func (rs booruBrowserRoutes) ImportTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !booruSameOrigin(r) {
		writeBooruError(w, 403, "origin", "Upload from this server's interface.")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/csv") {
		writeBooruError(w, 415, "tags", "Upload a text/csv database.")
		return
	}
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources first.")
		return
	}
	dir := filepath.Join(rs.configPath, "booru-tags")
	if err := os.MkdirAll(dir, 0700); err != nil {
		writeBooruError(w, 500, "tags", "Cannot create database directory.")
		return
	}
	path := filepath.Join(dir, source.Instance()+".csv")
	f, err := os.CreateTemp(dir, ".tags-*.tmp")
	if err != nil {
		writeBooruError(w, 500, "tags", "Cannot stage database.")
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()
	r.Body = http.MaxBytesReader(w, r.Body, booru.MaxTagCSVBytes)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), r.Body)
	if err != nil || n > booru.MaxTagCSVBytes {
		writeBooruError(w, 413, "tags", "Database exceeds 64 MiB or upload was interrupted.")
		return
	}
	if _, err = f.Seek(0, 0); err != nil {
		writeBooruError(w, 500, "tags", "Cannot verify database.")
		return
	}
	index, err := booru.ReadTagCSV(f)
	if err != nil {
		writeBooruError(w, 400, "tags", fmt.Sprintf("Database rejected: %s", err))
		return
	}
	booruTagDatabases.Lock()
	defer booruTagDatabases.Unlock()
	// Optimistic revision includes the entire file, including edits made outside
	// this process. A stale editor cannot overwrite a newer upload.
	revision := ""
	if file, err := os.Open(path); err == nil {
		h := sha256.New()
		_, err = io.Copy(h, io.LimitReader(file, booru.MaxTagCSVBytes+1))
		file.Close()
		if err != nil {
			writeBooruError(w, 500, "tags", "Cannot read database revision.")
			return
		}
		revision = hex.EncodeToString(h.Sum(nil))
	} else if !os.IsNotExist(err) {
		writeBooruError(w, 500, "tags", "Cannot read database revision.")
		return
	}
	if r.Header.Get("X-Booru-Revision") != revision {
		writeBooruError(w, 409, "tags", "Tag database changed. Reload it before uploading again.")
		return
	}
	if _, ok := rs.tagSource(r); !ok {
		writeBooruError(w, 409, "source", "Source changed during upload. Reload sources.")
		return
	}
	if err = f.Sync(); err == nil {
		err = f.Close()
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		writeBooruError(w, 500, "tags", "Cannot save database.")
		return
	}
	delete(booruTagDatabases.entries, path)
	writeVisualSimilarityJSON(w, map[string]any{"count": index.Count(), "revision": hex.EncodeToString(h.Sum(nil)), "suggestions": []booru.Suggestion{}})
}
