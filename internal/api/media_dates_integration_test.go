//go:build integration

package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

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
