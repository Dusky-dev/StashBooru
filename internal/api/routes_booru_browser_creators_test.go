package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

type creatorTestClient struct {
	booruBrowserClient
	search func(context.Context, booru.Source, string, int) (booru.CreatorResult, error)
}

func (c creatorTestClient) SearchCreators(ctx context.Context, s booru.Source, q string, p int) (booru.CreatorResult, error) {
	return c.search(ctx, s, q, p)
}

func TestBooruCreatorRouteSourceBindingAndValidation(t *testing.T) {
	var source booru.Source
	for _, s := range booru.Defaults() {
		if s.ID == "pawchive" {
			source = s
		}
	}
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	calls := 0
	rs.client = creatorTestClient{booruBrowserClient: booru.NewClient(), search: func(ctx context.Context, s booru.Source, q string, p int) (booru.CreatorResult, error) {
		calls++
		if s.Instance() != source.Instance() || q != "Artist" || p != 2 {
			t.Fatal(s, q, p)
		}
		return booru.CreatorResult{Creators: []booru.Creator{{ID: "42", Name: "Artist", Service: "patreon"}}, Page: p}, nil
	}}
	endpoint := "/?source=pawchive&instance=" + source.Instance() + "&q=Artist&page=2"
	w := httptest.NewRecorder()
	rs.Creators(w, httptest.NewRequest("GET", endpoint, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"service":"patreon"`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"/?source=pawchive&instance=obsolete&q=Artist", "/?source=pawchive&instance=" + source.Instance() + "&q=a", endpoint + "x"} {
		w = httptest.NewRecorder()
		rs.Creators(w, httptest.NewRequest("GET", q, nil))
		if w.Code != 400 && w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal("invalid directory request reached provider", calls)
	}
	rs.client = creatorTestClient{search: func(ctx context.Context, s booru.Source, q string, p int) (booru.CreatorResult, error) {
		if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(`[{"id":"pawchive","disabled":true}]`), 0600); err != nil {
			t.Fatal(err)
		}
		return booru.CreatorResult{Creators: []booru.Creator{}}, nil
	}}
	w = httptest.NewRecorder()
	rs.Creators(w, httptest.NewRequest("GET", endpoint, nil))
	if w.Code != 409 {
		t.Fatal("stale directory returned", w.Code, w.Body.String())
	}
}

type categorizedPostTestClient struct{ booruBrowserClient }

func (c categorizedPostTestClient) Search(ctx context.Context, s booru.Source, q string, p, l int) (booru.Result, error) {
	return booru.Result{Posts: []booru.Post{{ID: "42", Tags: []booru.Tag{{Name: "artist_a", Category: "unknown"}, {Name: "hero", Category: "unknown"}, {Name: "alias_a", Category: "unknown"}, {Name: "native", Category: "meta"}}}}}, nil
}
func TestBooruBrowserUsesExactInstalledTagCategories(t *testing.T) {
	var source booru.Source
	for _, s := range booru.Defaults() {
		if s.ID == "gelbooru" {
			source = s
		}
	}
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: categorizedPostTestClient{}}
	dir := filepath.Join(rs.configPath, "booru-tags")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, source.Instance()+".csv"), []byte("artist_a,1,10,alias_a\nhero,4,3,\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/?source=gelbooru&q=cat", nil))
	var result booru.Result
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Posts) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	tags := result.Posts[0].Tags
	if tags[0].Category != "artist" || tags[1].Category != "character" || tags[2].Category != "unknown" || tags[3].Category != "meta" {
		t.Fatal(tags)
	}
}
