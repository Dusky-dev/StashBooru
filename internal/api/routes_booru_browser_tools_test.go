package api

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Fixture transport checksum.
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type browserToolsClient struct {
	failure      bool
	staged       string
	jpeg         bool
	mediaRequest booru.MediaRequest
}

func (c *browserToolsClient) Search(_ context.Context, s booru.Source, _ string, p, l int) (booru.Result, error) {
	return booru.Result{SourceID: s.ID, Posts: []booru.Post{{ID: "1", SourceID: s.ID, OriginalURL: s.URL + "/original.jpg", PreviewURL: s.URL + "/preview.jpg", DownloadAvailable: true}}, Page: p, Limit: l}, nil
}
func (c *browserToolsClient) FetchMedia(ctx context.Context, s booru.Source, in booru.MediaRequest, w io.Writer) (booru.MediaInfo, error) {
	c.mediaRequest = in
	if f, ok := w.(*os.File); ok {
		c.staged = f.Name()
	}
	if c.failure {
		return booru.MediaInfo{}, fmt.Errorf("checksum mismatch")
	}
	_, err := w.Write([]byte("fixture-original"))
	if c.jpeg {
		return booru.MediaInfo{ContentType: "image/jpeg", Extension: "jpg", MD5: fmt.Sprintf("%x", md5.Sum([]byte("fixture-original"))), Bytes: 16}, err
	}
	return booru.MediaInfo{ContentType: "image/png", Extension: "png", MD5: fmt.Sprintf("%x", md5.Sum([]byte("fixture-original"))), Bytes: 16}, err
}
func TestBooruMotionPreviewBoundsAndRanges(t *testing.T) {
	client := &browserToolsClient{}
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: client}
	source := booru.Defaults()[0]
	post := booru.Post{ID: "99", Format: "webm", OriginalURL: source.URL + "/motion.webm", Bytes: 16}
	key := rememberBooruPost(source, post)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/booru-browser/media?key="+key+"&kind=motion", nil)
	r.Header.Set("Range", "bytes=0-6")
	rs.Media(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "fixture" || w.Header().Get("Content-Range") != "bytes 0-6/16" {
		t.Fatal("bounded preview seeking failed", w.Code, w.Body.String(), w.Header())
	}
	if client.mediaRequest.ImagesOnly || client.mediaRequest.Limit != 24<<20 || client.mediaRequest.URL != post.OriginalURL {
		t.Fatal("incorrect motion request", client.mediaRequest)
	}
	for _, invalid := range []booru.Post{
		{ID: "99", Format: "jpg", OriginalURL: source.URL + "/original.jpg"},
		{ID: "99", Format: "webm", OriginalURL: post.OriginalURL, Bytes: 25 << 20},
		{ID: "99", Format: "gif"},
	} {
		client.mediaRequest = booru.MediaRequest{}
		key := rememberBooruPost(source, invalid)
		w := httptest.NewRecorder()
		rs.Media(w, httptest.NewRequest("GET", "/booru-browser/media?key="+key+"&kind=motion", nil))
		if w.Code < 400 || client.mediaRequest.URL != "" {
			t.Fatal("invalid motion request reached transport", w.Code, client.mediaRequest)
		}
	}
}
func toolRequest(method, path, body, contentType string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	return r
}
func TestBooruSnapshotMediaIsolation(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: &browserToolsClient{}}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/booru-browser/search?source=danbooru&q=cat", nil))
	var result booru.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || len(result.Posts) != 1 {
		t.Fatal(w.Body.String(), err)
	}
	key := result.Posts[0].MediaKey
	if len(key) != 48 || strings.Contains(w.Body.String(), "original.jpg") || !strings.HasPrefix(result.Posts[0].PreviewURL, "booru-browser/media?") {
		t.Fatal("snapshot exposed original URL or no opaque key")
	}
	w = httptest.NewRecorder()
	rs.Media(w, httptest.NewRequest("GET", "/booru-browser/media?key="+key+"&kind=preview", nil))
	if w.Code != 200 || w.Body.String() != "fixture-original" || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, url := range []string{"/booru-browser/media?key=forged&kind=preview", "/booru-browser/media?key=" + key + "&kind=original"} {
		w = httptest.NewRecorder()
		rs.Media(w, httptest.NewRequest("GET", url, nil))
		if w.Code == 200 {
			t.Fatal("accepted forged key or original preview")
		}
	}
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(`[{"id":"danbooru","disabled":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	rs.Media(w, httptest.NewRequest("GET", "/booru-browser/media?key="+key+"&kind=preview", nil))
	if w.Code != 410 {
		t.Fatal("disabled source still served media")
	}
}
func TestBooruCSVAtomicUploadAndRevisions(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	source := booru.Defaults()[0]
	path := "/booru-browser/tags?source=" + source.ID + "&instance=" + source.Instance()
	csv := "blue_hair,0,42,azure_hair\nartist_a,1,4,\nblue_hair,0,12,azure_hair\n"
	upload := func(body, revision string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := toolRequest("POST", path, body, "text/csv")
		r.Header.Set("X-Booru-Revision", revision)
		rs.ImportTags(w, r)
		return w
	}
	w := upload(csv, "")
	var db struct {
		Count       int
		Revision    string
		Suggestions []booru.Suggestion
	}
	if json.Unmarshal(w.Body.Bytes(), &db) != nil || db.Count != 2 || db.Revision == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := upload(csv, ""); w.Code != 409 {
		t.Fatal("stale CSV overwrite", w.Code)
	}
	if w := upload("bad,0,nope,", db.Revision); w.Code != 400 {
		t.Fatal("invalid CSV", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	rs.Tags(w, httptest.NewRequest("GET", path+"&q=azure", nil))
	if json.Unmarshal(w.Body.Bytes(), &db) != nil || len(db.Suggestions) != 1 || db.Suggestions[0].Name != "blue_hair" || db.Count != 2 {
		t.Fatal("failed replacement damaged index", w.Body.String())
	}
	// Discard only memory, then load the persisted database again.
	booruTagDatabases.Lock()
	booruTagDatabases.entries = map[string]booruTagDatabase{}
	booruTagDatabases.Unlock()
	w = httptest.NewRecorder()
	rs.Tags(w, httptest.NewRequest("GET", path+"&q=artist", nil))
	if !strings.Contains(w.Body.String(), "artist_a") {
		t.Fatal(w.Body.String())
	}
	r := toolRequest("POST", path, csv, "text/csv")
	r.Header.Set("Origin", "https://other.example.org")
	w = httptest.NewRecorder()
	rs.ImportTags(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin CSV write")
	}
	w = httptest.NewRecorder()
	rs.Tags(w, httptest.NewRequest("GET", strings.Replace(path, source.Instance(), "stale", 1), nil))
	if w.Code != 409 {
		t.Fatal("stale instance")
	}
}
func TestBooruDownloadSettingsPersistAndRejectCrossSite(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	w := httptest.NewRecorder()
	rs.UpdateDownloadSettings(w, toolRequest("POST", "/", `{"jpegToJXL":true}`, "application/json"))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, err := rs.downloadSettings()
	if err != nil || !got.Convert {
		t.Fatal(got, err)
	}
	for _, body := range []string{`{"apiKey":"secret"}`, `{"jpegToJXL":true} {}`} {
		w = httptest.NewRecorder()
		rs.UpdateDownloadSettings(w, toolRequest("POST", "/", body, "application/json"))
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	r := toolRequest("POST", "/", `{"jpegToJXL":false}`, "application/json")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	rs.UpdateDownloadSettings(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestBooruDownloadJobPublishesOnlyCompleteFiles(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			client := &browserToolsClient{failure: failure}
			jobs := job.NewManager()
			defer jobs.Stop()
			rs := booruBrowserRoutes{configPath: t.TempDir(), client: client, jobs: jobs, downloads: manager.NewDownloadStore()}
			source := booru.Defaults()[0]
			key := rememberBooruPost(source, booru.Post{ID: "42", OriginalURL: source.URL + "/original.png"})
			w := httptest.NewRecorder()
			rs.StartDownload(w, toolRequest("POST", "/", `{"key":"`+key+`","convert":false,"computer":true}`, "application/json"))
			var result struct {
				JobID int
				Token string
			}
			if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.JobID == 0 {
				t.Fatal(w.Code, w.Body.String())
			}
			var state booruDownloadState
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				w = httptest.NewRecorder()
				rs.DownloadStatus(w, httptest.NewRequest("GET", fmt.Sprintf("/?jobID=%d&token=%s", result.JobID, result.Token), nil))
				_ = json.Unmarshal(w.Body.Bytes(), &state)
				if state.Status == "complete" || state.Status == "failed" {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if failure {
				if state.Status != "failed" || state.URL != "" {
					t.Fatal(state)
				}
			} else {
				if state.Status != "complete" || state.Converted || state.OriginalMD5 != state.OutputMD5 {
					t.Fatal(state)
				}
				hash := strings.Split(state.URL, "/")[1]
				w = httptest.NewRecorder()
				ctx, cancel := context.WithCancel(context.Background())
				rs.downloads.Serve(hash, w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
				cancel()
				if !bytes.Equal(w.Body.Bytes(), []byte("fixture-original")) {
					t.Fatal(w.Body.String())
				}
				rs.downloads.ExpireFile(hash, time.Millisecond)
			}
			if _, err := os.Stat(filepath.Dir(client.staged)); !os.IsNotExist(err) {
				t.Fatal("staging files were retained", err)
			}
			booruDownloads.Lock()
			delete(booruDownloads.jobs, result.JobID)
			booruDownloads.Unlock()
		})
	}
}

func TestBooruQueuedDownloadCanBeCancelled(t *testing.T) {
	jobs := job.NewManager()
	defer jobs.Stop()
	started := make(chan struct{})
	release := make(chan struct{})
	jobs.Add(context.Background(), "hold queue", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error { close(started); <-release; return nil }))
	<-started
	defer close(release)
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: &browserToolsClient{}, jobs: jobs, downloads: manager.NewDownloadStore()}
	source := booru.Defaults()[0]
	key := rememberBooruPost(source, booru.Post{ID: "100", OriginalURL: source.URL + "/original.png"})
	w := httptest.NewRecorder()
	rs.StartDownload(w, toolRequest("POST", "/", `{"key":"`+key+`","convert":false,"computer":true}`, "application/json"))
	var reply struct {
		JobID int
		Token string
	}
	_ = json.Unmarshal(w.Body.Bytes(), &reply)
	w = httptest.NewRecorder()
	rs.StartDownload(w, toolRequest("POST", "/", fmt.Sprintf(`{"cancel":%d,"token":"wrong"}`, reply.JobID), "application/json"))
	if w.Code != 404 {
		t.Fatal("forged job token cancelled a download")
	}
	w = httptest.NewRecorder()
	rs.DownloadStatus(w, httptest.NewRequest("GET", fmt.Sprintf("/?jobID=%d&token=wrong", reply.JobID), nil))
	if w.Code != 404 {
		t.Fatal("reused Job ID exposed another export")
	}
	w = httptest.NewRecorder()
	rs.StartDownload(w, toolRequest("POST", "/", fmt.Sprintf(`{"cancel":%d,"token":"%s"}`, reply.JobID, reply.Token), "application/json"))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	rs.DownloadStatus(w, httptest.NewRequest("GET", fmt.Sprintf("/?jobID=%d&token=%s", reply.JobID, reply.Token), nil))
	var state booruDownloadState
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	if state.Status != "cancelled" || state.URL != "" {
		t.Fatal(state)
	}
	booruDownloads.Lock()
	delete(booruDownloads.jobs, reply.JobID)
	booruDownloads.Unlock()
}
