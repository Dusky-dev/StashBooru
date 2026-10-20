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
	w := httptest.NewRecorder()
	rs.AccountFavorites(w, httptest.NewRequest("GET", "/?source=fixture&instance="+s.Instance(), nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got booru.Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.BlacklistedCount != 1 || len(got.Posts) != 1 || got.Posts[0].ID != "2" || !got.HasMore {
		t.Fatalf("bad filtered favorites: %+v", got)
	}
}
