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

type blacklistRouteClient struct{ booruBrowserClient }

func (blacklistRouteClient) Search(_ context.Context, source booru.Source, _ string, page, limit int) (booru.Result, error) {
	total := int64(50)
	return booru.Result{
		SourceID: source.ID, Page: page, Limit: limit, HasMore: true, Total: &total,
		Posts: []booru.Post{
			{ID: "1", SourceID: source.ID, Tags: []booru.Tag{{Name: "Hidden_Tag", Category: "general"}}},
			{ID: "2", SourceID: source.ID, Tags: []booru.Tag{{Name: "visible", Category: "general"}}},
		},
	}, nil
}

func TestBooruSearchPerSourceBlacklist(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: blacklistRouteClient{}}
	configured := booru.Defaults()[0]
	configured.BlacklistTags = []string{"hidden tag"}
	data, err := json.Marshal([]booru.Source{configured})
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600); err != nil { t.Fatal(err) }
	for _, tc := range []struct{ source string; wantHidden int }{{"danbooru", 1}, {"gelbooru", 0}} {
		w := httptest.NewRecorder()
		rs.Search(w, httptest.NewRequest("GET", "/?source="+tc.source+"&page=2&limit=2", nil))
		if w.Code != 200 { t.Fatalf("%s: %d %s", tc.source, w.Code, w.Body.String()) }
		var got booru.Result
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil { t.Fatal(err) }
		if got.BlacklistedCount != tc.wantHidden || len(got.Posts) != 2-tc.wantHidden || got.Total == nil || *got.Total != 50 || !got.HasMore || got.Page != 2 {
			t.Fatalf("%s: wrong source filtering or pagination: %+v", tc.source, got)
		}
		if tc.wantHidden > 0 && (got.Posts[0].ID != "2" || strings.Contains(w.Body.String(), "Hidden_Tag")) {
			t.Fatalf("blacklisted post leaked in response: %s", w.Body.String())
		}
	}
}

func TestBooruSettingsBlacklistRoundTrip(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	preset := booru.Defaults()[0]
	preset.BlacklistTags = []string{"artist_a", "another tag"}
	update := booruSettingsUpdate{Revision: settingsRead(t, rs).Revision, Action: "save", Source: preset}
	payload, err := json.Marshal(update)
	if err != nil { t.Fatal(err) }
	w := settingsWrite(rs, string(payload))
	if w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	view := settingsRead(t, rs)
	if len(view.Sources[0].BlacklistTags) != 2 || view.Sources[0].BlacklistTags[1] != "another tag" { t.Fatal("blacklist not returned to editor") }
	preset.BlacklistTags = []string{"bad", "BAD"}
	update = booruSettingsUpdate{Revision: view.Revision, Action: "save", Source: preset}
	payload, _ = json.Marshal(update)
	if w = settingsWrite(rs, string(payload)); w.Code != 400 { t.Fatalf("accepted duplicate blacklist: %d", w.Code) }
	if len(settingsRead(t, rs).Sources[0].BlacklistTags) != 2 { t.Fatal("rejected save changed stored blacklist") }
}
