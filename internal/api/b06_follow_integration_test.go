//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
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
	row = b06Mutation(t, rs, booruFollowMutation{Action: "unfollow", ID: row.ID, Version: row.Version}, 200)
	require.False(t, row.Active)
	require.NoError(t, os.Remove(settings))
	for _, source := range booru.Defaults() {
		if !supportsBooruFollow(source) {
			b06Mutation(t, rs, b06Create(source, "query", "anything"), 400)
		}
	}
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
	_, err = conn.Exec(`DROP TABLE booru_follows; UPDATE schema_migrations SET version=97,dirty=0`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(98), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 98))
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
