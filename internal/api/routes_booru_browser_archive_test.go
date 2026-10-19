package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func zipFixture(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBooruArchiveIndexFiltersUnsafeEntries(t *testing.T) {
	archive := zipFixture(t, map[string][]byte{
		"chapter/001.png":  []byte("image bytes"),
		"chapter/002.jpg":  []byte("image bytes"),
		"../../escape.jpg": []byte("unsafe"),
		"bad\\path.jpg":    []byte("unsafe"),
		"private.svg":      []byte("<svg/>"),
		"notes.txt":        []byte("untrusted"),
		"nested.cbz":       []byte("not followed"),
	})
	listing, zr, err := indexBooruArchive(archive)
	if err != nil || zr == nil || len(listing.Files) != 2 || listing.Skipped != 5 {
		t.Fatalf("archive index not bounded/sanitized: %+v %v", listing, err)
	}
	for _, file := range listing.Files {
		if !strings.HasPrefix(file.Name, "chapter/") || file.Format == "" {
			t.Fatalf("unexpected file disclosed: %+v", file)
		}
	}
}

func TestBooruArchiveRejectsCorruptOrHugeDirectory(t *testing.T) {
	if _, _, err := indexBooruArchive([]byte("<html>not a ZIP</html>")); err == nil {
		t.Fatal("HTML accepted as ZIP")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i <= booruArchiveMaxFiles; i++ {
		w, err := zw.Create(fmt.Sprintf("%03d.png", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("a")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := indexBooruArchive(buf.Bytes()); err == nil {
		t.Fatal("oversized entry table accepted")
	}
}

func TestBooruArchiveNoSymlinksOrNonMediaBytes(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: "link.jpg", Method: zip.Store}
	header.SetMode(os.ModeSymlink | 0777)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	listing, _, err := indexBooruArchive(buf.Bytes())
	if err != nil || len(listing.Files) != 0 || listing.Skipped != 1 {
		t.Fatalf("symlink accepted: %+v %v", listing, err)
	}
	if booruArchiveContentType("jpg", []byte("<html>script</html>")) != "" ||
		booruArchiveContentType("mp4", []byte("<svg>")) != "" ||
		booruArchiveContentType("webm", []byte("%PDF")) != "" {
		t.Fatal("unsafe archive entry served as media")
	}
	if booruArchiveContentType("png", []byte("\x89PNG\r\n\x1a\nrest")) != "image/png" {
		t.Fatal("PNG preview lost")
	}
	if http.DetectContentType([]byte("<svg>")) == "image/svg+xml" &&
		booruArchiveContentType("png", []byte("<svg>")) != "" {
		t.Fatal("SVG accepted")
	}
}

func TestBooruArchiveRejectsForgedDirectoryBeforeZIPReader(t *testing.T) {
	archive := zipFixture(t, map[string][]byte{"valid.png": []byte("example")})
	for name, mutate := range map[string]func([]byte){
		"too-many-records": func(data []byte) {
			binary.LittleEndian.PutUint16(data[len(data)-22+8:], 513)
			binary.LittleEndian.PutUint16(data[len(data)-22+10:], 513)
		},
		"ZIP64-count": func(data []byte) {
			binary.LittleEndian.PutUint16(data[len(data)-22+8:], 0xffff)
			binary.LittleEndian.PutUint16(data[len(data)-22+10:], 0xffff)
		},
		"oversized-directory": func(data []byte) {
			binary.LittleEndian.PutUint32(data[len(data)-22+12:], 3<<20)
		},
		"forged-entry-count": func(data []byte) {
			binary.LittleEndian.PutUint16(data[len(data)-22+8:], 0)
			binary.LittleEndian.PutUint16(data[len(data)-22+10:], 0)
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := append([]byte(nil), archive...)
			mutate(data)
			if err := checkBooruZIPDirectory(data); err == nil {
				t.Fatal("unsafe ZIP directory passed preflight")
			}
		})
	}
	if err := checkBooruZIPDirectory(archive); err != nil {
		t.Fatal("valid ZIP directory rejected:", err)
	}
}

func TestBooruArchiveIdleCacheExpires(t *testing.T) {
	key := "test-cache-expiration"
	originalTTL := booruArchiveCacheTTL
	booruArchiveCacheTTL = 20 * time.Millisecond
	t.Cleanup(func() {
		booruArchiveCacheTTL = originalTTL
		booruArchiveCache.Lock()
		evictBooruArchiveLocked(key)
		booruArchiveCache.Unlock()
	})
	_, err := cachedBooruArchive(context.Background(), key, func(context.Context) ([]byte, error) {
		return []byte("sample archive"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		booruArchiveCache.Lock()
		_, exists := booruArchiveCache.entries[key]
		booruArchiveCache.Unlock()
		if !exists {
			return // No additional archive request was needed.
		}
		select {
		case <-deadline:
			t.Fatal("idle archive cache did not expire")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestBooruArchiveConcurrentMissUsesOneDownload(t *testing.T) {
	key := "test-archive-singleflight"
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	const concurrent = 12
	var wg sync.WaitGroup
	errors := make(chan error, concurrent)
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := cachedBooruArchive(context.Background(), key, func(context.Context) ([]byte, error) {
				if calls.Add(1) == 1 {
					close(started)
				}
				<-release
				return []byte("shared archive"), nil
			})
			if err != nil {
				errors <- err
			} else if string(data) != "shared archive" {
				errors <- fmt.Errorf("bad cached bytes")
			}
		}()
	}
	<-started
	time.Sleep(30 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("archive downloaded %d times instead of once", got)
	}
	booruArchiveCache.Lock()
	evictBooruArchiveLocked(key)
	booruArchiveCache.Unlock()
}

func TestBooruArchiveVideoRangeReusesDecodedBytes(t *testing.T) {
	key := "test-video-range-cache"
	archive := zipFixture(t, map[string][]byte{
		"clip.webm": append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("video payload")...),
	})
	_, err := cachedBooruArchive(context.Background(), key, func(context.Context) ([]byte, error) {
		return archive, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		booruArchiveCache.Lock()
		evictBooruArchiveLocked(key)
		booruArchiveCache.Unlock()
	})
	listing, zr, err := indexBooruArchive(archive)
	if err != nil || len(listing.Files) != 1 {
		t.Fatalf("invalid video archive %v", err)
	}
	var inflations int
	for i := 0; i < 3; i++ {
		data, mime, err := cachedBooruArchiveVideo(context.Background(), key, listing.Files[0].Index, func(ctx context.Context) ([]byte, string, error) {
			inflations++
			return readBooruArchiveEntry(ctx, zr.File[0], listing.Files[0])
		})
		if err != nil || mime != "video/webm" || len(data) == 0 {
			t.Fatalf("video decode %d failed: %s %v", i, mime, err)
		}
		if len(booruArchiveInflations) != 0 {
			t.Fatal("inflation slot held after decompression completed")
		}
	}
	if inflations != 1 {
		t.Fatalf("video decoded %d times for repeated requests", inflations)
	}
}
