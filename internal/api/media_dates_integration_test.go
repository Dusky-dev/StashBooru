//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func TestFillMissingMediaDatesFromDownloadRecords(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "dates.sqlite")))
	defer db.Close()
	repo := db.Repository()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	kept, _ := models.ParseDate("2001-02-03")
	var undated, dated, renamed models.Image
	var video models.Scene
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		for _, item := range []struct {
			image *models.Image
			url   string
			date  *models.Date
		}{{&undated, "https://danbooru.example/posts/1", nil}, {&dated, "https://danbooru.example/posts/2", &kept}, {&renamed, "https://danbooru.example/posts/other", nil}} {
			*item.image = models.NewImage()
			item.image.URLs = models.NewRelatedStrings([]string{item.url})
			item.image.Date = item.date
			if err := repo.Image.Create(ctx, &models.CreateImageInput{Image: item.image}); err != nil {
				return err
			}
		}
		video = models.NewScene()
		video.URLs = models.NewRelatedStrings([]string{"https://gelbooru.example/post/9"})
		return repo.Scene.Create(ctx, &video, nil)
	}))
	dir := filepath.Join(rs.configPath, "booru-download-history")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	write := func(name, kind string, id int, url, created string) {
		data, err := json.Marshal(booruDownloadReceipt{
			Post:    booru.Post{ID: name, PostURL: url, CreatedAt: created},
			Library: &booruLibraryResult{Kind: kind, ID: strconv.Itoa(id)},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600))
	}
	write("a", "image", undated.ID, "https://danbooru.example/posts/1", "2019-07-14T10:00:00.000-04:00")
	write("b", "image", dated.ID, "https://danbooru.example/posts/2", "2019-07-14T10:00:00Z")
	write("c", "image", renamed.ID, "https://danbooru.example/posts/3", "2019-07-14T10:00:00Z")
	write("d", "scene", video.ID, "https://gelbooru.example/post/9", "Sun Jul 14 10:00:00 -0500 2019")
	write("e", "image", 99999, "https://danbooru.example/posts/9", "2019-07-14T10:00:00Z")

	run := &mediaDateRun{}
	require.NoError(t, rs.fillMediaDates(context.Background(), nil, run))
	require.Equal(t, 2, run.FromPosted, "%+v", run)
	require.Zero(t, run.FromFile)
	require.Zero(t, run.Failed)
	require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
		read := func(id int) *models.Date {
			item, err := repo.Image.Find(ctx, id)
			require.NoError(t, err)
			return item.Date
		}
		require.Equal(t, "2019-07-14", read(undated.ID).String())
		require.Equal(t, "2001-02-03", read(dated.ID).String(), "an existing Date is kept")
		require.Nil(t, read(renamed.ID), "a record whose URL is not on the item is ignored")
		scene, err := repo.Scene.Find(ctx, video.ID)
		require.NoError(t, err)
		require.Equal(t, "2019-07-14", scene.Date.String())
		return nil
	}))
	// Running again changes nothing.
	again := &mediaDateRun{}
	require.NoError(t, rs.fillMediaDates(context.Background(), nil, again))
	require.Zero(t, again.FromPosted)
	// Cancelling stops the run.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, rs.fillMediaDates(ctx, nil, &mediaDateRun{}), context.Canceled)
}

func TestOnThisDayUsesDateThenDateAdded(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "day.sqlite")))
	defer db.Close()
	repo := db.Repository()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	at := func(s string) time.Time {
		value, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return value
	}
	date := func(s string) *models.Date {
		value, err := models.ParseDate(s)
		require.NoError(t, err)
		return &value
	}
	ids := map[string]int{}
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		for _, item := range []struct {
			name    string
			date    *models.Date
			created string
		}{
			{"dated", date("2019-07-14"), "2025-01-01T00:00:00Z"},     // its Date wins
			{"added", nil, "2021-07-14T12:00:00Z"},                    // no Date: date added
			{"other-day", date("2019-07-15"), "2020-07-14T00:00:00Z"}, // Date wins even if added on the day
			{"year-only", date("2018"), "2022-07-14T00:00:00Z"},       // year-only Date: date added
			{"this-year", date("2026-07-14"), "2026-07-14T00:00:00Z"}, // current year excluded
		} {
			image := models.NewImage()
			image.Title = item.name
			image.Date = item.date
			image.CreatedAt, image.UpdatedAt = at(item.created), at(item.created)
			if err := repo.Image.Create(ctx, &models.CreateImageInput{Image: &image}); err != nil {
				return err
			}
			ids[item.name] = image.ID
		}
		return nil
	}))
	read := func(query string, status int) []models.OnThisDayItem {
		t.Helper()
		w := httptest.NewRecorder()
		rs.OnThisDay(w, httptest.NewRequest("GET", "/on-this-day?"+query, nil))
		require.Equal(t, status, w.Code, w.Body.String())
		var out struct{ Items []models.OnThisDayItem }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.Items
	}
	items := read("kind=image&month=7&day=14&year=2026", 200)
	got := []int{}
	for _, item := range items {
		got = append(got, item.ID)
	}
	require.Equal(t, []int{ids["year-only"], ids["added"], ids["dated"]}, got)
	require.Equal(t, "2022-07-14", items[0].Date)
	require.Empty(t, read("kind=scene&month=7&day=14&year=2026", 200))
	read("kind=gallery&month=7&day=14&year=2026", 400)
	read("kind=image&month=13&day=14&year=2026", 400)
}
