//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func b06Mutation(t *testing.T, rs booruBrowserRoutes, input booruFollowMutation, status int) models.BooruFollow {
	t.Helper()
	data, err := json.Marshal(input)
	require.NoError(t, err)
	r := httptest.NewRequest("POST", "/booru-browser/follows", strings.NewReader(string(data)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rs.UpdateFollow(w, r)
	require.Equal(t, status, w.Code, w.Body.String())
	var row models.BooruFollow
	if status == 200 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &row))
	}
	return row
}

func b06List(t *testing.T, rs booruBrowserRoutes) []booruFollowView {
	t.Helper()
	w := httptest.NewRecorder()
	rs.Follows(w, httptest.NewRequest("GET", "/booru-browser/follows", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	var rows []booruFollowView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	return rows
}

func b06Create(source booru.Source, kind, target string) booruFollowMutation {
	return booruFollowMutation{Action: "create", Source: source.ID, Instance: source.Instance(), Name: "My follow", Kind: kind, Target: target}
}

func TestB06FollowLifecyclePersistenceAndIndependentReview(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "follows.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	require.Empty(t, b06List(t, rs))
	query := "  {artist_a artist_b} copyright_s -lowres rating:g  "
	input := b06Create(dan, "query", query)
	row := b06Mutation(t, rs, input, 200)
	require.Equal(t, query, row.Target, "never broaden or rewrite compound search intent")
	require.Equal(t, 1, row.QueryRevision)
	require.Equal(t, 1, row.Version)
	require.True(t, row.Active)
	created := row.CreatedAt
	b06Mutation(t, rs, input, 409)
	other := b06Mutation(t, rs, b06Create(gel, "query", query), 200)
	require.NotEqual(t, row.ID, other.ID)
	require.NotEqual(t, row.Instance, other.Instance)
	// Categories are provider intent and stay distinct even for equal names.
	artist := b06Mutation(t, rs, b06Create(dan, "artist", "same_name"), 200)
	character := b06Mutation(t, rs, b06Create(dan, "character", "same_name"), 200)
	copyright := b06Mutation(t, rs, b06Create(dan, "copyright", "same_name"), 200)
	require.NotEqual(t, artist.ID, character.ID)
	require.NotEqual(t, artist.ID, copyright.ID)
	rename := booruFollowMutation{Action: "update", ID: row.ID, Version: row.Version, Name: "Renamed", Kind: row.Kind, Target: row.Target}
	row = b06Mutation(t, rs, rename, 200)
	require.Equal(t, 1, row.QueryRevision, "rename must not reset a future feed cursor")
	b06Mutation(t, rs, rename, 409)
	edit := rename
	edit.Version, edit.Target = row.Version, "artist_a copyright_s -censored"
	row = b06Mutation(t, rs, edit, 200)
	require.Equal(t, 2, row.QueryRevision)
	require.Equal(t, created, row.CreatedAt)
	// A conflicting update must not rename/change/version another follow.
	b06Mutation(t, rs, booruFollowMutation{Action: "update", ID: artist.ID, Version: artist.Version, Name: "Bad overwrite", Kind: character.Kind, Target: character.Target}, 409)
	rows := b06List(t, rs)
	for _, item := range rows {
		if item.ID == artist.ID {
			require.Equal(t, artist, item.BooruFollow)
		}
	}
	marked := b02Write(t, rs, dan, models.BooruReviewChange{PostID: "123", Ignored: b02Bool(true), Downloaded: b02Bool(true)})
	row = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: row.ID, Version: row.Version}, 200)
	require.False(t, row.Active)
	b06Mutation(t, rs, b06Create(dan, row.Kind, row.Target), 409)
	row = b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: row.ID, Version: row.Version}, 200)
	require.True(t, row.Active)
	require.Equal(t, 2, row.QueryRevision)
	require.NoError(t, db.Close())
	db = sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	rows = b06List(t, rs)
	require.Len(t, rows, 5)
	require.Equal(t, row, rows[0].BooruFollow)
	require.Equal(t, "available", rows[0].Status)
	require.Equal(t, marked[0].IgnoredAt, b02Read(t, rs, dan, booruReviewPost{ID: "123"})[0].IgnoredAt)
	require.Equal(t, marked[0].DownloadedAt, b02Read(t, rs, dan, booruReviewPost{ID: "123"})[0].DownloadedAt)

	// Concurrent edits with the same version yield one success and one conflict.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Go(func() {
			data, _ := json.Marshal(booruFollowMutation{Action: "unfollow", ID: row.ID, Version: row.Version})
			r := httptest.NewRequest("POST", "/", strings.NewReader(string(data)))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			rs.UpdateFollow(w, r)
			codes <- w.Code
		})
	}
	wg.Wait()
	close(codes)
	sum := 0
	for code := range codes {
		sum += code
	}
	require.Equal(t, 200+409, sum)
}

func TestB06FeedFiltersComposeBeforePaging(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "following.sqlite")))
	defer db.Close()
	repo := db.Repository()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	artist := b06Mutation(t, rs, b06Create(dan, "artist", "artist_a"), 200)
	series := b06Mutation(t, rs, b06Create(dan, "copyright", "series_s"), 200)
	other := b06Mutation(t, rs, b06Create(gel, "artist", "artist_a"), 200)

	seed := func(follow models.BooruFollow, ids ...string) {
		t.Helper()
		posts := make([]models.FollowingSnapshot, 0, len(ids))
		for _, id := range ids {
			posts = append(posts, models.FollowingSnapshot{PostID: id, Payload: []byte("{}")})
		}
		require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
			_, err := repo.Following.ApplyPage(ctx, follow, 0, "", false, posts)
			return err
		}))
	}
	seed(artist, "1", "2")
	seed(series, "2", "3")
	seed(other, "2")

	read := func(query url.Values, status int) ([]followingPostView, string) {
		t.Helper()
		w := httptest.NewRecorder()
		rs.FollowingFeed(w, httptest.NewRequest("GET", "/following/feed?"+query.Encode(), nil))
		require.Equal(t, status, w.Code, w.Body.String())
		var page struct {
			Items []followingPostView
			Next  string
		}
		if status == 200 {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		}
		return page.Items, page.Next
	}

	query := url.Values{"instance": {artist.Instance}, "kind": {"artist"}, "limit": {"1"}}
	first, next := read(query, 200)
	require.Len(t, first, 1)
	require.Equal(t, artist.Instance, first[0].Instance)
	require.Equal(t, "2", first[0].PostID)
	require.Len(t, first[0].Matches, 2, "retain all explanations for a shared snapshot")
	require.NotEmpty(t, next)
	query.Set("before", next)
	older, next := read(query, 200)
	require.Len(t, older, 1)
	require.Equal(t, "1", older[0].PostID)
	require.Empty(t, next)

	rows, _ := read(url.Values{"kind": {"artist"}}, 200)
	require.Len(t, rows, 3)
	require.Equal(t, other.Instance, rows[0].Instance)
	require.Equal(t, "2", rows[0].PostID)
	require.Equal(t, "2", rows[1].PostID, "equal post IDs on other sources stay distinct")

	b06Mutation(t, rs, booruFollowMutation{
		Action: "update", ID: artist.ID, Version: artist.Version,
		Name: artist.Name, Kind: "character", Target: artist.Target,
	}, 200)
	query = url.Values{"instance": {artist.Instance}, "kind": {"artist"}}
	rows, _ = read(query, 200)
	require.Empty(t, rows)
	query.Set("history", "1")
	rows, _ = read(query, 200)
	require.Len(t, rows, 2, "history filters by the saved match type")
	query.Set("follow", series.ID)
	rows, _ = read(query, 200)
	require.Empty(t, rows, "the same match must satisfy follow and type")
	read(url.Values{"kind": {"unknown"}}, 400)
	read(url.Values{"instance": {"not-an-instance"}}, 400)
}

func TestB06SourceBindingAndCredentials(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "follows.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	source := booru.Defaults()[0]
	source.Login, source.APIKey = "private-account", "private-secret"
	settings := filepath.Join(rs.configPath, "booru-browser.json")
	save := func(sources ...booru.Source) {
		t.Helper()
		data, err := json.Marshal(sources)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(settings, data, 0600))
	}
	save(source)
	row := b06Mutation(t, rs, b06Create(source, "artist", "artist_a"), 200)
	source.APIKey = "rotated-secret"
	save(source)
	rows := b06List(t, rs)
	require.Equal(t, "available", rows[0].Status)
	data, err := json.Marshal(rows)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-account")
	require.NotContains(t, string(data), "private-secret")
	require.NotContains(t, string(data), "rotated-secret")
	changed := source
	changed.URL = "https://different.example.org"
	save(changed)
	require.Equal(t, "source-changed", b06List(t, rs)[0].Status)
	b06Mutation(t, rs, b06Create(source, "artist", "artist_b"), 409)
	row = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: row.ID, Version: row.Version}, 200)
	b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: row.ID, Version: row.Version}, 409)
	alias := source
	alias.ID = "renamed-danbooru"
	save(booru.Source{ID: source.ID, Disabled: true}, alias)
	rows = b06List(t, rs)
	require.Equal(t, "available", rows[0].Status)
	require.Equal(t, alias.ID, rows[0].ResolvedSource)
	b06Mutation(t, rs, b06Create(alias, "artist", "artist_a"), 409)
	row = b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: row.ID, Version: row.Version}, 200)
	save(booru.Source{ID: source.ID, Disabled: true})
	require.Equal(t, "source-unavailable", b06List(t, rs)[0].Status)
	// A broken config must not trap an active follow.
	require.NoError(t, os.WriteFile(settings, []byte(`broken`), 0600))
	// Simulate a reloaded client: recover the ID/version from storage instead of
	// relying on the mutation response retained before configuration broke.
	rows = b06List(t, rs)
	require.Equal(t, row.ID, rows[0].ID)
	require.Equal(t, row.Version, rows[0].Version)
	require.Equal(t, "configuration-error", rows[0].Status)
	require.Empty(t, rows[0].ResolvedSource)
	row = rows[0].BooruFollow
	row = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: row.ID, Version: row.Version}, 200)
	require.False(t, row.Active)
	require.NoError(t, os.Remove(settings))
	// An unreadable source path also leaves the persisted list accessible.
	require.NoError(t, os.Mkdir(settings, 0700))
	require.Equal(t, "configuration-error", b06List(t, rs)[0].Status)
	require.NoError(t, os.Remove(settings))
	for _, source := range booru.Defaults() {
		if !supportsBooruFollow(source) {
			b06Mutation(t, rs, b06Create(source, "query", "anything"), 400)
		}
	}
	// Pawchive follows one creator account, never a free query.
	for _, source := range booru.Defaults() {
		if source.Engine == "pawchive" {
			b06Mutation(t, rs, b06Create(source, "query", "anything"), 400)
			b06Mutation(t, rs, b06Create(source, "artist", "creator:patreon:42"), 200)
		}
	}
	// Targets a feed check would reject are refused when saved, not later.
	b06Mutation(t, rs, b06Create(booru.Defaults()[0], "artist", "re:zero"), 400)
	b06Mutation(t, rs, b06Create(booru.Defaults()[0], "artist", "two words"), 400)
}

func TestB06FollowCapacityAndStoreGuards(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "capacity.sqlite")))
	defer db.Close()
	repo := db.Repository()
	source := booru.Defaults()[0]
	provider := models.BooruReviewProvider{Instance: source.Instance(), SourceID: source.ID, Engine: source.Engine, URL: source.URL}
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		for i := range models.MaxBooruFollows {
			_, err := repo.BooruFollow.Create(ctx, provider, models.BooruFollowDefinition{Name: "Follow", Kind: "artist", Target: fmt.Sprintf("artist_%d", i)})
			if err != nil {
				return err
			}
		}
		return nil
	}))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	rows := b06List(t, rs)
	require.Len(t, rows, models.MaxBooruFollows)
	b06Mutation(t, rs, b06Create(source, "artist", "one_more"), 409)
	row := b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: rows[0].ID, Version: rows[0].Version}, 200)
	b06Mutation(t, rs, booruFollowMutation{Action: "refollow", ID: row.ID, Version: row.Version}, 200)
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := repo.BooruFollow.Update(ctx, rows[0].ID, 1, nil, nil)
		require.ErrorIs(t, err, models.ErrBooruFollowInvalid)
		_, err = repo.BooruFollow.Create(ctx, provider, models.BooruFollowDefinition{})
		require.ErrorIs(t, err, models.ErrBooruFollowInvalid)
		return nil
	}))
}

func TestB06MigrationUpgrades97AndPreservesCatalogueAndReview(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "upgrade.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	repo := db.Repository()
	image := models.NewImage()
	image.Title = "Preserved Image"
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		return repo.Image.Create(ctx, &models.CreateImageInput{Image: &image})
	}))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	source := booru.Defaults()[0]
	b02Write(t, rs, source, models.BooruReviewChange{PostID: "123", Ignored: b02Bool(true)})
	require.NoError(t, db.Close())
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`DROP TABLE character_dedup_exclusions; DROP TABLE provider_identity_rejections; DROP TABLE provider_identities; DROP TABLE following_matches; DROP TABLE following_checkpoints; DROP TABLE following_seen; DROP TABLE following_posts; DROP TABLE booru_follows; UPDATE schema_migrations SET version=97,dirty=0; ALTER TABLE tags DROP COLUMN hide_thumbnail; ALTER TABLE tags DROP COLUMN ignored; ALTER TABLE performers DROP COLUMN ignored; ALTER TABLE studios DROP COLUMN ignored; ALTER TABLE copyrights DROP COLUMN ignored; DROP TRIGGER IF EXISTS image_visual_embedding_cleanup; DROP INDEX files_missing_since_index; ALTER TABLE files DROP COLUMN missing_since; DROP INDEX folders_missing_since_index; ALTER TABLE folders DROP COLUMN missing_since`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(105), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 98))
	require.NoError(t, migrator.RunMigration(context.Background(), 99))
	require.NoError(t, migrator.RunMigration(context.Background(), 100))
	require.NoError(t, migrator.RunMigration(context.Background(), 101))
	require.NoError(t, migrator.RunMigration(context.Background(), 102))
	require.NoError(t, migrator.RunMigration(context.Background(), 103))
	require.NoError(t, migrator.RunMigration(context.Background(), 104))
	require.NoError(t, migrator.RunMigration(context.Background(), 105))
	migrator.Close()
	require.NoError(t, db.Open(path))
	defer db.Close()
	repo = db.Repository()
	rs.repository = repo
	require.Empty(t, b06List(t, rs))
	b06Mutation(t, rs, b06Create(source, "copyright", "series_s"), 200)
	require.NotEmpty(t, b02Read(t, rs, source, booruReviewPost{ID: "123"})[0].IgnoredAt)
	require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
		found, err := repo.Image.Find(ctx, image.ID)
		require.NoError(t, err)
		require.Equal(t, image.Title, found.Title)
		return nil
	}))
}

func TestB06FollowShowsConnectedEntity(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "connection.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	dan := booru.Defaults()[0]
	follow := b06Mutation(t, rs, b06Create(dan, "artist", "artist_a"), 200)
	query := b06Mutation(t, rs, b06Create(dan, "query", "artist_a rating:g"), 200)
	require.Nil(t, b06List(t, rs)[0].Connection, "no stored source name yet")

	var observed models.ProviderIdentity
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.Observe(ctx, []models.IdentityObservation{identityObservation(dan, "artist", "artist_a", "", "", 0)})
		if err == nil {
			observed = rows[0]
		}
		return err
	}))
	linked := identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: observed.ID, Version: observed.Version}}, "Local Artist", 200)[0]
	for _, view := range b06List(t, rs) {
		switch view.ID {
		case follow.ID:
			require.NotNil(t, view.Connection)
			require.Equal(t, observed.ID, view.Connection.IdentityID)
			require.Equal(t, linked.EntityID, view.Connection.EntityID)
			require.Equal(t, "Local Artist", view.Connection.EntityName)
		case query.ID:
			require.Nil(t, view.Connection, "queries are not source names")
		}
	}
}

func TestB06MarkSeenBatchAndSeenEviction(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "seen.sqlite")))
	defer db.Close()
	repo := db.Repository()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	dan := booru.Defaults()[0]
	a := b06Mutation(t, rs, b06Create(dan, "artist", "artist_a"), 200)
	b := b06Mutation(t, rs, b06Create(dan, "artist", "artist_b"), 200)
	seed := func(follow models.BooruFollow, version int, ids ...string) {
		t.Helper()
		posts := make([]models.FollowingSnapshot, 0, len(ids))
		for _, id := range ids {
			posts = append(posts, models.FollowingSnapshot{PostID: id, Payload: []byte("{}")})
		}
		require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
			_, err := repo.Following.ApplyPage(ctx, follow, version, "", false, posts)
			return err
		}))
	}
	seed(a, 0, "1", "2")
	seed(b, 0, "3")
	post := func(body string, status int) int {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/following/seen-batch", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		rs.MarkFollowingSeen(w, r)
		require.Equal(t, status, w.Code, w.Body.String())
		var out struct{ Changed int }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.Changed
	}
	unseen := func() (map[string]int, int) {
		t.Helper()
		var counts map[string]int
		var total int
		require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
			var err error
			counts, total, err = repo.Following.UnseenCounts(ctx)
			return err
		}))
		return counts, total
	}
	feed := func(query string) int {
		t.Helper()
		w := httptest.NewRecorder()
		rs.FollowingFeed(w, httptest.NewRequest("GET", "/following/feed?"+query, nil))
		require.Equal(t, 200, w.Code, w.Body.String())
		var page struct{ Items []followingPostView }
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		return len(page.Items)
	}
	require.Equal(t, 2, feed("follow="+a.ID))
	require.Equal(t, 3, feed("follow="+a.ID+"&follow="+b.ID), "one Artist followed on several sites")
	post(`{"seen":true}`, 400)
	post(`{"seen":false,"upTo":"9"}`, 400)
	post(`{"seen":true,"sequences":["1"],"upTo":"9"}`, 400)
	// Mark all for one follow, bounded by what the page showed (sequence 1).
	require.Equal(t, 1, post(`{"seen":true,"followIDs":["`+a.ID+`"],"upTo":"1"}`, 200))
	counts, total := unseen()
	require.Equal(t, 1, counts[a.ID], "a post saved after the page loaded stays unseen")
	require.Equal(t, 2, total)
	// Selection: mark seen, then unseen again; repeats change nothing.
	require.Equal(t, 2, post(`{"seen":true,"sequences":["2","3"]}`, 200))
	require.Equal(t, 0, post(`{"seen":true,"sequences":["2","3"]}`, 200))
	_, total = unseen()
	require.Zero(t, total)
	require.Equal(t, 1, post(`{"seen":false,"sequences":["3"]}`, 200))
	_, total = unseen()
	require.Equal(t, 1, total)
	// Single-post decisions see the batch's new version.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/following/seen", strings.NewReader(`{"sequence":"3","version":0,"seen":true}`))
	r.Header.Set("Content-Type", "application/json")
	rs.SetFollowingSeen(w, r)
	require.Equal(t, 409, w.Code, "a stale per-post version must not overwrite a batch decision")
	// Posts shown in the Booru browser, named by site ID: only saved ones
	// change, and unknown IDs are ignored.
	require.Equal(t, 1, post(`{"seen":true,"instance":"`+a.Instance+`","postIDs":["3","not-saved"]}`, 200))
	require.Equal(t, 0, post(`{"seen":true,"instance":"other","postIDs":["1"]}`, 200))
	post(`{"seen":true,"postIDs":["1"]}`, 400)
	post(`{"seen":true,"instance":"`+a.Instance+`","postIDs":["1"],"sequences":["1"]}`, 400)
	require.Equal(t, 1, post(`{"seen":false,"sequences":["3"]}`, 200))
	// All follows, everything shown.
	require.Equal(t, 1, post(`{"seen":true,"upTo":"3"}`, 200))
	_, total = unseen()
	require.Zero(t, total)

	// A full cache replaces the oldest seen snapshot instead of failing.
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		for i := 3; i < models.MaxFollowingPosts; i += 50 {
			posts := []models.FollowingSnapshot{}
			for j := i; j < i+50 && j < models.MaxFollowingPosts; j++ {
				posts = append(posts, models.FollowingSnapshot{PostID: fmt.Sprintf("fill-%d", j), Payload: []byte("{}")})
			}
			state, err := repo.Following.Checkpoint(ctx, b.ID, b.QueryRevision)
			if err != nil {
				return err
			}
			if _, err := repo.Following.ApplyPage(ctx, b, state.Version, "", false, posts); err != nil {
				return err
			}
		}
		return nil
	}))
	version := func() int {
		var state models.FollowingCheckpoint
		require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
			var err error
			state, err = repo.Following.Checkpoint(ctx, b.ID, b.QueryRevision)
			return err
		}))
		return state.Version
	}
	seed(b, version(), "new-1")
	_, total = unseen()
	require.Equal(t, models.MaxFollowingPosts-3+1, total)
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		_, _, err := repo.Following.SetSeen(ctx, 1, 1, false)
		require.ErrorIs(t, err, models.ErrBooruFollowConflict, "the oldest seen snapshot made room")
		return nil
	}))
	// Once only unseen posts remain, the limit applies again.
	for _, id := range []string{"extra-1", "extra-2"} {
		seed(b, version(), id)
	}
	current := version()
	err := repo.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := repo.Following.ApplyPage(ctx, b, current, "", false, []models.FollowingSnapshot{{PostID: "overflow", Payload: []byte("{}")}})
		return err
	})
	require.ErrorIs(t, err, models.ErrFollowingLimit)
}
