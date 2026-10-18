//go:build integration

package manager

import (
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/sqlite"
	_ "github.com/stashapp/stash/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func TestBooruScanNowRegistersDownloadedImageIdempotently(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	config.InitializeEmpty()
	cfg := config.GetInstance()
	root := t.TempDir()
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yml"))
	cfg.SetInterface(config.Stash, []string{root})
	cfg.SetInterface(config.ImageExtensions, []string{"png"})
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "booru.sqlite")))
	defer func() { require.NoError(t, db.Close()) }()
	mgr := &Manager{Config: cfg, Repository: db.Repository(), FFMpeg: ffmpeg.NewEncoder(ffmpegPath), FFProbe: ffmpeg.NewFFProbe(probePath), PluginCache: plugin.NewCache(cfg), Paths: &paths.Paths{}, scanSubs: &subscriptionManager{}}
	previous := instance
	instance = mgr
	defer func() { instance = previous }()
	path := filepath.Join(root, "booru-fixture.png")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, image.NewRGBA(image.Rect(0, 0, 8, 8))))
	require.NoError(t, f.Close())
	jobs := job.NewManager()
	defer jobs.Stop()
	for i := 0; i < 2; i++ {
		done := make(chan error, 1)
		jobs.Add(context.Background(), "Booru import scan", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error {
			err := mgr.ScanNow(ctx, ScanMetadataInput{Paths: []string{path}}, p)
			done <- err
			return err
		}))
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(20 * time.Second):
			t.Fatal("native scan did not finish inside the existing Job")
		}
	}
	require.NoError(t, mgr.Repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		file, err := mgr.Repository.File.FindByPath(ctx, path, true)
		require.NoError(t, err)
		require.NotNil(t, file)
		images, err := mgr.Repository.Image.FindByFileID(ctx, file.Base().ID)
		require.NoError(t, err)
		require.Len(t, images, 1)
		require.NotNil(t, images[0].PrimaryFileID)
		require.Equal(t, file.Base().ID, *images[0].PrimaryFileID)
		return nil
	}))
}
