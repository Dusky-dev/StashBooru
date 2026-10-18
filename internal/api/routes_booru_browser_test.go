package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestBooruBrowserSourcesAndReadOnlyValidation(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: booru.NewClient()}
	w := httptest.NewRecorder()
	rs.Sources(w, httptest.NewRequest("GET", "/booru-browser/sources", nil))
	var sources []booru.PublicSource
	if err := json.Unmarshal(w.Body.Bytes(), &sources); err != nil || len(sources) != len(booru.Defaults()) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bad sources response: %s %v", w.Body.String(), err)
	}
	for _, query := range []string{"source=unknown&q=cat", "source=danbooru&q=", "source=danbooru&q=cat&page=-1", "source=danbooru&q=cat&limit=1000", "source=danbooru&q=cat&page=garbage"} {
		w = httptest.NewRecorder()
		rs.Search(w, httptest.NewRequest("GET", "/booru-browser/search?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query not rejected: %s %d", query, w.Code)
		}
	}
	path := filepath.Join(rs.configPath, "booru-browser.json")
	if err := os.WriteFile(path, []byte(`[{"id":"fixture","name":"Fixture","engine":"gelbooru","url":"https://board.example.org","apiKey":"private-secret","userID":"42"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	rs.Sources(w, httptest.NewRequest("GET", "/booru-browser/sources", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-secret") || !strings.Contains(w.Body.String(), `"credentialsConfigured":true`) {
		t.Fatalf("bad private source response: %s", w.Body.String())
	}
	if err := os.WriteFile(path, []byte(`{"private-secret":"bad config"}`), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	rs.Sources(w, httptest.NewRequest("GET", "/booru-browser/sources", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private-secret") {
		t.Fatalf("configuration error leaked: %s", w.Body.String())
	}
}
