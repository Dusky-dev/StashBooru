package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
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
		"chapter/001.png": []byte("image bytes"),
		"chapter/002.jpg": []byte("image bytes"),
		"../../escape.jpg": []byte("unsafe"),
		"bad\\path.jpg": []byte("unsafe"),
		"private.svg": []byte("<svg/>"),
		"notes.txt": []byte("untrusted"),
		"nested.cbz": []byte("not followed"),
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
		if err != nil { t.Fatal(err) }
		if _, err := w.Write([]byte("a")); err != nil { t.Fatal(err) }
	}
	if err := zw.Close(); err != nil { t.Fatal(err) }
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
	if err != nil { t.Fatal(err) }
	if _, err := w.Write([]byte("target")); err != nil { t.Fatal(err) }
	if err := zw.Close(); err != nil { t.Fatal(err) }
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
