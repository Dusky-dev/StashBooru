package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaUploadFileNaming(t *testing.T) {
	for _, name := range []string{"demo.JPG", "test.jxl", "clip.mkv", "space name.avif"} {
		if got, err := safeMediaUploadName(name); err != nil || got != name {
			t.Fatalf("valid upload filename %q: %q %v", name, got, err)
		}
	}
	for _, name := range []string{"", "../escape.jpg", "sub/file.jpg", "..\\outside.png", ".hidden.mp4", "malware.exe", "image.svg", "evil\\n.jpg", "C:foo.mp4"} {
		if _, err := safeMediaUploadName(name); err == nil {
			t.Errorf("unsafe upload filename accepted: %q", name)
		}
	}
}

func TestMediaUploadMetadataIDs(t *testing.T) {
	ids, err := parseMediaUploadIDs("1,3,20", 3)
	if err != nil || len(ids) != 3 {
		t.Fatalf("IDs incorrectly parsed: %v %v", ids, err)
	}
	for _, value := range []string{"0", "-1", "1,1", "1,a", "1,,2", "1, 2", "1,2,3,4"} {
		if _, err := parseMediaUploadIDs(value, 3); err == nil {
			t.Errorf("invalid selection accepted: %q", value)
		}
	}
}

func TestMediaUploadPublishDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".upload.part")
	if err := os.WriteFile(stage, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(existing, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := publishMediaUpload(stage, dir, "test.jpg")
	if err != nil || filepath.Base(result) != "test_1.jpg" {
		t.Fatalf("collision handling failed: %s %v", result, err)
	}
	content, err := os.ReadFile(existing)
	if err != nil || string(content) != "old" {
		t.Fatal("the original media file was replaced")
	}
	data, err := os.ReadFile(result)
	if err != nil || string(data) != "new" {
		t.Fatal("published media did not match staged bytes")
	}
}

func TestMediaUploadRejectsCrossOriginAndBadFields(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	if err := rs.saveMediaUploadSettings(mediaUploadSettings{Enabled: true, AllowURL: true, AllowFolderCreation: true}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"http://attacker.test", "https://attacker.test", "http://stash.test.evil"} {
		req := httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		rs.UploadMedia(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("foreign origin %q accepted: %d", origin, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	rs.UploadMedia(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatal("cross-site fetch accepted")
	}
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	if err := writer.WriteField("unrecognized", "input"); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("file", "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(file, strings.NewReader("fake")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	rs.UploadMedia(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field was accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestMediaUploadProcessingFlagsParseStrictly(t *testing.T) {
	for _, input := range []string{"true", "false"} {
		value, err := parseMediaUploadBoolean(input)
		if err != nil || value != (input == "true") {
			t.Errorf("processing flag %q: %v, %v", input, value, err)
		}
	}
	for _, input := range []string{"", "1", "TRUE", "yes", "false ", " true", "invalid"} {
		if _, err := parseMediaUploadBoolean(input); err == nil {
			t.Errorf("accepted malformed processing flag %q", input)
		}
	}
}
