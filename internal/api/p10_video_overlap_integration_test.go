//go:build integration

package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Native fixture fingerprint, not a security primitive.
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/videooverlap"
	"github.com/stretchr/testify/require"
)

type p10Progress struct{ total, processed int }

func (p *p10Progress) SetTotal(n int)     { p.total = n }
func (p *p10Progress) SetProcessed(n int) { p.processed = n }
func (p *p10Progress) Increment()         { p.processed++ }

// This HTTP worker tests the native catalogue/job boundary. Real decoding and
// interval measurements are independently exercised by make test-video-overlap.
type p10Worker struct {
	mu       sync.Mutex
	calls    int
	fail     map[string]bool
	cancelAt int
	cancel   context.CancelFunc
}

func p10Client(t *testing.T) (videooverlap.Client, videooverlap.Capabilities, *p10Worker) {
	t.Helper()
	caps := videooverlap.Capabilities{Algorithm: videooverlap.Algorithm, Signature: strings.Repeat("a", 64), MaxFrames: videooverlap.MaxFrames}
	worker := &p10Worker{fail: map[string]bool{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(caps)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		worker.mu.Lock()
		worker.calls++
		fail := worker.fail[string(body)]
		if worker.calls == worker.cancelAt && worker.cancel != nil {
			worker.cancel()
		}
		worker.mu.Unlock()
		if fail {
			http.Error(w, "invalid fixture media", 422)
			return
		}
		frames := []map[string]any{}
		for i := 0; i < 12; i++ {
			rgb := make([]byte, 32*32*3)
			random := rand.New(rand.NewSource(int64(17 + i)))
			_, _ = random.Read(rgb)
			frames = append(frames, map[string]any{"time": i, "rgb": rgb})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"algorithm": caps.Algorithm, "decoder": caps.Signature, "sha256": fmt.Sprintf("%x", sha256.Sum256(body)), "step": 1,
			"media": videooverlap.Media{Duration: 12, Width: 128, Height: 96, Codec: "h264", AudioTracks: 1}, "frames": frames})
	}))
	t.Cleanup(server.Close)
	return videooverlap.Client{URL: server.URL, Token: "fixture-token"}, caps, worker
}

func p10Files(t *testing.T, r models.Repository, count int) []string {
	t.Helper()
	paths := make([]string, count)
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		files, err := r.File.Find(ctx, 2)
		require.NoError(t, err)
		base := *files[0].Base()
		for i := 0; i < count; i++ {
			f := &models.VideoFile{BaseFile: &models.BaseFile{ParentFolderID: base.ParentFolderID, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}, Format: "mp4", Width: 128, Height: 96, Duration: 12, VideoCodec: "h264"}
			if i == 0 {
				f = files[0].(*models.VideoFile)
			}
			f.Path = filepath.Join(filepath.Dir(base.Path), fmt.Sprintf("video-%d.mp4", i+1))
			f.Basename = filepath.Base(f.Path)
			body := []byte(fmt.Sprintf("native synthetic video %02d", i+1))
			if i == 1 {
				body = []byte("native synthetic video 01")
			}
			require.NoError(t, os.WriteFile(f.Path, body, 0600))
			stat, e := os.Stat(f.Path)
			require.NoError(t, e)
			f.Size, f.ModTime = stat.Size(), stat.ModTime()
			f.SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: fmt.Sprintf("%x", md5.Sum(body))})
			if i == 0 {
				require.NoError(t, r.File.Update(ctx, f))
			} else {
				require.NoError(t, r.File.Create(ctx, f))
				require.NoError(t, r.Scene.AddFileID(ctx, i+1, f.ID))
			}
			_, err = r.Scene.UpdatePartial(ctx, i+1, models.ScenePartial{PrimaryFileID: &f.ID})
			require.NoError(t, err)
			paths[i] = f.Path
		}
		return nil
	}))
	return paths
}

func p10IndexJob(total int) *videooverlap.Job {
	return &videooverlap.Job{ID: videooverlap.NewID(), Action: "index", Status: "running", Config: videooverlap.DefaultConfig(), All: true, UpperID: total, Total: total, Items: []videooverlap.JobItem{}, FailedIDs: []int{}}
}

func p10SearchJob() *videooverlap.Job {
	return &videooverlap.Job{ID: videooverlap.NewID(), Action: "search", Status: "running", Config: videooverlap.DefaultConfig(), Versions: map[int]string{}, Review: &videooverlap.Review{Reference: 1, Options: videooverlap.DefaultMatchOptions(), Matches: []videooverlap.Match{}, Errors: []string{}}}
}

func TestP10IndexSearchPreserveNativeCatalogueAndRejectStale(t *testing.T) {
	ctx := context.Background()
	r := p07Fixture(t)
	paths := p10Files(t, r, 3)
	store := videooverlap.Store{Root: t.TempDir()}
	client, caps, worker := p10Client(t)
	require.NoError(t, r.WithTxn(ctx, func(ctx context.Context) error {
		character, artist, tag := models.NewPerformer(), models.NewStudio(), models.NewTag()
		character.Name, artist.Name, tag.Name = "Character", "Artist", "Direct tag"
		require.NoError(t, r.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &character}))
		require.NoError(t, r.Studio.Create(ctx, &models.CreateStudioInput{Studio: &artist}))
		require.NoError(t, r.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
		_, err := r.Scene.UpdatePartial(ctx, 1, models.ScenePartial{PerformerIDs: &models.UpdateIDs{IDs: []int{character.ID}, Mode: models.RelationshipUpdateModeSet}, TagIDs: &models.UpdateIDs{IDs: []int{tag.ID}, Mode: models.RelationshipUpdateModeSet}})
		require.NoError(t, err)
		require.NoError(t, r.SceneArtist.SetSceneArtists(ctx, 1, []int{artist.ID}))
		copyright, err := r.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Native Copyright"})
		require.NoError(t, err)
		require.NoError(t, r.Copyright.SetSceneCopyrights(ctx, 1, []int{copyright.ID}))
		gallery := models.NewGallery()
		require.NoError(t, r.Gallery.Create(ctx, &models.CreateGalleryInput{Gallery: &gallery}))
		_, err = r.Scene.UpdatePartial(ctx, 1, models.ScenePartial{GalleryIDs: &models.UpdateIDs{IDs: []int{gallery.ID}, Mode: models.RelationshipUpdateModeSet}, URLs: &models.UpdateStrings{Values: []string{"https://example.test/provenance"}, Mode: models.RelationshipUpdateModeSet}})
		require.NoError(t, err)
		return r.Scene.SetCustomFields(ctx, 1, models.CustomFieldsInput{Full: map[string]interface{}{"provenance": "original"}})
	}))
	snapshot := func() []byte {
		var data []byte
		require.NoError(t, r.WithReadTxn(ctx, func(ctx context.Context) error {
			scene, err := r.Scene.Find(ctx, 1)
			require.NoError(t, err)
			require.NoError(t, scene.LoadPrimaryFile(ctx, r.File))
			require.NoError(t, scene.LoadURLs(ctx, r.Scene))
			require.NoError(t, scene.LoadTagIDs(ctx, r.Scene))
			require.NoError(t, scene.LoadPerformerIDs(ctx, r.Scene))
			require.NoError(t, scene.LoadGalleryIDs(ctx, r.Scene))
			copyrights, err := r.Copyright.FindBySceneIDOrdered(ctx, 1)
			require.NoError(t, err)
			artists, err := r.SceneArtist.FindBySceneID(ctx, 1)
			require.NoError(t, err)
			fields, err := r.Scene.GetCustomFields(ctx, 1)
			require.NoError(t, err)
			data, err = json.Marshal([]any{scene, scene.Files.Primary(), copyrights, artists, fields})
			return err
		}))
		return data
	}
	before := snapshot()
	job := p10IndexJob(3)
	require.NoError(t, runVideoOverlapIndex(ctx, r, store, job, client, caps, &p10Progress{}))
	require.Equal(t, 3, job.Indexed)
	require.Equal(t, before, snapshot())
	search := p10SearchJob()
	require.NoError(t, runVideoOverlapSearch(ctx, r, store, search, caps, &p10Progress{}))
	require.Len(t, search.Review.Matches, 2)
	require.Equal(t, "exact-file", search.Review.Matches[0].Class)
	require.Equal(t, before, snapshot(), "review preserves files, active fingerprints, native IDs, relationships and provenance")
	worker.mu.Lock()
	worker.calls = 0
	worker.mu.Unlock()
	incremental := p10IndexJob(3)
	require.NoError(t, runVideoOverlapIndex(ctx, r, store, incremental, client, caps, &p10Progress{}))
	require.Equal(t, 3, incremental.Skipped)
	worker.mu.Lock()
	require.Zero(t, worker.calls)
	worker.mu.Unlock()
	changedDecoder := caps
	changedDecoder.Signature = strings.Repeat("b", 64)
	require.ErrorContains(t, runVideoOverlapSearch(ctx, r, store, p10SearchJob(), changedDecoder, &p10Progress{}), "missing or stale")
	changedConfig := p10SearchJob()
	changedConfig.Config.SampleSeconds = 2
	require.ErrorContains(t, runVideoOverlapSearch(ctx, r, store, changedConfig, caps, &p10Progress{}), "missing or stale")
	// Even unchanged stat/native hashes cannot authorize an exact-file claim if
	// bytes changed externally. Restore mtime deliberately to exercise rehashing.
	stat, err := os.Stat(paths[1])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(paths[1], []byte("native synthetic video xx"), 0600))
	require.NoError(t, os.Chtimes(paths[1], stat.ModTime(), stat.ModTime()))
	stale := p10SearchJob()
	require.NoError(t, runVideoOverlapSearch(ctx, r, store, stale, caps, &p10Progress{}))
	require.Equal(t, 1, stale.Review.Skipped)
	require.Contains(t, strings.Join(stale.Review.Errors, " "), "contents changed")
	// A native primary-file swap invalidates the previous signature even if its
	// pixels could be similar. Source lineage is never the active identity.
	require.NoError(t, r.WithTxn(ctx, func(ctx context.Context) error {
		scene, err := r.Scene.Find(ctx, 3)
		require.NoError(t, err)
		require.NoError(t, scene.LoadPrimaryFile(ctx, r.File))
		id := scene.Files.Primary().Base().ID
		require.NoError(t, r.Scene.AddFileID(ctx, 2, id))
		_, err = r.Scene.UpdatePartial(ctx, 2, models.ScenePartial{PrimaryFileID: &id})
		return err
	}))
	stale = p10SearchJob()
	require.NoError(t, runVideoOverlapSearch(ctx, r, store, stale, caps, &p10Progress{}))
	require.Contains(t, strings.Join(stale.Review.Errors, " "), "index is stale")
	// A bounded alignment must be disclosed even when it produces no interval.
	limited, err := store.Get(ctx, 3)
	require.NoError(t, err)
	ref, err := store.Get(ctx, 1)
	require.NoError(t, err)
	limited.Frames = nil
	for i := 0; i < 300; i++ {
		f := ref.Frames[0]
		f.Time = float64(i)
		limited.Frames = append(limited.Frames, f)
	}
	limited.Media.Duration = 300
	require.NoError(t, store.Put(ctx, *limited))
	bounded := p10SearchJob()
	require.NoError(t, runVideoOverlapSearch(ctx, r, store, bounded, caps, &p10Progress{}))
	require.Equal(t, 1, bounded.Review.AlignmentLimited)
}

func TestP10CancelledIndexResumesAtomicCheckpointAndFailedItems(t *testing.T) {
	r := p07Fixture(t)
	paths := p10Files(t, r, 5)
	store := videooverlap.Store{Root: t.TempDir()}
	client, caps, worker := p10Client(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker.mu.Lock()
	worker.cancelAt, worker.cancel = 2, cancel
	worker.mu.Unlock()
	job := p10IndexJob(5)
	require.ErrorIs(t, runVideoOverlapIndex(ctx, r, store, job, client, caps, &p10Progress{}), context.Canceled)
	require.Equal(t, 1, job.Cursor)
	require.Equal(t, 1, job.Processed)
	require.NoError(t, store.SaveJob(context.Background(), job))
	reloaded, err := (videooverlap.Store{Root: store.Root}).LoadJob(context.Background(), job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, reloaded.Cursor)
	body, err := os.ReadFile(paths[2])
	require.NoError(t, err)
	worker.mu.Lock()
	worker.cancelAt = 0
	worker.fail[string(body)] = true
	worker.mu.Unlock()
	require.NoError(t, runVideoOverlapIndex(context.Background(), r, store, reloaded, client, caps, &p10Progress{}))
	require.Equal(t, 5, reloaded.Processed)
	require.Equal(t, 4, reloaded.Indexed)
	require.Equal(t, []int{3}, reloaded.FailedIDs)
	require.Equal(t, 5, reloaded.Cursor)
	// Cancellation while retrying retains the original failed IDs and counts,
	// rather than losing them when the stored cursor is already at the end.
	ctx, cancelRetry := context.WithCancel(context.Background())
	defer cancelRetry()
	worker.mu.Lock()
	worker.cancelAt = worker.calls + 1
	worker.cancel = cancelRetry
	worker.mu.Unlock()
	require.ErrorIs(t, runVideoOverlapIndex(ctx, r, store, reloaded, client, caps, &p10Progress{}), context.Canceled)
	require.Equal(t, []int{3}, reloaded.FailedIDs)
	require.Equal(t, 5, reloaded.Processed)
	require.Equal(t, 1, reloaded.Failed)
	require.NotEmpty(t, reloaded.Items[2].Error, "cancelled retry retains the original per-item error")
	worker.mu.Lock()
	worker.cancelAt = 0
	worker.fail[string(body)] = false
	worker.mu.Unlock()
	require.NoError(t, runVideoOverlapIndex(context.Background(), r, store, reloaded, client, caps, &p10Progress{}))
	require.Empty(t, reloaded.FailedIDs)
	require.Equal(t, 5, reloaded.Indexed)
	require.Zero(t, reloaded.Failed)
	// Library jobs keep the captured upper bound when later Videos are created.
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error { scene := models.NewScene(); return r.Scene.Create(ctx, &scene, nil) }))
	require.NoError(t, runVideoOverlapIndex(context.Background(), r, store, p10IndexJob(5), client, caps, &p10Progress{}))
}
