package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMediaUploadSettingsDefaultDisabled(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	settings, err := rs.readMediaUploadSettings()
	if err != nil || settings.Enabled || !settings.AllowURL || !settings.AllowFolderCreation {
		t.Fatalf("experimental uploads must be opt-in: %+v %v", settings, err)
	}
	for _, endpoint := range []struct {
		apply func(http.ResponseWriter, *http.Request)
		path  string
	}{
		{rs.UploadMedia, "/media-upload"},
		{rs.ImportMediaURL, "/media-upload/url"},
		{rs.CreateUploadFolder, "/media-upload/folder"},
	} {
		req := httptest.NewRequest(http.MethodPost, endpoint.path, nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		endpoint.apply(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s did not reject disabled experimental uploads: %d", endpoint.path, w.Code)
		}
	}
}

func TestMediaUploadSettingsPersistAndEnforceSubfeatures(t *testing.T) {
	root := t.TempDir()
	rs := booruBrowserRoutes{
		configPath:   t.TempDir(),
		libraryRoots: func() []string { return []string{root} },
	}
	body := []byte(`{"enabled":true,"allowURL":false,"allowFolderCreation":false,"defaultDirectory":` +
		strconv.Quote(root) + `}`)
	req := httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/settings", bytes.NewReader(body))
	req.Header.Set("Origin", "http://stash.test")
	w := httptest.NewRecorder()
	rs.UpdateMediaUploadSettings(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", w.Code, w.Body.String())
	}
	read, err := rs.readMediaUploadSettings()
	if err != nil || !read.Enabled || read.AllowURL || read.AllowFolderCreation || read.DefaultDirectory != root {
		t.Fatalf("settings not persisted: %+v %v", read, err)
	}
	for _, entry := range []struct {
		handler func(http.ResponseWriter, *http.Request)
		path    string
	}{
		{rs.ImportMediaURL, "/media-upload/url"},
		{rs.CreateUploadFolder, "/media-upload/folder"},
	} {
		req = httptest.NewRequest(http.MethodPost, "http://stash.test"+entry.path, nil)
		w = httptest.NewRecorder()
		entry.handler(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s bypassed disabled subfeature: %d", entry.path, w.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/settings", bytes.NewReader(body))
	req.Header.Set("Origin", "http://evil.test")
	w = httptest.NewRecorder()
	rs.UpdateMediaUploadSettings(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatal("cross-origin settings update accepted")
	}
	req = httptest.NewRequest(http.MethodPost, "http://stash.test/media-upload/settings", bytes.NewReader([]byte(`{"enabled":true,"defaultDirectory":"/outside"}`)))
	w = httptest.NewRecorder()
	rs.UpdateMediaUploadSettings(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatal("outside default directory accepted")
	}
}
