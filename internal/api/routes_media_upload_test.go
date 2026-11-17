package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/models"
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

type mediaUploadTestTxn struct{}

func (mediaUploadTestTxn) Begin(ctx context.Context, _ bool) (context.Context, error) {
	return ctx, nil
}

func (mediaUploadTestTxn) Commit(context.Context) error { return nil }

func (mediaUploadTestTxn) Rollback(context.Context) error { return nil }

func (mediaUploadTestTxn) IsLocked(error) bool { return false }

func (mediaUploadTestTxn) WithDatabase(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func TestMediaUploadReportsSavedFileWhenImporterUnavailable(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rs := booruBrowserRoutes{
		configPath:   t.TempDir(),
		libraryRoots: func() []string { return []string{dir} },
		repository:   models.Repository{TxnManager: mediaUploadTestTxn{}},
	}
	if err := rs.saveMediaUploadSettings(mediaUploadSettings{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	if err := writer.WriteField("destination", dir); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("file", "retained.png")
	if err != nil {
		t.Fatal(err)
	}
	const original = "isolated upload fixture"
	if _, err := io.WriteString(file, original); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload", &data)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	rs.UploadMedia(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected importer failure, got %d: %s", w.Code, w.Body.String())
	}

	var problem struct {
		Code      string              `json:"code"`
		SavedPath string              `json:"savedPath"`
		Result    *booruLibraryResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "import" || problem.SavedPath != filepath.Join(dir, "retained.png") || problem.Result != nil {
		t.Fatalf("invalid saved-file recovery information: %+v", problem)
	}
	if raw, err := os.ReadFile(problem.SavedPath); err != nil || string(raw) != original {
		t.Fatalf("saved bytes missing or changed: %q, %v", raw, err)
	}
}
