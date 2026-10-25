package api

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestBooruSevenZipEntryNamesDenyTraversalAndUnsafeFiles(t *testing.T) {
	for _, name := range []string{"chapter/001.png", "images/002.WEBP", "video/clip.mp4"} {
		if !validBooruSevenZipName(name) {
			t.Fatalf("safe 7z member rejected: %q", name)
		}
	}
	for _, name := range []string{"../escape.png", "/absolute.png", "one/../two.jpg", "a\\b.png",
		"drive:C.png", "script.svg", "notes.txt", "nested.7z", "link.zip", "two\x00.png", "a//b.png"} {
		if validBooruSevenZipName(name) {
			t.Fatalf("unsafe 7z member accepted: %q", name)
		}
	}
}

func TestBooruSevenZipRejectsWrongMagicOrHeaderBounds(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<html>"), bytes.Repeat([]byte{0}, 32)} {
		if _, _, err := indexBooruSevenZipWithLimit(data, 256<<20); err == nil {
			t.Fatal("invalid input accepted as 7z archive")
		}
	}
	for _, format := range []string{"7z", "zip", "cbz"} {
		if !supportedBooruArchiveFormat(format) {
			t.Fatalf("supported archive format omitted: %s", format)
		}
	}
	if supportedBooruArchiveFormat("rar") {
		t.Fatal("RAR incorrectly accepted")
	}
	_, _, err := openBooruArchiveEntriesWithLimit([]byte("archive"), "rar", 256<<20)
	if err == nil || !strings.Contains(err.Error(), "unsupported archive") {
		t.Fatalf("unsupported archive not rejected: %v", err)
	}
}

func TestBooruSevenZipMediaReadNeverServesForgedContent(t *testing.T) {
	selected := booruArchiveFile{Index: 0, Name: "image.png", Bytes: 13, Format: "png"}
	_, _, err := readBooruArchiveStreamWithLimit(context.Background(), func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<html>malware")), nil
	}, selected, 128)
	if err == nil || !strings.Contains(err.Error(), "valid supported image") {
		t.Fatalf("forged media content was accepted: %v", err)
	}
}
