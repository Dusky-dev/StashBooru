package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestBooruFollowRequestBoundaries(t *testing.T) {
	rs := booruBrowserRoutes{}
	for _, test := range []struct {
		body, origin, contentType string
		status                    int
	}{
		{`{}`, "", "application/json", 400},
		{`{"action":"create","source":"danbooru","instance":"x","name":"A","kind":"artist","target":"a"}`, "", "application/json", 503},
		{`{"action":"create","source":"danbooru","instance":"x","name":"A","kind":"artist","target":""}`, "", "application/json", 400},
		{`{"action":"unfollow","id":"ABCDEFGHIJKLMNOPQRSTUVWXYZ","version":1,"target":"ignored"}`, "", "application/json", 400},
		{`{"action":"refollow","id":"ABCDEFGHIJKLMNOPQRSTUVWXYZ","version":0}`, "", "application/json", 400},
		{`{"action":"create","autoDownload":true}`, "", "application/json", 400},
		{`{} {}`, "", "application/json", 400},
		{`{}`, "https://another.example", "application/json", 403},
		{`{}`, "", "text/plain", 415},
		{strings.Repeat("x", 65<<10), "", "application/json", 400},
	} {
		r := httptest.NewRequest("POST", "/booru-browser/follows", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.contentType)
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		rs.UpdateFollow(w, r)
		require.Equal(t, test.status, w.Code, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
	w := httptest.NewRecorder()
	rs.Follows(w, httptest.NewRequest("GET", "/booru-browser/follows", nil))
	require.Equal(t, 503, w.Code)
}

func TestBooruFollowSourceResolution(t *testing.T) {
	source := booru.Defaults()[0]
	row := models.BooruFollow{Instance: source.Instance(), SourceID: source.ID}
	alias := source
	alias.ID = "alias"
	changed := source
	changed.URL = "https://other.example.org"
	for _, test := range []struct {
		sources []booru.Source
		status  string
		id      string
	}{
		{nil, "source-unavailable", ""},
		{[]booru.Source{changed}, "source-changed", ""},
		{[]booru.Source{alias, changed}, "available", "alias"},
		{[]booru.Source{alias, source}, "available", source.ID},
	} {
		view := resolveBooruFollow(row, test.sources)
		require.Equal(t, test.status, view.Status)
		require.Equal(t, test.id, view.ResolvedSource)
		require.Equal(t, row, view.BooruFollow, "resolution must not rewrite persisted identity")
	}
}
