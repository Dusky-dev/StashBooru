package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestAccountFavoritesRespectsSourceBlacklist(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	s := booru.Source{
		ID: "fixture", Name: "Fixture", Engine: "gelbooru", URL: "https://board.example.org",
		UserID: "42", APIKey: "not-a-real-key", BlacklistTags: []string{"blocked"},
	}
	data, err := json.Marshal([]booru.Source{s})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	rs.client = accountFavoritesTestClient{favorites: func(_ booru.Source, _, _ int) (booru.Result, error) {
		return booru.Result{
			Posts: []booru.Post{
				{ID: "1", Tags: []booru.Tag{{Name: "blocked"}}},
				{ID: "2", Tags: []booru.Tag{{Name: "okay"}}},
			},
			HasMore: true,
		}, nil
	}}
	for _, tc := range []struct {
		mode       string
		wantHidden int
		wantPosts  int
	}{{"", 1, 1}, {"&blacklist=show", 0, 2}} {
		w := httptest.NewRecorder()
		rs.AccountFavorites(w, httptest.NewRequest("GET", "/?source=fixture&instance="+s.Instance()+tc.mode, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var got booru.Result
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.BlacklistedCount != tc.wantHidden || len(got.Posts) != tc.wantPosts || !got.HasMore || got.Posts[len(got.Posts)-1].ID != "2" {
			t.Fatalf("wrong favorites for blacklist mode %q: %+v", tc.mode, got)
		}
		if tc.wantHidden == 0 && got.Posts[0].ID != "1" {
			t.Fatalf("missing formerly blacklisted favorite: %+v", got)
		}
	}
	for _, bad := range []string{"&blacklist=", "&blacklist=off", "&blacklist=show&blacklist=show"} {
		w := httptest.NewRecorder()
		rs.AccountFavorites(w, httptest.NewRequest("GET", "/?source=fixture&instance="+s.Instance()+bad, nil))
		if w.Code != 400 {
			t.Fatalf("accepted invalid favorites blacklist mode %q: %d", bad, w.Code)
		}
	}
}
