package booru

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func gelbooruRecords(ids ...int) string {
	rows := []map[string]any{}
	for _, id := range ids {
		rows = append(rows, map[string]any{"id": id, "tags": "artist_a", "preview_url": "https://cdn.example.org/p.jpg"})
	}
	data, err := json.Marshal(map[string]any{"post": rows, "@attributes": map[string]any{"count": 1000}})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func span(high, low int) []int {
	ids := []int{}
	for i := high; i >= low; i-- {
		ids = append(ids, i)
	}
	return ids
}

func TestFollowingNumericPagesBootstrapResumeRepeatsAndBoundary(t *testing.T) {
	pages := map[string]string{}
	client := testClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "artist_a", r.URL.Query().Get("tags"), "the follow target is searched as is")
		pid := r.URL.Query().Get("pid")
		body, ok := pages[pid]
		require.True(t, ok, "unexpected page pid=%s", pid)
		return response(200, body), nil
	})
	source := testSource("gelbooru")
	// First check: one recent window only.
	pages["0"] = gelbooruRecords(span(100, 51)...)
	page, err := client.FollowPage(context.Background(), source, "artist", "artist_a", "")
	require.NoError(t, err)
	require.Len(t, page.Posts, 50)
	require.False(t, page.HasMore)
	var state pagedFollowCursor
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(100), state.Checkpoint)

	// 70 new posts: page 1 is 170..121, then 5 more arrive before page 2,
	// pushing 125..121 onto page 2 as repeats.
	pages["0"] = gelbooruRecords(span(170, 121)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "artist_a", page.Cursor)
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Len(t, page.Posts, 50)
	pages["1"] = gelbooruRecords(span(125, 76)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "artist_a", page.Cursor)
	require.NoError(t, err)
	require.False(t, page.HasMore, "stops at the old checkpoint")
	require.Len(t, page.Posts, 20, "120..101 are new; 125..121 are repeats")
	require.Equal(t, "120", page.Posts[0].ID)
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(170), state.Checkpoint)
	require.Zero(t, state.Page)

	// The posts pushed down are fetched by the next check.
	pages["0"] = gelbooruRecords(span(175, 126)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "artist_a", page.Cursor)
	require.NoError(t, err)
	require.Len(t, page.Posts, 5)
	require.False(t, page.HasMore)
}

func TestFollowingNumericPagesRejectsOrderAndCaps(t *testing.T) {
	source := testSource("gelbooru")
	client := testClient(func(r *http.Request) (*http.Response, error) {
		return response(200, gelbooruRecords(5, 9, 3)), nil
	})
	_, err := client.FollowPage(context.Background(), source, "query", "artist_a sort:score", "")
	require.Error(t, err, "results not newest first must not advance a checkpoint")

	calls := 0
	client = testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		pid, _ := strconv.Atoi(r.URL.Query().Get("pid"))
		high := 100000 - pid*50
		return response(200, gelbooruRecords(span(high, high-49)...)), nil
	})
	cursor := `{"initialized":true,"checkpoint":"1"}`
	var page FollowPage
	for range followPageCap + 2 {
		page, err = client.FollowPage(context.Background(), source, "artist", "artist_a", cursor)
		require.NoError(t, err)
		cursor = page.Cursor
		if !page.HasMore {
			break
		}
	}
	require.Equal(t, followPageCap, calls)
	require.Contains(t, page.Notice, "older new posts were skipped")
	var state pagedFollowCursor
	require.NoError(t, json.Unmarshal([]byte(cursor), &state))
	require.Equal(t, uint64(100000), state.Checkpoint)
}

func pawchiveCreatorPosts(ids ...int) string {
	rows := []map[string]any{}
	for _, id := range ids {
		rows = append(rows, map[string]any{"id": strconv.Itoa(id), "user": "42", "service": "patreon", "title": fmt.Sprint("Post ", id), "file": map[string]string{"path": "/a.jpg"}})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestFollowingPawchiveCreatorStopsAtRememberedPosts(t *testing.T) {
	source := testSource("pawchive")
	pages := map[string]string{}
	client := testClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v1/patreon/user/42", r.URL.Path)
		body, ok := pages[r.URL.Query().Get("o")]
		require.True(t, ok, "unexpected offset %s", r.URL.Query().Get("o"))
		return response(200, body), nil
	})
	require.Error(t, FollowQueryFor("pawchive", "artist", "patreon_42"))
	require.Error(t, FollowQueryFor("pawchive", "query", "creator:patreon:42"))
	require.NoError(t, FollowQueryFor("pawchive", "artist", "creator:patreon:42"))

	pages["0"] = pawchiveCreatorPosts(span(500, 451)...)
	page, err := client.FollowPage(context.Background(), source, "artist", "creator:patreon:42", "")
	require.NoError(t, err)
	require.Len(t, page.Posts, 50)
	require.False(t, page.HasMore)

	// 60 new posts span two pages; the second stops at a remembered post.
	pages["0"] = pawchiveCreatorPosts(span(560, 511)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "creator:patreon:42", page.Cursor)
	require.NoError(t, err)
	require.True(t, page.HasMore)
	pages["50"] = pawchiveCreatorPosts(span(510, 461)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "creator:patreon:42", page.Cursor)
	require.NoError(t, err)
	require.False(t, page.HasMore)
	require.Len(t, page.Posts, 10)
	require.LessOrEqual(t, len(page.Cursor), 4096)

	// Nothing new: the first post is remembered.
	pages["0"] = pawchiveCreatorPosts(span(560, 511)...)
	page, err = client.FollowPage(context.Background(), source, "artist", "creator:patreon:42", page.Cursor)
	require.NoError(t, err)
	require.Empty(t, page.Posts)
	require.False(t, page.HasMore)
}
