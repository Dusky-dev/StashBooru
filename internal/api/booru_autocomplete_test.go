package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

type autocompleteTestClient struct {
	browserToolsClient
	calls int
	fail  bool
	after func()
}

func (c *autocompleteTestClient) Autocomplete(context.Context, booru.Source, string) ([]booru.Suggestion, error) {
	c.calls++
	if c.after != nil {
		c.after()
	}
	if c.fail {
		return nil, fmt.Errorf("private transport details")
	}
	return []booru.Suggestion{{Name: "blue_hair", Category: "general", Count: 42}}, nil
}

func TestBooruAutocompleteLiveFallbackAndSourceChanges(t *testing.T) {
	client := &autocompleteTestClient{}
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: client}
	source := booru.Defaults()[0]
	path := "/booru-browser/autocomplete?source=" + source.ID + "&instance=" + source.Instance() + "&q=blue"
	request := func(extra string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		rs.Autocomplete(w, httptest.NewRequest("GET", path+extra, nil))
		return w
	}
	w := request("")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mode":"live"`) || client.calls != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	client.fail = true
	if w := request(""); w.Code != 502 || strings.Contains(w.Body.String(), "private transport") {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := os.MkdirAll(filepath.Join(rs.configPath, "booru-tags"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-tags", source.Instance()+".csv"), []byte("blue_hair,0,42,azure_hair\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w = request("")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mode":"database"`) || !strings.Contains(w.Body.String(), "blue_hair") {
		t.Fatal(w.Code, w.Body.String())
	}
	calls := client.calls
	w = request("&mode=database")
	if w.Code != 200 || client.calls != calls {
		t.Fatal("offline mode contacted provider", w.Code, w.Body.String())
	}
	client.fail = false
	client.after = func() {
		if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(`[{"id":"danbooru","disabled":true}]`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if w := request(""); w.Code != 409 {
		t.Fatal("served suggestions after source disabled", w.Code, w.Body.String())
	}
}

func TestBooruUnsupportedSortIsAClientError(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: booru.NewClient()}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/booru-browser/search?source=gelbooru&q=cat&sort=updated", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "unsupported-filter") {
		t.Fatal(w.Code, w.Body.String())
	}
}
