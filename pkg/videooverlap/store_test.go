package videooverlap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDurableIndexInvertedCandidatesReindexAndLimits(t *testing.T) {
	ctx := context.Background()
	s := Store{Root: t.TempDir()}
	a := fixture(1, 42, 24)
	require.NoError(t, s.Put(ctx, a))
	for id := 2; id <= 15; id++ {
		b := fixture(id, 42, 24)
		if id == 2 {
			b.SHA256 = a.SHA256
			b.Frames = nil
			b.Frames = []Frame{{Time: 0, Weight: 0}}
		}
		require.NoError(t, s.Put(ctx, b))
	}
	report, err := s.Candidates(ctx, a, 3)
	require.NoError(t, err)
	require.Equal(t, 14, report.Total)
	require.Len(t, report.IDs, 3)
	require.Equal(t, 2, report.IDs[0], "exact digest candidate survives blank-frame filtering and ranks first")
	require.True(t, report.Limited)
	stored, err := (Store{Root: s.Root}).Get(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, a.Frames, stored.Frames, "JSON retains all 64 hash bits across reopening")
	b := fixture(3, 99999, 24)
	b.Frames = []Frame{{Time: 0, Weight: 0}}
	require.NoError(t, s.Put(ctx, b))
	report, err = s.Candidates(ctx, a, 100)
	require.NoError(t, err)
	require.NotContains(t, report.IDs, 3, "reindex replaces obsolete postings atomically")
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, s.Put(ctx, b), context.Canceled)
	count, err := s.Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 15, count)
}

func TestCacheIdentityIncludesActiveSourceSettingsDecoderAndAlgorithm(t *testing.T) {
	a := fixture(1, 42, 24)
	require.True(t, a.Current(a.Source, a.Config, a.Decoder))
	for _, change := range []func(*Signature){func(s *Signature) { s.Source.ModTime++ }, func(s *Signature) { s.Source.FileID++ }, func(s *Signature) { s.Source.Fingerprint = "new active hash" }, func(s *Signature) { s.Source.Path = "new path" }, func(s *Signature) { s.Source.Size++ }, func(s *Signature) { s.Config.AudioDigest = true }, func(s *Signature) { s.Config.SampleSeconds = 2 }, func(s *Signature) { s.Decoder = "new decoder" }, func(s *Signature) { s.Algorithm = "old algorithm" }} {
		b := a
		change(&b)
		require.False(t, b.Current(a.Source, a.Config, a.Decoder))
		require.NotEqual(t, a.Key(), b.Key())
	}
	path := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(path, []byte("original bytes"), 0600))
	source, err := Measure(Source{Path: path})
	require.NoError(t, err)
	require.EqualValues(t, 14, source.Size)
	hash, err := DigestFile(context.Background(), path)
	require.NoError(t, err)
	require.Len(t, hash, 64)
	require.NoError(t, os.WriteFile(path, []byte("different data"), 0600))
	changed, err := DigestFile(context.Background(), path)
	require.NoError(t, err)
	require.NotEqual(t, hash, changed)
}

func TestSettingsAndSavedCheckpointsSurviveRestart(t *testing.T) {
	ctx := context.Background()
	s := Store{Root: t.TempDir()}
	c, err := s.Config(ctx)
	require.NoError(t, err)
	require.Equal(t, DefaultConfig(), c)
	c.Backend = "remote"
	c.AudioDigest = true
	require.NoError(t, s.Configure(ctx, c))
	c2, err := (Store{Root: s.Root}).Config(ctx)
	require.NoError(t, err)
	require.Equal(t, c, c2)
	j := &Job{ID: NewID(), Action: "index", Status: "running", All: true, UpperID: 900, Cursor: 100, FailedIDs: []int{3}, Processed: 100, Failed: 1, Config: c, Items: []JobItem{{ID: 3, Status: "failed", Error: "decoder missing"}}}
	require.NoError(t, s.SaveJob(ctx, j))
	saved, err := (Store{Root: s.Root}).LoadJob(ctx, j.ID)
	require.NoError(t, err)
	require.Equal(t, 100, saved.Cursor)
	require.Equal(t, 900, saved.UpperID)
	require.Equal(t, []int{3}, saved.FailedIDs)
	for i := 0; i < 60; i++ {
		j.AddItem(JobItem{ID: i + 1, Status: "failed", Error: strings.Repeat("x", 3000)})
	}
	require.Len(t, j.Items, 50)
	require.Len(t, j.Items[0].Error, 2048)
	require.NoError(t, s.SaveJob(ctx, j))
	jobs, err := s.LatestJobs(ctx, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
}
