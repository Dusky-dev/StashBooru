//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

type feedFixtureClient struct {
	booruBrowserClient
	page func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error)
}

func (c feedFixtureClient) FollowPage(ctx context.Context, s booru.Source, kind, target, cursor string) (booru.FollowPage, error) {
	return c.page(ctx, s, kind, target, cursor)
}

func feedRequest(t *testing.T, handler http.HandlerFunc, payload any, status int) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	r := httptest.NewRequest("POST", "/following", strings.NewReader(string(data)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)
	require.Equal(t, status, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	return w.Body.Bytes()
}

func feedRead(t *testing.T, rs booruBrowserRoutes, query string) []followingPostView {
	t.Helper()
	w := httptest.NewRecorder()
	rs.FollowingFeed(w, httptest.NewRequest("GET", "/following/feed"+query, nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "original-secret.jpg")
	var page struct{ Items []followingPostView }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	return page.Items
}

func feedSync(t *testing.T, rs booruBrowserRoutes, f models.BooruFollow, status int) {
	t.Helper()
	feedRequest(t, rs.SyncFollow, map[string]any{"id": f.ID, "version": f.Version}, status)
}

func feedPosts(ids ...string) []booru.Post {
	source := booru.Defaults()[0]
	posts := []booru.Post{}
	for _, id := range ids {
		posts = append(posts, booru.Post{ID: id, PostURL: source.URL + "/posts/" + id, PreviewURL: source.URL + "/preview.jpg", OriginalURL: source.URL + "/original-secret.jpg", Tags: []booru.Tag{{Name: "artist_a", Category: "artist"}}, CreatedAt: "2026-10-20T00:00:00Z"})
	}
	return posts
}

func TestFollowingFeedPersistenceDedupSeenHistoryAndPrune(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "feed.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	a := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	b := b06Mutation(t, rs, b06Create(source, "copyright", "series_s"), 200)
	posts, cursor, more := feedPosts("10", "9"), "pending", true
	rs.client = feedFixtureClient{page: func(_ context.Context, _ booru.Source, _, _ string, current string) (booru.FollowPage, error) {
		if current != "" {
			require.Equal(t, "pending", current)
		}
		return booru.FollowPage{Posts: posts, Cursor: cursor, HasMore: more}, nil
	}}
	feedSync(t, rs, a, 200)
	feedSync(t, rs, b, 200)
	rows := feedRead(t, rs, "")
	require.Len(t, rows, 2, "matching two follows must not duplicate posts")
	require.Equal(t, "10", rows[0].PostID)
	require.Len(t, rows[0].Matches, 2)
	require.Contains(t, rows[0].Content.Thumbnail, "booru-browser/media?key=")
	require.Contains(t, rows[0].Content.BrowseURL, "id%3A10")
	seen := map[string]any{"sequence": strconv.FormatInt(rows[0].Sequence, 10), "version": 0, "seen": true}
	feedRequest(t, rs.SetFollowingSeen, seen, 200)
	feedRequest(t, rs.SetFollowingSeen, seen, 409)
	require.Len(t, feedRead(t, rs, "?unseen=1"), 1)
	// Seen/unseen is explicit and independent of native ignore/download marks.
	b02Write(t, rs, source, models.BooruReviewChange{PostID: "9", Ignored: b02Bool(true)}, models.BooruReviewChange{PostID: "10", Downloaded: b02Bool(true)})
	require.Len(t, feedRead(t, rs, ""), 1)
	require.Len(t, feedRead(t, rs, "?ignored=1"), 2)
	require.Len(t, feedRead(t, rs, "?ignored=1&downloaded=hide"), 1)
	posts, cursor, more = feedPosts("11"), "done", false
	feedSync(t, rs, a, 200)
	require.NoError(t, db.Close())
	db = sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	rows = feedRead(t, rs, "?ignored=1")
	require.Len(t, rows, 3)
	require.Equal(t, "11", rows[0].PostID)
	require.NotEmpty(t, rows[1].SeenAt)
	require.Equal(t, 1, rows[1].SeenVersion)
	follows := b06List(t, rs)
	require.False(t, follows[0].Feed.HasMore)
	require.True(t, follows[1].Feed.HasMore, "each follow has an independent durable checkpoint")
	// Query changes start a fresh revision without silently using the old cursor.
	a = b06Mutation(t, rs, booruFollowMutation{Action: "update", ID: a.ID, Version: a.Version, Name: a.Name, Kind: "artist", Target: "artist_b"}, 200)
	b = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: b.ID, Version: b.Version}, 200)
	require.Empty(t, feedRead(t, rs, "?ignored=1"))
	require.Len(t, feedRead(t, rs, "?ignored=1&history=1"), 3)
	posts, cursor = feedPosts("10"), "new-revision"
	feedSync(t, rs, a, 200)
	rows = feedRead(t, rs, "")
	require.Len(t, rows, 1)
	require.NotEmpty(t, rows[0].SeenAt, "a new query revision must not reset an explicit seen decision")
	feedRequest(t, rs.PruneFollowingSeen, map[string]any{"action": "remove-seen-snapshots"}, 200)
	require.Empty(t, feedRead(t, rs, ""))
	// A different follow can rediscover a pruned snapshot without losing its seen tombstone.
	b = b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: b.ID, Version: b.Version}, 200)
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		return booru.FollowPage{Posts: feedPosts("10"), Cursor: "rediscovered"}, nil
	}}
	feedSync(t, rs, b, 200)
	rows = feedRead(t, rs, "")
	require.Len(t, rows, 1)
	require.NotEmpty(t, rows[0].SeenAt)
	require.Empty(t, feedRead(t, rs, "?unseen=1"))
	feedRequest(t, rs.SetFollowingSeen, map[string]any{"sequence": strconv.FormatInt(rows[0].Sequence, 10), "version": 1, "seen": false}, 200)
	require.Len(t, feedRead(t, rs, "?unseen=1"), 1)
	require.NotEmpty(t, b02Read(t, rs, source, booruReviewPost{ID: "10"})[0].DownloadedAt)
}

func TestFollowingFailedStaleAndConcurrentPagesAreAtomic(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "feed.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	follow := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		return booru.FollowPage{}, errors.New("private transport detail")
	}}
	feedSync(t, rs, follow, 500)
	require.Empty(t, feedRead(t, rs, ""))
	require.Empty(t, b06List(t, rs)[0].Feed.CheckedAt)
	require.NotEmpty(t, b06List(t, rs)[0].Feed.LastError)
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		follow = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: follow.ID, Version: follow.Version}, 200)
		return booru.FollowPage{Posts: feedPosts("10"), Cursor: "must-not-commit"}, nil
	}}
	feedSync(t, rs, follow, 409)
	require.Empty(t, feedRead(t, rs, "?history=1"))
	follow = b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: follow.ID, Version: follow.Version}, 200)
	// Two in-flight requests read one checkpoint; only one can commit it.
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		arrived <- struct{}{}
		<-release
		return booru.FollowPage{Posts: feedPosts("10"), Cursor: "one-commit"}, nil
	}}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Go(func() {
			data, _ := json.Marshal(map[string]any{"id": follow.ID, "version": follow.Version})
			r := httptest.NewRequest("POST", "/following/sync", strings.NewReader(string(data)))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			rs.SyncFollow(w, r)
			codes <- w.Code
		})
	}
	<-arrived
	<-arrived
	close(release)
	wg.Wait()
	close(codes)
	sum := 0
	for code := range codes {
		sum += code
	}
	require.Equal(t, 200+409, sum)
	require.Len(t, feedRead(t, rs, ""), 1)
	require.Equal(t, 2, b06List(t, rs)[0].Feed.Version)
	require.Empty(t, b06List(t, rs)[0].Feed.LastError)
}

func TestFollowingSourceRotationAndBrokenConfigurationRecovery(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "feed.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	follow := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		return booru.FollowPage{Posts: feedPosts("10"), Cursor: "saved"}, nil
	}}
	feedSync(t, rs, follow, 200)
	settings := filepath.Join(rs.configPath, "booru-browser.json")
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		source.Login, source.APIKey = "private-login", "new-secret"
		data, err := json.Marshal([]booru.Source{source})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(settings, data, 0600))
		return booru.FollowPage{Posts: feedPosts("11"), Cursor: "must-not-save"}, nil
	}}
	feedSync(t, rs, follow, 409)
	require.Len(t, feedRead(t, rs, ""), 1)
	require.Equal(t, 2, b06List(t, rs)[0].Feed.Version)
	require.NotEmpty(t, b06List(t, rs)[0].Feed.LastError)
	require.NoError(t, os.WriteFile(settings, []byte("broken"), 0600))
	rows := feedRead(t, rs, "")
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].Content.Thumbnail)
	feedRequest(t, rs.SetFollowingSeen, map[string]any{"sequence": strconv.FormatInt(rows[0].Sequence, 10), "version": 0, "seen": true}, 200)
	require.Empty(t, feedRead(t, rs, "?unseen=1"))
}

func TestFollowingCapacityFailureRollsBackAllPageWork(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "capacity.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	follow := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<4999)
 INSERT INTO following_posts(instance,post_id,payload,retrieved_at) SELECT ?,cast(x AS TEXT),'{}','fixture' FROM n`, source.Instance())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		return booru.FollowPage{Posts: feedPosts("6001", "6000"), Cursor: "must-rollback"}, nil
	}}
	feedSync(t, rs, follow, 409)
	require.Empty(t, b06List(t, rs)[0].Feed.CheckedAt)
	require.NotEmpty(t, b06List(t, rs)[0].Feed.LastError)
	require.Empty(t, feedRead(t, rs, "?history=1"), "the first inserted item/match must roll back too")
}

func TestFollowingMigration98PreservesDefinitions(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "upgrade.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	follow := b06Mutation(t, rs, b06Create(booru.Defaults()[0], "copyright", "series_s"), 200)
	require.NoError(t, db.Close())
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`DROP TABLE following_matches; DROP TABLE following_checkpoints; DROP TABLE following_seen; DROP TABLE following_posts; UPDATE schema_migrations SET version=98,dirty=0`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(99), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 99))
	migrator.Close()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	require.Equal(t, follow, b06List(t, rs)[0].BooruFollow)
	require.Empty(t, feedRead(t, rs, ""))
}

func TestFollowingPaginationMatchBoundsAndCancelledChecks(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "feed.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	var last *models.BooruFollow
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		provider := models.BooruReviewProvider{SourceID: source.ID, Engine: source.Engine, URL: source.URL, Instance: source.Instance()}
		for i := range 55 {
			var err error
			last, err = rs.repository.BooruFollow.Create(ctx, provider, models.BooruFollowDefinition{Name: "Follow", Kind: "artist", Target: "artist_" + strconv.Itoa(i)})
			if err != nil {
				return err
			}
			_, err = rs.repository.Following.ApplyPage(ctx, *last, 0, "opaque-provider-cursor", false, []models.FollowingSnapshot{{PostID: "10", Payload: []byte(`{}`)}})
			if err != nil {
				return err
			}
		}
		_, err := rs.repository.Following.ApplyPage(ctx, *last, 1, "next", false, []models.FollowingSnapshot{{PostID: "11", Payload: []byte(`{}`)}})
		return err
	}))
	rows := feedRead(t, rs, "?follow="+last.ID)
	require.Len(t, rows, 2)
	require.Len(t, rows[1].Matches, 50)
	require.True(t, rows[1].MatchesTruncated)
	require.Equal(t, last.ID, rows[1].Matches[0].FollowID, "the selected follow must remain visible in bounded explanations")
	w := httptest.NewRecorder()
	rs.FollowingFeed(w, httptest.NewRequest("GET", "/following/feed?limit=1", nil))
	var page struct {
		Items []followingPostView
		Next  string
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	require.NotEmpty(t, page.Next)
	older := feedRead(t, rs, "?before="+page.Next)
	require.Len(t, older, 1)
	require.NotEqual(t, page.Items[0].Sequence, older[0].Sequence)
	for _, query := range []string{"?limit=0", "?limit=51", "?before=-1", "?unseen=wrong", "?downloaded=1"} {
		w := httptest.NewRecorder()
		rs.FollowingFeed(w, httptest.NewRequest("GET", "/following/feed"+query, nil))
		require.Equal(t, 400, w.Code, query)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rs.client = feedFixtureClient{page: func(ctx context.Context, _ booru.Source, _, _, _ string) (booru.FollowPage, error) {
		cancel()
		return booru.FollowPage{}, ctx.Err()
	}}
	data, err := json.Marshal(map[string]any{"id": last.ID, "version": last.Version})
	require.NoError(t, err)
	r := httptest.NewRequest("POST", "/following/sync", strings.NewReader(string(data))).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	rs.SyncFollow(w, r)
	require.NotEqual(t, 200, w.Code)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		state, err := rs.repository.Following.Checkpoint(ctx, last.ID, last.QueryRevision)
		require.Equal(t, 2, state.Version)
		require.Equal(t, "next", state.Cursor)
		return err
	}))
}

func TestFollowingSnapshotsOmitSignedMediaAndTokensAcrossRestart(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "feed.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	follow := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	posts := feedPosts("10", "9")
	signed := &posts[0]
	signed.PreviewURL += "?X-Amz-Credential=private-credential&X-Amz-Signature=secret-signature"
	signed.SampleURL = source.URL + "/sample.jpg?Policy=private-policy&Signature=secret-signature"
	signed.OriginalURL += "?X-Goog-Credential=private-credential&X-Goog-Signature=secret-signature"
	signed.MediaKey, signed.DirectPreviewURL, signed.DirectSampleURL = "ephemeral-grant", signed.PreviewURL, signed.SampleURL
	signed.Attachments = []booru.Post{{
		ID:               "10",
		PreviewURL:       signed.PreviewURL,
		SampleURL:        signed.SampleURL,
		MediaKey:         "ephemeral-grant",
		DirectPreviewURL: signed.PreviewURL,
	}}
	// The existing transport accepts these provider-approved URLs for transient
	// use. Following must enforce its stricter durable-storage policy itself.
	require.Equal(t, signed.PreviewURL, source.ApprovedMediaURL(signed.PreviewURL))
	rs.client = feedFixtureClient{page: func(context.Context, booru.Source, string, string, string) (booru.FollowPage, error) {
		return booru.FollowPage{Posts: posts, Cursor: "saved"}, nil
	}}
	feedSync(t, rs, follow, 200)
	require.NoError(t, db.Close())
	db = sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	var snapshots []models.FollowingPost
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		var err error
		snapshots, err = rs.repository.Following.List(ctx, models.FollowingFilter{Limit: 25})
		return err
	}))
	require.Len(t, snapshots, 2)
	require.Equal(t, "10", snapshots[0].PostID)
	for _, forbidden := range []string{"private-credential", "secret-signature", "private-policy", "ephemeral-grant", "X-Amz", "X-Goog"} {
		require.NotContains(t, string(snapshots[0].Payload), forbidden)
	}
	var stored followingStoredPost
	require.NoError(t, json.Unmarshal(snapshots[0].Payload, &stored))
	require.Empty(t, stored.Original)
	require.Empty(t, stored.Post.PreviewURL)
	require.Empty(t, stored.Post.SampleURL)
	require.False(t, stored.Post.DownloadAvailable)
	require.Len(t, stored.Post.Attachments, 1)
	require.Empty(t, stored.Post.Attachments[0].PreviewURL)
	rows := feedRead(t, rs, "")
	require.Empty(t, rows[0].Content.Thumbnail)
	require.NotEmpty(t, rows[0].Content.URL)
	require.NotEmpty(t, rows[0].Content.Tags)
	require.NotEmpty(t, rows[0].Content.BrowseURL, "fresh metadata stays available through the browser")
	require.Contains(t, rows[1].Content.Thumbnail, "booru-browser/media?key=")
	require.NoError(t, json.Unmarshal(snapshots[1].Payload, &stored))
	require.Equal(t, source.URL+"/original-secret.jpg", stored.Original)
}

func TestFollowingFeedPrefersSnapshotAliasAndCurrentCredentials(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "feed.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	builtin := booru.Defaults()[0]
	alias := builtin
	alias.ID, alias.Name, alias.Login, alias.APIKey = "private-dan", "Private Danbooru", "account", "first-secret"
	settings := filepath.Join(rs.configPath, "booru-browser.json")
	saveAlias := func() {
		data, err := json.Marshal([]booru.Source{alias})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(settings, data, 0600))
	}
	saveAlias()
	follow := b06Mutation(t, rs, b06Create(alias, "artist", "artist_a"), 200)
	rs.client = feedFixtureClient{page: func(_ context.Context, source booru.Source, _, _, _ string) (booru.FollowPage, error) {
		require.Equal(t, alias.ID, source.ID)
		posts := feedPosts("10")
		posts[0].SourceID = builtin.ID // Sync stamps the server-selected alias.
		return booru.FollowPage{Posts: posts, Cursor: "saved"}, nil
	}}
	feedSync(t, rs, follow, 200)
	assertAlias := func(id, key string) {
		rows := feedRead(t, rs, "")
		require.Len(t, rows, 1)
		browse, err := url.Parse(rows[0].Content.BrowseURL)
		require.NoError(t, err)
		require.Equal(t, id, browse.Query().Get("source"))
		preview, err := url.Parse(rows[0].Content.Thumbnail)
		require.NoError(t, err)
		source, _, ok := rs.mediaSnapshot(preview.Query().Get("key"))
		require.True(t, ok)
		require.Equal(t, id, source.ID)
		require.Equal(t, key, source.APIKey)
		require.NotContains(t, string(rows[0].Payload), "first-secret")
		require.NotContains(t, string(rows[0].Payload), "rotated-secret")
	}
	assertAlias(alias.ID, alias.APIKey)
	alias.APIKey = "rotated-secret"
	saveAlias()
	assertAlias(alias.ID, alias.APIKey)
	alias.URL = "https://other.example.org"
	saveAlias()
	assertAlias(builtin.ID, "")
	alias = booru.Source{ID: "private-dan", Disabled: true}
	saveAlias()
	assertAlias(builtin.ID, "")
}
