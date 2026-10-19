package api

import (
	"context"
	"encoding/json"
	"github.com/stashapp/stash/pkg/booru"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type accountFavoritesTestClient struct {
	booruBrowserClient
	favorites func(booru.Source, int, int) (booru.Result, error)
}

func (c accountFavoritesTestClient) AccountFavorites(_ context.Context, s booru.Source, p, l int) (booru.Result, error) {
	return c.favorites(s, p, l)
}
func TestAccountFavoritesRouteBindingAndProjection(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	s := booru.Source{ID: "fixture", Name: "Fixture", Engine: "gelbooru", URL: "https://board.example.org", UserID: "123", APIKey: "account-secret"}
	write := func() {
		data, err := json.Marshal([]booru.Source{s})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	dir := filepath.Join(rs.configPath, "booru-tags")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, s.Instance()+".csv"), []byte("artist_a,1,10,\n"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	rs.client = accountFavoritesTestClient{favorites: func(source booru.Source, p, l int) (booru.Result, error) {
		calls++
		if source.APIKey != s.APIKey || p != 2 || l != 3 {
			t.Fatal("lost account/page")
		}
		return booru.Result{Posts: []booru.Post{{ID: "42", SourceID: s.ID, OriginalURL: s.URL + "/original.png", PreviewURL: s.URL + "/preview.png", Tags: []booru.Tag{{Name: "artist_a", Category: "unknown"}}}}}, nil
	}}
	endpoint := "/?source=fixture&instance=" + s.Instance() + "&page=2&limit=3"
	request := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		rs.AccountFavorites(w, httptest.NewRequest("GET", url, nil))
		return w
	}
	w := request(endpoint)
	if w.Code != 200 || strings.Contains(w.Body.String(), "account-secret") || strings.Contains(w.Body.String(), "original.png") || !strings.Contains(w.Body.String(), "mediaKey") || !strings.Contains(w.Body.String(), `"category":"artist"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"&sort=newest", "&rating=safe", "&q=cat", "&limit=51", "&page=no", "&instance=old"} {
		url := endpoint
		if strings.Contains(q, "limit=") {
			url = "/?source=fixture&instance=" + s.Instance() + "&page=2"
		}
		if strings.Contains(q, "page=") {
			url = "/?source=fixture&instance=" + s.Instance() + "&limit=3"
		}
		if strings.Contains(q, "instance=") {
			url = "/?source=fixture&page=2&limit=3"
		}
		w = request(url + q)
		if w.Code != 400 && w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal("invalid request reached account", calls)
	}
	rs.client = accountFavoritesTestClient{favorites: func(booru.Source, int, int) (booru.Result, error) {
		s.APIKey = "replacement-secret"
		write()
		return booru.Result{Posts: []booru.Post{}}, nil
	}}
	if w = request(endpoint); w.Code != 409 {
		t.Fatal("changed account response retained", w.Code, w.Body.String())
	}
}

type emptySearchTestClient struct{ booruBrowserClient }

func (emptySearchTestClient) Search(_ context.Context, s booru.Source, _ string, p, l int) (booru.Result, error) {
	return booru.Result{SourceID: s.ID, Page: p, Limit: l, Posts: []booru.Post{}}, nil
}
func TestBooruSearchRouteAllowsBlankQuery(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: emptySearchTestClient{}}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/?source=danbooru&q=", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
