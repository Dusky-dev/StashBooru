package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/stashapp/stash/pkg/booru"
)

// Only ZIP/CBZ files are supported. Archives are never extracted onto disk.
// These limits apply before inflation and are independent of the provider's
// advertised content length (which may be missing or untrusted).
const (
	booruArchiveMaxBytes  = 64 << 20
	booruArchiveEntryMax  = 24 << 20
	booruArchiveMaxFiles  = 512
	booruArchiveMaxMedia  = 128
	booruArchiveCacheSize = 4
)

type booruArchiveFile struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Bytes  uint64 `json:"bytes"`
	Format string `json:"format"`
}

type booruArchiveListing struct {
	Files   []booruArchiveFile `json:"files"`
	Skipped int                `json:"skipped"`
}

type booruCachedArchive struct {
	Data    []byte
	Created time.Time
}

var booruArchiveCache = struct {
	sync.Mutex
	entries map[string]booruCachedArchive
}{entries: make(map[string]booruCachedArchive)}

// Limit decompression as well as remote media fetching. ZIP inflation can be
// expensive independently of the FetchMedia concurrency gate.
var booruArchiveInflations = make(chan struct{}, 4)

func validBooruArchiveEntry(f *zip.File) bool {
	if f.FileInfo().IsDir() || !f.Mode().IsRegular() || f.Flags&1 != 0 ||
		(f.Method != zip.Store && f.Method != zip.Deflate) ||
		f.UncompressedSize64 == 0 || f.UncompressedSize64 > booruArchiveEntryMax ||
		f.CompressedSize64 == 0 || f.UncompressedSize64 > f.CompressedSize64*100+65536 {
		return false
	}
	name := strings.ReplaceAll(f.Name, "\\", "/")
	if !utf8.ValidString(name) || len(name) > 256 || strings.ContainsRune(name, 0) ||
		strings.Contains(f.Name, "\\") ||
		strings.HasPrefix(name, "/") || strings.Contains(name, ":") ||
		path.Clean(name) != name || name == "." || name == ".." ||
		strings.HasPrefix(name, "../") {
		return false
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".apng", ".webp", ".avif", ".jxl", ".bmp", ".tiff", ".mp4", ".webm":
		return true
	default:
		return false
	}
}

func indexBooruArchive(data []byte) (booruArchiveListing, *zip.Reader, error) {
	listing := booruArchiveListing{Files: []booruArchiveFile{}}
	if len(data) == 0 || len(data) > booruArchiveMaxBytes {
		return listing, nil, fmt.Errorf("archive is empty or exceeds 64 MiB")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return listing, nil, fmt.Errorf("provider did not return a valid ZIP archive: %w", err)
	}
	if len(zr.File) > booruArchiveMaxFiles {
		return listing, nil, fmt.Errorf("archive contains too many entries")
	}
	for i, f := range zr.File {
		if !validBooruArchiveEntry(f) || len(listing.Files) >= booruArchiveMaxMedia {
			listing.Skipped++
			continue
		}
		listing.Files = append(listing.Files, booruArchiveFile{
			Index: i, Name: f.Name, Bytes: f.UncompressedSize64,
			Format: strings.TrimPrefix(strings.ToLower(path.Ext(f.Name)), "."),
		})
	}
	return listing, zr, nil
}

// Reject HTML, SVG, scripts and entries whose bytes disagree with their media
// extension. The browser never receives arbitrary untrusted archive content.
func booruArchiveContentType(format string, data []byte) string {
	detected := http.DetectContentType(data)
	switch format {
	case "jpg", "jpeg":
		if detected == "image/jpeg" {
			return detected
		}
	case "png", "apng":
		if detected == "image/png" {
			return detected
		}
	case "gif":
		if detected == "image/gif" {
			return detected
		}
	case "webp":
		if detected == "image/webp" {
			return detected
		}
	case "bmp":
		if detected == "image/bmp" {
			return detected
		}
	case "tiff":
		if detected == "image/tiff" {
			return detected
		}
	case "mp4":
		if len(data) >= 12 && string(data[4:8]) == "ftyp" &&
			string(data[8:12]) != "avif" && string(data[8:12]) != "avis" {
			return "video/mp4"
		}
	case "webm":
		if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
			return "video/webm"
		}
	case "avif":
		if len(data) >= 12 && string(data[4:8]) == "ftyp" &&
			(string(data[8:12]) == "avif" || string(data[8:12]) == "avis") {
			return "image/avif"
		}
	case "jxl":
		if len(data) >= 2 && data[0] == 0xff && data[1] == 0x0a {
			return "image/jxl"
		}
		if len(data) >= 12 && string(data[4:12]) == "JXL \r\n\x87\n" {
			return "image/jxl"
		}
	}
	return ""
}

func (rs booruBrowserRoutes) loadBooruArchive(r *http.Request, key string) ([]byte, error) {
	source, post, ok := rs.mediaSnapshot(key)
	if !ok {
		return nil, fmt.Errorf("archive preview expired or its source changed; refresh the search")
	}
	if post.OriginalURL == "" ||
		(strings.ToLower(post.Format) != "zip" && strings.ToLower(post.Format) != "cbz") {
		return nil, fmt.Errorf("this post does not have a supported ZIP/CBZ archive")
	}
	if post.Bytes > booruArchiveMaxBytes {
		return nil, fmt.Errorf("archive exceeds the 64 MiB preview limit; open the remote post")
	}
	booruArchiveCache.Lock()
	now := time.Now()
	for token, entry := range booruArchiveCache.entries {
		if now.Sub(entry.Created) > 5*time.Minute {
			delete(booruArchiveCache.entries, token)
		}
	}
	if entry, ok := booruArchiveCache.entries[key]; ok {
		booruArchiveCache.Unlock()
		return entry.Data, nil // Immutable cached byte slice.
	}
	booruArchiveCache.Unlock()

	var buf bytes.Buffer
	_, err := rs.client.FetchMedia(r.Context(), source, booru.MediaRequest{
		URL: post.OriginalURL, Limit: booruArchiveMaxBytes,
		ExpectedBytes: post.Bytes, ExpectedMD5: post.MD5, AllowZIP: true,
	}, &buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if _, _, err = indexBooruArchive(data); err != nil {
		return nil, err
	}
	booruArchiveCache.Lock()
	defer booruArchiveCache.Unlock()
	if len(booruArchiveCache.entries) >= booruArchiveCacheSize {
		var oldestKey string
		var oldestTime time.Time
		for token, entry := range booruArchiveCache.entries {
			if oldestKey == "" || entry.Created.Before(oldestTime) {
				oldestKey, oldestTime = token, entry.Created
			}
		}
		delete(booruArchiveCache.entries, oldestKey)
	}
	booruArchiveCache.entries[key] = booruCachedArchive{Data: data, Created: time.Now()}
	return data, nil
}

// Archive serves a listing or one strictly verified image/video from a
// server-snapshot-approved ZIP/CBZ. The caller can select an entry index, never
// a remote URL or filesystem path. Existing application authentication applies.
func (rs booruBrowserRoutes) Archive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	key := r.URL.Query().Get("key")
	if len(key) != 48 {
		writeBooruError(w, http.StatusBadRequest, "archive", "Invalid archive key.")
		return
	}
	data, err := rs.loadBooruArchive(r, key)
	if err != nil {
		writeBooruError(w, http.StatusBadGateway, "archive", err.Error())
		return
	}
	listing, zr, err := indexBooruArchive(data)
	if err != nil {
		writeBooruError(w, http.StatusUnprocessableEntity, "archive", err.Error())
		return
	}
	fileIndex := r.URL.Query().Get("file")
	if fileIndex == "" {
		writeVisualSimilarityJSON(w, listing)
		return
	}
	n, err := strconv.Atoi(fileIndex)
	if err != nil || n < 0 || strconv.Itoa(n) != fileIndex {
		writeBooruError(w, http.StatusBadRequest, "archive", "Invalid archive media index.")
		return
	}
	var selected *booruArchiveFile
	for i := range listing.Files {
		if listing.Files[i].Index == n {
			selected = &listing.Files[i]
			break
		}
	}
	if selected == nil {
		writeBooruError(w, http.StatusNotFound, "archive", "Unsupported or missing archive media.")
		return
	}
	select {
	case booruArchiveInflations <- struct{}{}:
		defer func() { <-booruArchiveInflations }()
	case <-r.Context().Done():
		return
	}
	entry, err := zr.File[n].Open()
	if err != nil {
		writeBooruError(w, http.StatusBadGateway, "archive", "Cannot open archive entry.")
		return
	}
	defer entry.Close()
	decoded, err := io.ReadAll(io.LimitReader(entry, booruArchiveEntryMax+1))
	if err != nil || uint64(len(decoded)) != selected.Bytes || len(decoded) > booruArchiveEntryMax {
		writeBooruError(w, http.StatusUnprocessableEntity, "archive", "Archive entry is corrupt or exceeds the preview limit.")
		return
	}
	mime := booruArchiveContentType(selected.Format, decoded)
	if mime == "" {
		writeBooruError(w, http.StatusUnsupportedMediaType, "archive", "Archive entry is not a valid supported image or video.")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, selected.Name, time.Time{}, bytes.NewReader(decoded))
}
