package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
)

func waitBooruDownload(t *testing.T, rs booruBrowserRoutes, id int, token string) booruDownloadState {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		w := httptest.NewRecorder()
		rs.DownloadStatus(w, httptest.NewRequest("GET", fmt.Sprintf("/?jobID=%d&token=%s", id, token), nil))
		var state booruDownloadState
		if json.Unmarshal(w.Body.Bytes(), &state) != nil {
			t.Fatal(w.Body.String())
		}
		if state.Status == "complete" || state.Status == "failed" || state.Status == "cancelled" {
			return state
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("download did not finish")
	return booruDownloadState{}
}
func TestBooruLibraryDownloadRetainsFileAndRetriesImport(t *testing.T) {
	root := t.TempDir()
	jobs := job.NewManager()
	defer jobs.Stop()
	client := &browserToolsClient{}
	var paths []string
	failImport := true
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: client, jobs: jobs, downloads: manager.NewDownloadStore(), libraryRoots: func() []string { return []string{root} }, importFile: func(ctx context.Context, path string, s booru.Source, p booru.Post, convert bool, progress *job.Progress) (booruLibraryResult, error) {
		if len(paths) < 2 && !convert {
			t.Error("conversion preference not passed to native import")
		}
		paths = append(paths, path)
		if failImport {
			return booruLibraryResult{}, fmt.Errorf("scanner unavailable")
		}
		return booruLibraryResult{Kind: "image", ID: "12", Path: path}, nil
	}}
	source := booru.Defaults()[0]
	key := rememberBooruPost(source, booru.Post{ID: "1", OriginalURL: source.URL + "/file.png", PostURL: source.URL + "/posts/1"})
	start := func() booruDownloadState {
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]any{"key": key, "convert": true, "destination": root})
		rs.StartDownload(w, toolRequest("POST", "/", string(body), "application/json"))
		var reply struct {
			JobID int
			Token string
		}
		if json.Unmarshal(w.Body.Bytes(), &reply) != nil || reply.JobID == 0 {
			t.Fatal(w.Code, w.Body.String())
		}
		return waitBooruDownload(t, rs, reply.JobID, reply.Token)
	}
	first := start()
	if first.Status != "failed" || first.SavedPath == "" {
		t.Fatal(first)
	}
	if data, err := os.ReadFile(first.SavedPath); err != nil || string(data) != "fixture-original" {
		t.Fatal("failed scan lost saved file", err)
	}
	failImport = false
	second := start()
	if second.Status != "complete" || second.Library == nil || second.Library.ID != "12" || len(paths) != 2 || paths[0] != paths[1] {
		t.Fatal(second, paths)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("retry duplicated file", len(entries), err)
	}
	third := start()
	if third.Status != "complete" || third.Library.ID != "12" {
		t.Fatal(third)
	}
	if _, err := os.Stat(filepath.Join(rs.configPath, "booru-download-history", receiptName(source.Instance(), "1"))); err != nil {
		t.Fatal(err)
	}
	// A changed destination must never be overwritten, even on retry.
	if err := os.WriteFile(paths[0], []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if state := start(); state.Status != "failed" || !strings.Contains(state.Error, "not overwritten") {
		t.Fatal(state)
	}
}
func TestBooruDestinationAndDefaultFolderValidation(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	rs := booruBrowserRoutes{configPath: t.TempDir(), libraryRoots: func() []string { return []string{root} }}
	for _, path := range []string{outside, filepath.Join(root, "missing"), "relative"} {
		if _, err := rs.destination(path); err == nil {
			t.Fatal("unsafe destination", path)
		}
	}
	link := filepath.Join(root, "escape")
	if os.Symlink(outside, link) == nil {
		if _, err := rs.destination(link); err == nil {
			t.Fatal("symlink escaped library")
		}
	}
	alias := filepath.Join(t.TempDir(), "library-alias")
	if os.Symlink(root, alias) == nil {
		linked := booruBrowserRoutes{libraryRoots: func() []string { return []string{alias} }}
		resolved, err := linked.destination(alias)
		if err != nil || resolved != alias {
			t.Fatal("symlink library lost native scan namespace", resolved, err)
		}
	}
	data, _ := json.Marshal(map[string]any{"convert": true, "defaultDirectory": root})
	w := httptest.NewRecorder()
	rs.UpdateDownloadSettings(w, toolRequest("POST", "/", string(data), "application/json"))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	value, err := rs.downloadSettings()
	if err != nil || !value.Convert || value.DefaultDirectory != root {
		t.Fatal(value, err)
	}
	w = httptest.NewRecorder()
	rs.Directories(w, httptest.NewRequest("GET", "/?path=", nil))
	if !strings.Contains(w.Body.String(), root) {
		t.Fatal(w.Body.String())
	}
}
func TestBooruAutocompleteCanLoadServerPath(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	source := booru.Defaults()[0]
	path := filepath.Join(t.TempDir(), "tags.csv")
	if err := os.WriteFile(path, []byte("blue_hair,0,99,azure_hair\n"), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint := "/?source=" + source.ID + "&instance=" + source.Instance()
	body, _ := json.Marshal(map[string]string{"path": path, "revision": ""})
	w := httptest.NewRecorder()
	rs.ImportTagPath(w, toolRequest("POST", endpoint, string(body), "application/json"))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	rs.Tags(w, httptest.NewRequest("GET", endpoint+"&q=azure", nil))
	if !strings.Contains(w.Body.String(), "blue_hair") || !strings.Contains(w.Body.String(), "tags.csv") {
		t.Fatal(w.Body.String())
	}
	// Invalid replacement must leave both the installed index and its path.
	bad := filepath.Join(t.TempDir(), "bad.csv")
	_ = os.WriteFile(bad, []byte("invalid"), 0600)
	var view struct{ Revision string }
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	body, _ = json.Marshal(map[string]string{"path": bad, "revision": view.Revision})
	w = httptest.NewRecorder()
	rs.ImportTagPath(w, toolRequest("POST", endpoint, string(body), "application/json"))
	if w.Code != 400 || rs.tagDatabasePath(source) != path {
		t.Fatal(w.Code, w.Body.String())
	}
}
