package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
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
	db, err := rs.tagDatabase(source)
	if err != nil {
		writeBooruError(w, 500, "tags", err.Error())
		return
	}
	items := []booru.Suggestion{}
	count := 0
	if db.index != nil {
		count = db.index.Count()
		items = db.index.Suggest(r.Context(), r.URL.Query().Get("q"))
	}
	writeVisualSimilarityJSON(w, map[string]any{"path": rs.tagDatabasePath(source), "count": count, "revision": db.revision, "suggestions": items})
}

func (rs booruBrowserRoutes) tagDatabase(source booru.Source) (booruTagDatabase, error) {
	path := filepath.Join(rs.configPath, "booru-tags", source.Instance()+".csv")
	booruTagDatabases.Lock()
	defer booruTagDatabases.Unlock()
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		delete(booruTagDatabases.entries, path)
		return booruTagDatabase{}, nil
	}
	if err != nil || stat.Size() > booru.MaxTagCSVBytes {
		return booruTagDatabase{}, fmt.Errorf("cannot read tag database")
	}
	db, cached := booruTagDatabases.entries[path]
	if cached && db.size == stat.Size() && db.mod.Equal(stat.ModTime()) {
		return db, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return booruTagDatabase{}, fmt.Errorf("cannot open tag database")
	}
	defer file.Close()
	h := sha256.New()
	index, err := booru.ReadTagCSVForEngine(io.TeeReader(file, h), source.Engine)
	if err != nil {
		return booruTagDatabase{}, fmt.Errorf("invalid tag database; upload a valid CSV to replace it")
	}
	db = booruTagDatabase{index, hex.EncodeToString(h.Sum(nil)), stat.ModTime(), stat.Size()}
	if len(booruTagDatabases.entries) >= 4 {
		for key := range booruTagDatabases.entries {
			delete(booruTagDatabases.entries, key)
			break
		}
	}
	booruTagDatabases.entries[path] = db
	return db, nil
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
	rs.importTagCSV(w, r, "")
}

func (rs booruBrowserRoutes) importTagCSV(w http.ResponseWriter, r *http.Request, databasePath string) {
	w.Header().Set("Cache-Control", "no-store")
	if !booruSameOrigin(r) {
		writeBooruError(w, 403, "origin", "Upload from this server's interface.")
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "text/csv" {
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
	index, err := booru.ReadTagCSVForEngine(f, source.Engine)
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
	metadata, encodeErr := json.Marshal(map[string]string{"path": databasePath})
	if encodeErr != nil {
		writeBooruError(w, 500, "tags", "Cannot save database path.")
		return
	}
	if err := writeBooruAtomic(dir, source.Instance()+".path.json", metadata); err != nil {
		delete(booruTagDatabases.entries, path)
		writeBooruError(w, 500, "tags", "CSV installed, but its path could not be saved. Reload settings.")
		return
	}
	delete(booruTagDatabases.entries, path)
	writeVisualSimilarityJSON(w, map[string]any{"path": databasePath, "count": index.Count(), "revision": hex.EncodeToString(h.Sum(nil)), "suggestions": []booru.Suggestion{}})
}

func (rs booruBrowserRoutes) tagDatabasePath(source booru.Source) string {
	data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-tags", source.Instance()+".path.json"))
	if err != nil || len(data) > 8192 {
		return ""
	}
	var value struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	return value.Path
}
func (rs booruBrowserRoutes) ImportTagPath(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path     string `json:"path"`
		Revision string `json:"revision"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if !filepath.IsAbs(input.Path) || len(input.Path) > 4096 || !strings.EqualFold(filepath.Ext(input.Path), ".csv") {
		writeBooruError(w, 400, "tags", "Enter an absolute server path to a CSV database.")
		return
	}
	file, err := os.Open(input.Path)
	if err != nil {
		writeBooruError(w, 400, "tags", "Cannot open that CSV path on the Stash server.")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > booru.MaxTagCSVBytes {
		writeBooruError(w, 400, "tags", "Database must be a regular CSV file of at most 64 MiB.")
		return
	}
	requestClone := r.Clone(r.Context())
	requestClone.Header = r.Header.Clone()
	requestClone.Header.Set("Content-Type", "text/csv")
	requestClone.Header.Set("X-Booru-Revision", input.Revision)
	requestClone.Body = file
	rs.importTagCSV(w, requestClone, input.Path)
}
