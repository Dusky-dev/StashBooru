package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

var testIcon = []byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 32, 0}

func TestSiteIconServesCachesAndRemembersMisses(t *testing.T) {
	calls := map[string]int{}
	rs := booruBrowserRoutes{configPath: t.TempDir(), iconDir: t.TempDir()}
	rs.fetchIcon = func(_ context.Context, source booru.Source) ([]byte, string, error) {
		calls[source.ID]++
		if source.ID == "gelbooru" {
			return nil, "", fmt.Errorf("%w: status 404", booru.ErrNoIcon)
		}
		return testIcon, "image/x-icon", nil
	}
	get := func(id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		rs.SiteIcon(w, httptest.NewRequest("GET", "/booru-browser/icon?source="+id, nil))
		return w
	}
	for range 2 {
		w := get("danbooru")
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/x-icon" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("icon not served: %d %v", w.Code, w.Header())
		}
	}
	if calls["danbooru"] != 1 {
		t.Fatalf("icon should be fetched once and then cached, fetched %d times", calls["danbooru"])
	}
	for range 2 {
		if w := get("gelbooru"); w.Code != http.StatusNotFound {
			t.Fatalf("missing icon should be 404, got %d", w.Code)
		}
	}
	if calls["gelbooru"] != 1 {
		t.Fatalf("a miss should be remembered, fetched %d times", calls["gelbooru"])
	}
	// Unknown or malformed source IDs never reach the network.
	for _, id := range []string{"not-configured", "../etc", "https%3A%2F%2Fexample.org"} {
		if w := get(id); w.Code != http.StatusNotFound {
			t.Fatalf("%q: expected 404, got %d", id, w.Code)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("unexpected fetches: %v", calls)
	}
}

func TestSiteIconRetriesAfterCancelledFetch(t *testing.T) {
	calls := 0
	rs := booruBrowserRoutes{configPath: t.TempDir(), iconDir: t.TempDir()}
	rs.fetchIcon = func(context.Context, booru.Source) ([]byte, string, error) {
		calls++
		if calls == 1 {
			return nil, "", context.Canceled
		}
		return testIcon, "image/x-icon", nil
	}
	for i, want := range []int{http.StatusNotFound, http.StatusOK} {
		w := httptest.NewRecorder()
		rs.SiteIcon(w, httptest.NewRequest("GET", "/booru-browser/icon?source=danbooru", nil))
		if w.Code != want {
			t.Fatalf("request %d: got %d want %d", i, w.Code, want)
		}
	}
}
