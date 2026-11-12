package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestMediaUploadFolderNames(t *testing.T) {
	for _, name := range []string{"New album", "art_2026", "漫画"} {
		if !validMediaUploadFolderName(name) {
			t.Fatalf("valid name %q rejected", name)
		}
	}
	for _, name := range []string{"", ".", "..", "../other", "sub/folder", ".hidden", "bad\\name", "C:name", " bad", "x\n"} {
		if validMediaUploadFolderName(name) {
			t.Fatalf("unsafe name %q accepted", name)
		}
	}
}

func TestCreateUploadFolderWithinLibrary(t *testing.T) {
	root := t.TempDir()
	rs := booruBrowserRoutes{configPath: t.TempDir(), libraryRoots: func() []string { return []string{root} }}
	if err := rs.saveMediaUploadSettings(mediaUploadSettings{Enabled: true, AllowURL: true, AllowFolderCreation: true}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"parent":` + strconv.Quote(root) + `,"name":"New art"}`)
	request := httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/folder", bytes.NewReader(body))
	request.Header.Set("Origin", "http://stash.test")
	w := httptest.NewRecorder()
	rs.CreateUploadFolder(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("create folder: %d %s", w.Code, w.Body.String())
	}
	if info, err := os.Stat(filepath.Join(root, "New art")); err != nil || !info.IsDir() {
		t.Fatalf("folder not created: %v", err)
	}
	request = httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/folder", bytes.NewReader(body))
	w = httptest.NewRecorder()
	rs.CreateUploadFolder(w, request)
	if w.Code != http.StatusConflict {
		t.Fatalf("existing folder accepted: %d", w.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/folder", bytes.NewReader([]byte(`{"parent":"/tmp","name":"invalid"}`)))
	w = httptest.NewRecorder()
	rs.CreateUploadFolder(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("outside root accepted: %d", w.Code)
	}
}
