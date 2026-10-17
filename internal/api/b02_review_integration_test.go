//go:build integration

package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Fixture content identity.
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

func b02Request(t *testing.T, rs booruBrowserRoutes, update bool, payload any, status int) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	request := httptest.NewRequest("POST", "/booru-browser/review", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	if update {
		rs.UpdateReview(w, request)
	} else {
		rs.ReadReview(w, request)
	}
	require.Equal(t, status, w.Code, w.Body.String())
	return w.Body.Bytes()
}
func b02Read(t *testing.T, rs booruBrowserRoutes, source booru.Source, posts ...booruReviewPost) []booruReviewItem {
	t.Helper()
	body := b02Request(t, rs, false, booruReviewRead{Source: source.ID, Instance: source.Instance(), Posts: posts}, 200)
	var rows []booruReviewItem
	require.NoError(t, json.Unmarshal(body, &rows))
	return rows
}
func b02Write(t *testing.T, rs booruBrowserRoutes, source booru.Source, changes ...models.BooruReviewChange) []models.BooruReviewState {
	t.Helper()
	body := b02Request(t, rs, true, booruReviewUpdate{Source: source.ID, Instance: source.Instance(), Changes: changes}, 200)
	var rows []models.BooruReviewState
	require.NoError(t, json.Unmarshal(body, &rows))
	return rows
}
func b02Bool(value bool) *bool { return &value }

func TestB02ReviewPersistenceIsolationConflictsAndUndo(t *testing.T) {
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "review.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	require.Zero(t, b02Read(t, rs, dan, booruReviewPost{ID: "1"})[0].Version)
	marked := b02Write(t, rs, dan, models.BooruReviewChange{PostID: "1", Ignored: b02Bool(true), Downloaded: b02Bool(true)})
	require.NotEmpty(t, marked[0].IgnoredAt)
	require.NotEmpty(t, marked[0].DownloadedAt)
	require.Zero(t, b02Read(t, rs, gel, booruReviewPost{ID: "1"})[0].Version, "same IDs across providers must remain separate")
	unignored := b02Write(t, rs, dan, models.BooruReviewChange{PostID: "1", Version: marked[0].Version, Ignored: b02Bool(false)})
	require.Empty(t, unignored[0].IgnoredAt)
	require.Equal(t, marked[0].DownloadedAt, unignored[0].DownloadedAt)
	b02Request(t, rs, true, booruReviewUpdate{Source: dan.ID, Instance: dan.Instance(), Changes: []models.BooruReviewChange{
		{PostID: "2", Ignored: b02Bool(true)}, {PostID: "1", Version: 0, Downloaded: b02Bool(false)},
	}}, 409)
	require.Zero(t, b02Read(t, rs, dan, booruReviewPost{ID: "2"})[0].Version, "stale bulk edit must roll back every item")
	bulk := b02Write(t, rs, dan, models.BooruReviewChange{PostID: "1", Version: unignored[0].Version, Ignored: b02Bool(true)}, models.BooruReviewChange{PostID: "2", Ignored: b02Bool(true)})
	undone := b02Write(t, rs, dan, models.BooruReviewChange{PostID: "1", Version: bulk[0].Version, Ignored: b02Bool(false)}, models.BooruReviewChange{PostID: "2", Version: bulk[1].Version, Ignored: b02Bool(false)})
	require.NotEmpty(t, undone[0].DownloadedAt)
	require.Greater(t, undone[1].Version, 0, "cleared rows keep versions to reject ABA edits")
	require.NoError(t, db.Close())
	db = sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	persisted := b02Read(t, rs, dan, booruReviewPost{ID: "1"}, booruReviewPost{ID: "2"})
	require.NotEmpty(t, persisted[0].DownloadedAt)
	require.Equal(t, undone[1].Version, persisted[1].Version)
	// The same settings ID at a different site is a new instance.
	config := fmt.Sprintf(`[{"id":"danbooru","name":"Other","engine":"danbooru","url":"https://other.example.org"}]`)
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(config), 0600))
	changed := dan
	changed.URL = "https://other.example.org"
	b02Request(t, rs, false, booruReviewRead{Source: dan.ID, Instance: dan.Instance(), Posts: []booruReviewPost{{ID: "1"}}}, 409)
	require.Zero(t, b02Read(t, rs, changed, booruReviewPost{ID: "1"})[0].Version)
	require.NoError(t, os.Remove(filepath.Join(rs.configPath, "booru-browser.json")))
	require.NotEmpty(t, b02Read(t, rs, dan, booruReviewPost{ID: "1"})[0].DownloadedAt)
	// Two clients using the same version cannot both succeed.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload, err := json.Marshal(booruReviewUpdate{Source: dan.ID, Instance: dan.Instance(), Changes: []models.BooruReviewChange{{PostID: "3", Ignored: b02Bool(true)}}})
			if err != nil {
				codes <- 500
				return
			}
			request := httptest.NewRequest("POST", "/", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			rs.UpdateReview(w, request)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	sum := 0
	for code := range codes {
		sum += code
	}
	require.Equal(t, 200+409, sum)
}

func TestB02CurrentFileMatchesAreRecomputedAndTyped(t *testing.T) {
	repo := p07Fixture(t)
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: repo}
	source := booru.Defaults()[0]
	data := []byte("original fixture bytes")
	hash := fmt.Sprintf("%x", md5.Sum(data))
	var imagePath, videoPath string
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		files, err := repo.File.Find(ctx, 1, 2)
		if err != nil {
			return err
		}
		for _, file := range files {
			require.NoError(t, os.WriteFile(file.Base().Path, data, 0600))
			require.NoError(t, repo.File.ModifyFingerprints(ctx, file.Base().ID, []models.Fingerprint{{Type: "md5", Fingerprint: hash}}))
			if file.Base().ID == 1 {
				imagePath = file.Base().Path
			} else {
				videoPath = file.Base().Path
			}
		}
		return nil
	}))
	posts := []booruReviewPost{{ID: "1", MD5: hash}, {ID: "2"}, {ID: "3", MD5: strings.Repeat("0", 32)}}
	rows := b02Read(t, rs, source, posts...)
	require.Len(t, rows[0].Matches, 2)
	require.Equal(t, "image", rows[0].Matches[0].Kind)
	require.Equal(t, "scene", rows[0].Matches[1].Kind)
	require.Equal(t, rows[0].Matches[0].ID, rows[0].Matches[1].ID)
	require.Equal(t, "no-remote-md5", rows[1].MatchStatus)
	require.Equal(t, "no-indexed-match", rows[2].MatchStatus)
	b02Write(t, rs, source, models.BooruReviewChange{PostID: "1", Downloaded: b02Bool(true)})
	// Same size and preserved mtime still require rehashing the actual bytes.
	info, err := os.Stat(imagePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(imagePath, []byte(strings.Repeat("x", len(data))), 0600))
	require.NoError(t, os.Chtimes(imagePath, info.ModTime(), info.ModTime()))
	require.NoError(t, os.Remove(videoPath))
	rows = b02Read(t, rs, source, posts...)
	require.Empty(t, rows[0].Matches)
	require.NotEmpty(t, rows[0].DownloadedAt, "historical manual mark survives loss of local evidence")
	require.NoError(t, os.WriteFile(imagePath, data, 0600))
	rows = b02Read(t, rs, source, posts...)
	require.Len(t, rows[0].Matches, 1)
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error { return repo.Image.RemoveFileID(ctx, 1, 1) }))
	rows = b02Read(t, rs, source, posts...)
	require.Empty(t, rows[0].Matches)
	require.NotEmpty(t, rows[0].DownloadedAt)
	require.NoError(t, os.WriteFile(videoPath, data, 0600))
	rows = b02Read(t, rs, source, posts...)
	require.Len(t, rows[0].Matches, 1)
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error { return repo.Scene.Destroy(ctx, 1) }))
	rows = b02Read(t, rs, source, posts...)
	require.Empty(t, rows[0].Matches)
	require.NotEmpty(t, rows[0].DownloadedAt)
}

func TestB02MigrationUpgrades96AndPreservesMedia(t *testing.T) {
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
	require.NoError(t, db.Close())
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`DROP TABLE booru_review; UPDATE schema_migrations SET version=96,dirty=0`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(97), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 97))
	migrator.Close()
	require.NoError(t, db.Open(path))
	defer db.Close()
	repo = db.Repository()
	require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
		found, err := repo.Image.Find(ctx, image.ID)
		require.NoError(t, err)
		require.Equal(t, image.Title, found.Title)
		rows, err := repo.BooruReview.Read(ctx, booru.Defaults()[0].Instance(), []string{"1"})
		require.NoError(t, err)
		require.Zero(t, rows[0].Version)
		return nil
	}))
}
