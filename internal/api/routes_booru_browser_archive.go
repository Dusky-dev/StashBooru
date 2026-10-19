package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
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
	booruArchiveMaxBytes       = booruDefaultFileSizeMiB << 20
	booruArchiveEntryMax       = booruDefaultFileSizeMiB << 20
	booruArchiveMaxFiles       = 512
	booruArchiveMaxMedia       = 128
	booruArchiveCacheSize      = 4
	booruArchiveCacheMaxBytes  = 256 << 20
	booruArchiveVideoCacheMax  = 32 << 20
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
	Data       []byte
	Created    time.Time
	Expires    *time.Timer
	VideoIndex int
	Video      []byte // Only small validated videos are retained for range seeking.
	VideoMIME  string
}

type booruArchiveLoad struct {
	Done chan struct{}
	Data []byte
	Err  error
}

type booruVideoLoad struct {
	Done chan struct{}
	Data []byte
	MIME string
	Err  error
}

// All maps, decoded video slots, and entry timers are guarded by Mutex.
var booruArchiveCache = struct {
	sync.Mutex
	entries map[string]*booruCachedArchive
	loading map[string]*booruArchiveLoad
	videos  map[string]*booruVideoLoad
}{
	entries: make(map[string]*booruCachedArchive),
	loading: make(map[string]*booruArchiveLoad),
	videos:  make(map[string]*booruVideoLoad),
}

var booruArchiveCacheTTL = 5 * time.Minute

func evictBooruArchiveLocked(key string) {
	if entry := booruArchiveCache.entries[key]; entry != nil {
		entry.Expires.Stop()
		delete(booruArchiveCache.entries, key)
	}
}

// Called with the cache mutex held. The timer independently removes idle
// entries, and identity checks stop a stale timer evicting a replacement.
func storeBooruArchiveLocked(key string, data []byte) {
	var usedBytes int
	for _, entry := range booruArchiveCache.entries {
		usedBytes += len(entry.Data) + len(entry.Video)
	}
	for len(booruArchiveCache.entries) > 0 && (len(booruArchiveCache.entries) >= booruArchiveCacheSize || usedBytes+len(data) > booruArchiveCacheMaxBytes) {
		var oldestKey string
		var oldestTime time.Time
		for token, entry := range booruArchiveCache.entries {
			if oldestKey == "" || entry.Created.Before(oldestTime) {
				oldestKey, oldestTime = token, entry.Created
			}
		}
		usedBytes -= len(booruArchiveCache.entries[oldestKey].Data) + len(booruArchiveCache.entries[oldestKey].Video)
		evictBooruArchiveLocked(oldestKey)
	}
	if len(data) > booruArchiveCacheMaxBytes {
		return // Larger configured archives are fetched but never held indefinitely.
	}
	entry := &booruCachedArchive{Data: data, Created: time.Now(), VideoIndex: -1}
	booruArchiveCache.entries[key] = entry
	entry.Expires = time.AfterFunc(booruArchiveCacheTTL, func() {
		booruArchiveCache.Lock()
		defer booruArchiveCache.Unlock()
		if booruArchiveCache.entries[key] == entry {
			delete(booruArchiveCache.entries, key)
		}
	})
}

// Concurrent requests for the same server-issued media key share one provider
// fetch. A waiter can cancel without aborting the request doing the download.
func cachedBooruArchive(ctx context.Context, key string, fetch func(context.Context) ([]byte, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	booruArchiveCache.Lock()
	if entry := booruArchiveCache.entries[key]; entry != nil {
		if time.Since(entry.Created) < booruArchiveCacheTTL {
			booruArchiveCache.Unlock()
			return entry.Data, nil
		}
		evictBooruArchiveLocked(key)
	}
	if pending := booruArchiveCache.loading[key]; pending != nil {
		booruArchiveCache.Unlock()
		select {
		case <-pending.Done:
			return pending.Data, pending.Err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	pending := &booruArchiveLoad{Done: make(chan struct{})}
	booruArchiveCache.loading[key] = pending
	booruArchiveCache.Unlock()

	// The shared work does not inherit the initiating HTTP request's
	// cancellation: a disconnected viewer must not abort other waiting viewers.
	go func() {
		fetchCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		data, err := fetch(fetchCtx)
		booruArchiveCache.Lock()
		if err == nil {
			storeBooruArchiveLocked(key, data)
		}
		pending.Data, pending.Err = data, err
		delete(booruArchiveCache.loading, key)
		close(pending.Done)
		booruArchiveCache.Unlock()
	}()
	select {
	case <-pending.Done:
		return pending.Data, pending.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cache only the most recently accessed decoded video per archive, so HTTP
// range requests do not reinflate the same 24 MiB entry for each seek.
func cachedBooruArchiveVideo(ctx context.Context, key string, index int, decode func(context.Context) ([]byte, string, error)) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	id := key + ":" + strconv.Itoa(index)
	booruArchiveCache.Lock()
	if entry := booruArchiveCache.entries[key]; entry != nil && entry.VideoIndex == index && entry.Video != nil {
		data, mime := entry.Video, entry.VideoMIME
		booruArchiveCache.Unlock()
		return data, mime, nil
	}
	if pending := booruArchiveCache.videos[id]; pending != nil {
		booruArchiveCache.Unlock()
		select {
		case <-pending.Done:
			return pending.Data, pending.MIME, pending.Err
		case <-ctx.Done():
			return nil, "", ctx.Err()
		}
	}
	pending := &booruVideoLoad{Done: make(chan struct{})}
	booruArchiveCache.videos[id] = pending
	booruArchiveCache.Unlock()

	go func() {
		decodeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		data, mime, err := decode(decodeCtx)
		booruArchiveCache.Lock()
		if err == nil && len(data) <= booruArchiveVideoCacheMax {
			if entry := booruArchiveCache.entries[key]; entry != nil {
				entry.VideoIndex, entry.Video, entry.VideoMIME = index, data, mime
			}
		}
		pending.Data, pending.MIME, pending.Err = data, mime, err
		delete(booruArchiveCache.videos, id)
		close(pending.Done)
		booruArchiveCache.Unlock()
	}()
	select {
	case <-pending.Done:
		return pending.Data, pending.MIME, pending.Err
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
}

// Limit decompression as well as remote media fetching. ZIP inflation can be
// expensive independently of the FetchMedia concurrency gate.
var booruArchiveInflations = make(chan struct{}, 4)

func validBooruArchiveEntryWithLimit(f *zip.File, maxBytes int64) bool {
	if f.FileInfo().IsDir() || !f.Mode().IsRegular() || f.Flags&1 != 0 ||
		(f.Method != zip.Store && f.Method != zip.Deflate) ||
		f.UncompressedSize64 == 0 || f.UncompressedSize64 > uint64(maxBytes) ||
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

// checkBooruZIPDirectory rejects ZIP64, multi-disk archives, oversized central
// directories and forged entry counts BEFORE zip.NewReader allocates zip.File
// records. The preview limits do not require ZIP64 or self-extracting prefixes.
func checkBooruZIPDirectory(data []byte) error {
	const eocdLength = 22
	if len(data) < eocdLength {
		return fmt.Errorf("invalid ZIP end-of-central-directory record")
	}
	start := len(data) - eocdLength - 65535
	if start < 0 {
		start = 0
	}
	for offset := len(data) - eocdLength; offset >= start; offset-- {
		if binary.LittleEndian.Uint32(data[offset:offset+4]) != 0x06054b50 ||
			offset+eocdLength+int(binary.LittleEndian.Uint16(data[offset+20:offset+22])) != len(data) {
			continue
		}
		footer := data[offset:]
		entriesOnDisk := binary.LittleEndian.Uint16(footer[8:10])
		entries := binary.LittleEndian.Uint16(footer[10:12])
		directorySize := uint64(binary.LittleEndian.Uint32(footer[12:16]))
		directoryOffset := uint64(binary.LittleEndian.Uint32(footer[16:20]))
		if binary.LittleEndian.Uint16(footer[4:6]) != 0 ||
			binary.LittleEndian.Uint16(footer[6:8]) != 0 || entries != entriesOnDisk ||
			entries == 0xffff || entries > booruArchiveMaxFiles ||
			directorySize == 0xffffffff || directoryOffset == 0xffffffff {
			return fmt.Errorf("ZIP64, multi-disk or excessive archive directory is unsupported")
		}
		if directorySize > 2<<20 || directoryOffset+directorySize != uint64(offset) {
			return fmt.Errorf("archive directory is oversized or malformed")
		}
		position := int(directoryOffset)
		for i := 0; i < int(entries); i++ {
			if position+46 > offset || binary.LittleEndian.Uint32(data[position:position+4]) != 0x02014b50 {
				return fmt.Errorf("malformed ZIP central directory")
			}
			nameLength := int(binary.LittleEndian.Uint16(data[position+28 : position+30]))
			extraLength := int(binary.LittleEndian.Uint16(data[position+30 : position+32]))
			commentLength := int(binary.LittleEndian.Uint16(data[position+32 : position+34]))
			if binary.LittleEndian.Uint16(data[position+34:position+36]) != 0 ||
				binary.LittleEndian.Uint32(data[position+20:position+24]) == 0xffffffff ||
				binary.LittleEndian.Uint32(data[position+24:position+28]) == 0xffffffff ||
				binary.LittleEndian.Uint32(data[position+42:position+46]) == 0xffffffff ||
				uint64(binary.LittleEndian.Uint32(data[position+42:position+46])) >= directoryOffset {
				return fmt.Errorf("unsupported ZIP64 or multi-disk entry")
			}
			position += 46 + nameLength + extraLength + commentLength
			if position > offset {
				return fmt.Errorf("invalid ZIP central directory bounds")
			}
		}
		if position != offset {
			return fmt.Errorf("ZIP central directory contains undeclared records")
		}
		return nil
	}
	return fmt.Errorf("ZIP end-of-central-directory record is missing")
}

func indexBooruArchive(data []byte) (booruArchiveListing, *zip.Reader, error) {
	return indexBooruArchiveWithLimit(data, booruArchiveMaxBytes)
}

func indexBooruArchiveWithLimit(data []byte, maxBytes int64) (booruArchiveListing, *zip.Reader, error) {
	listing := booruArchiveListing{Files: []booruArchiveFile{}}
	if maxBytes < 1 || maxBytes > booruHardMaxFileSizeMiB<<20 || len(data) == 0 || int64(len(data)) > maxBytes {
		return listing, nil, fmt.Errorf("archive is empty or exceeds the configured size limit")
	}
	if err := checkBooruZIPDirectory(data); err != nil {
		return listing, nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return listing, nil, fmt.Errorf("provider did not return a valid ZIP archive: %w", err)
	}
	if len(zr.File) > booruArchiveMaxFiles {
		return listing, nil, fmt.Errorf("archive contains too many entries")
	}
	for i, f := range zr.File {
		if !validBooruArchiveEntryWithLimit(f, maxBytes) || len(listing.Files) >= booruArchiveMaxMedia {
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
		if len(data) >= 4 && (bytes.Equal(data[:4], []byte{0x49, 0x49, 0x2a, 0x00}) ||
			bytes.Equal(data[:4], []byte{0x4d, 0x4d, 0x00, 0x2a})) {
			return "image/tiff"
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

func (rs booruBrowserRoutes) archiveFileLimit() (int64, error) {
	preferences, err := rs.downloadSettings()
	if err != nil {
		return 0, err
	}
	return int64(preferences.MaxFileSizeMiB) << 20, nil
}

func (rs booruBrowserRoutes) loadBooruArchive(r *http.Request, key string) ([]byte, error) {
	limit, err := rs.archiveFileLimit()
	if err != nil {
		return nil, err
	}
	source, post, ok := rs.mediaSnapshot(key)
	if !ok {
		return nil, fmt.Errorf("archive preview expired or its source changed; refresh the search")
	}
	if post.OriginalURL == "" || source.ApprovedMediaURL(post.OriginalURL) != post.OriginalURL ||
		(strings.ToLower(post.Format) != "zip" && strings.ToLower(post.Format) != "cbz") {
		return nil, fmt.Errorf("this post does not have a supported ZIP/CBZ archive")
	}
	if post.Bytes > limit {
		return nil, fmt.Errorf("archive exceeds the configured %d MiB limit; open the remote post", limit>>20)
	}
	data, err := cachedBooruArchive(r.Context(), key, func(ctx context.Context) ([]byte, error) {
		var buf bytes.Buffer
		_, fetchErr := rs.client.FetchMedia(ctx, source, booru.MediaRequest{
			URL: post.OriginalURL, Limit: limit,
			ExpectedBytes: post.Bytes, ExpectedMD5: post.MD5, AllowZIP: true,
		}, &buf)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if _, _, fetchErr = indexBooruArchiveWithLimit(buf.Bytes(), limit); fetchErr != nil {
			return nil, fetchErr
		}
		return buf.Bytes(), nil
	})
	if err == nil {
		_, _, err = indexBooruArchiveWithLimit(data, limit) // Also revalidate cached bytes after a settings change.
	}
	return data, err
}

// The inflation semaphore only covers decompression and validation, never
// HTTP response transfer. Range requests reuse bounded decoded video bytes.
func readBooruArchiveEntry(ctx context.Context, file *zip.File, selected booruArchiveFile) ([]byte, string, error) {
	return readBooruArchiveEntryWithLimit(ctx, file, selected, booruArchiveEntryMax)
}

func readBooruArchiveEntryWithLimit(ctx context.Context, file *zip.File, selected booruArchiveFile, maxBytes int64) ([]byte, string, error) {
	select {
	case booruArchiveInflations <- struct{}{}:
		defer func() { <-booruArchiveInflations }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	entry, err := file.Open()
	if err != nil {
		return nil, "", fmt.Errorf("cannot open archive entry: %w", err)
	}
	defer entry.Close()
	decoded, err := io.ReadAll(io.LimitReader(entry, maxBytes+1))
	if err != nil || uint64(len(decoded)) != selected.Bytes || int64(len(decoded)) > maxBytes {
		return nil, "", fmt.Errorf("archive entry is corrupt or exceeds the preview limit")
	}
	mime := booruArchiveContentType(selected.Format, decoded)
	if mime == "" {
		return nil, "", fmt.Errorf("archive entry is not a valid supported image or video")
	}
	return decoded, mime, nil
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
	limit, err := rs.archiveFileLimit()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "archive", "Cannot read archive size configuration.")
		return
	}
	listing, zr, err := indexBooruArchiveWithLimit(data, limit)
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
	decode := func(ctx context.Context) ([]byte, string, error) {
		return readBooruArchiveEntryWithLimit(ctx, zr.File[n], *selected, limit)
	}
	var decoded []byte
	var mime string
	if selected.Format == "mp4" || selected.Format == "webm" {
		decoded, mime, err = cachedBooruArchiveVideo(r.Context(), key, n, decode)
	} else {
		decoded, mime, err = decode(r.Context())
	}
	if err != nil {
		writeBooruError(w, http.StatusUnprocessableEntity, "archive", err.Error())
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, selected.Name, time.Time{}, bytes.NewReader(decoded))
}
