package videooverlap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRealVideoIntervals(t *testing.T) {
	root := os.Getenv("STASH_VIDEO_OVERLAP_FIXTURES")
	if root == "" {
		t.Skip("run make test-video-overlap for labelled real FFmpeg fixtures")
	}
	data, err := os.ReadFile(filepath.Join(root, "cases.json"))
	require.NoError(t, err)
	var cases []struct {
		Name      string   `json:"name"`
		A         string   `json:"a"`
		B         string   `json:"b"`
		Class     string   `json:"class"`
		NotClass  string   `json:"notClass"`
		Offset    *float64 `json:"offset"`
		Intervals int      `json:"intervals"`
		AudioDiff bool     `json:"audioDiff"`
	}
	require.NoError(t, json.Unmarshal(data, &cases))
	ctx := context.Background()
	client := Client{}
	caps, err := client.Capabilities(ctx)
	require.NoError(t, err)
	config := DefaultConfig()
	config.AudioDigest = true
	indexed := map[string]Signature{}
	get := func(name string) Signature {
		if s, ok := indexed[name]; ok {
			return s
		}
		id := len(indexed) + 1
		source, err := Measure(Source{SceneID: id, FileID: int64(id), Path: filepath.Join(root, name)})
		require.NoError(t, err)
		s, err := client.Index(ctx, source, config, caps)
		require.NoError(t, err)
		indexed[name] = s
		return s
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			a, b := get(c.A), get(c.B)
			m, err := Compare(ctx, a, b, DefaultMatchOptions())
			require.NoError(t, err)
			if c.NotClass != "" {
				require.NotEqual(t, c.NotClass, m.Class)
			} else {
				require.Equal(t, c.Class, m.Class)
			}
			if c.Offset != nil {
				require.NotEmpty(t, m.Intervals)
				require.InDelta(t, *c.Offset, m.Intervals[0].Offset, m.Tolerance, "measured temporal offset")
			}
			if c.Intervals > 0 {
				require.GreaterOrEqual(t, len(m.Intervals), c.Intervals)
			}
			if c.AudioDiff {
				require.Contains(t, m.Audio, "digests differ")
			}
			t.Logf("class=%q intervals=%d coverage=%.3f/%.3f tolerance=%.3fs", m.Class, len(m.Intervals), m.CoverageA, m.CoverageB, m.Tolerance)
		})
	}
	// Retrieval is exercised with the actual decoded fingerprints too: the
	// 8-second clip must retrieve a much longer compilation without duration gates.
	s := Store{Root: t.TempDir()}
	for _, signature := range indexed {
		require.NoError(t, s.Put(ctx, signature))
	}
	clip, comp := get("clip.mp4"), get("compilation.mp4")
	report, err := s.Candidates(ctx, clip, 200)
	require.NoError(t, err)
	require.Contains(t, report.IDs, comp.Source.SceneID)
}
