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

func followRecords(high, low int) string {
	rows := []map[string]any{}
	for i := high; i >= low; i-- {
		rows = append(rows, map[string]any{"id": i, "tag_string_artist": "artist_a", "preview_file_url": "/preview.jpg"})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestFollowingKeysetBootstrapNewInsertionAndResume(t *testing.T) {
	body, expectedPage := followRecords(100, 51), "b9223372036854775807"
	query := "{artist_a artist_b} copyright_s -lowres"
	client := testClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, expectedPage, r.URL.Query().Get("page"))
		require.Equal(t, query, r.URL.Query().Get("tags"), "compound predicate must be untouched")
		require.Equal(t, "50", r.URL.Query().Get("limit"))
		return response(200, body), nil
	})
	page, err := client.FollowPage(context.Background(), testSource("danbooru"), "query", query, "")
	require.NoError(t, err)
	require.Len(t, page.Posts, 50)
	require.False(t, page.HasMore, "first check is a declared recent window")
	var state followCursor
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(100), state.Checkpoint)
	body = followRecords(175, 126)
	page, err = client.FollowPage(context.Background(), testSource("danbooru"), "query", query, page.Cursor)
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(100), state.Checkpoint, "pending pages cannot advance the completed checkpoint")
	require.Equal(t, uint64(175), state.High)
	expectedPage, body = "b126", followRecords(125, 76)
	page, err = client.FollowPage(context.Background(), testSource("danbooru"), "query", query, page.Cursor)
	require.NoError(t, err)
	require.False(t, page.HasMore)
	require.Len(t, page.Posts, 25)
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(175), state.Checkpoint)
	// Posts inserted above the run's frozen high-water ID are fetched next check.
	expectedPage, body = "b9223372036854775807", followRecords(180, 131)
	page, err = client.FollowPage(context.Background(), testSource("danbooru"), "query", query, page.Cursor)
	require.NoError(t, err)
	require.Len(t, page.Posts, 5)
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, uint64(180), state.Checkpoint)
}

func TestFollowingRejectsCursorViolationsAndControlQueries(t *testing.T) {
	client := testClient(func(r *http.Request) (*http.Response, error) { return response(200, `[{"id":11},{"id":12}]`), nil })
	_, err := client.FollowPage(context.Background(), testSource("danbooru"), "artist", "artist_a", "")
	require.Error(t, err)
	for _, target := range []string{"", "x order:score", "x LIMIT:1", "{x random:1}", "x -page:3"} {
		require.Error(t, FollowQuery("query", target), target)
	}
	for _, target := range []string{"artist_a other", "-artist", "~artist", "artist*", "rating:g"} {
		require.Error(t, FollowQuery("artist", target), target)
	}
	for _, engine := range []string{"gelbooru", "e621", "pixiv"} {
		_, err := client.FollowPage(context.Background(), testSource(engine), "artist", "a", "")
		require.Error(t, err)
	}
	for _, cursor := range []string{"broken", `{"checkpoint":"9223372036854775807"}`, `{"initialized":true,"checkpoint":"100","high":"150","before":"99"}`} {
		_, err := client.FollowPage(context.Background(), testSource("danbooru"), "artist", "a", cursor)
		require.Error(t, err)
	}
}

func TestFollowingEmptyBootstrapAndFailedPageKeepReplayableCursor(t *testing.T) {
	body, status := `[]`, 200
	client := testClient(func(r *http.Request) (*http.Response, error) { return response(status, body), nil })
	page, err := client.FollowPage(context.Background(), testSource("danbooru"), "artist", "artist_a", "")
	require.NoError(t, err)
	first := page.Cursor
	body, status = followRecords(60, 11), 503
	_, err = client.FollowPage(context.Background(), testSource("danbooru"), "artist", "artist_a", first)
	require.Error(t, err)
	status = 200
	page, err = client.FollowPage(context.Background(), testSource("danbooru"), "artist", "artist_a", first)
	require.NoError(t, err)
	require.True(t, page.HasMore, "an initially empty source still exhausts all later new IDs")
	var state followCursor
	require.NoError(t, json.Unmarshal([]byte(page.Cursor), &state))
	require.Equal(t, "11", strconv.FormatUint(state.Before, 10))
	body = fmt.Sprintf(`[{"id":%d}]`, state.Before)
	_, err = client.FollowPage(context.Background(), testSource("danbooru"), "artist", "artist_a", page.Cursor)
	require.Error(t, err, "repeated boundary IDs must not create a non-progressing cursor")
}
