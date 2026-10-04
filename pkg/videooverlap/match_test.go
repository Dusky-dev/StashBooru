package videooverlap

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixture(id, seed, seconds int) Signature {
	random := rand.New(rand.NewSource(int64(seed))) // Deterministic labelled frames, not security data.
	ret := Signature{Source: Source{SceneID: id, FileID: int64(id), Path: "fixture", Size: 100, ModTime: 1}, SHA256: strings.Repeat("a", 63) + string("0123456789abcdef"[id%16]), Algorithm: Algorithm, Decoder: strings.Repeat("d", 64), Config: DefaultConfig(), Step: 1, Media: Media{Duration: float64(seconds), Width: 1280, Height: 720}}
	for i := 0; i < seconds; i++ {
		ret.Frames = append(ret.Frames, Frame{Time: float64(i), Hash: random.Uint64(), DHash: random.Uint64(), RGB: [3]float64{120, 140, 160}, Weight: 1})
	}
	return ret
}

func segment(source Signature, id, start, end int) Signature {
	ret := fixture(id, 999, end-start)
	ret.Frames = append([]Frame(nil), source.Frames[start:end]...)
	for i := range ret.Frames {
		ret.Frames[i].Time = float64(i)
	}
	return ret
}

func TestLabelledIntervalClasses(t *testing.T) {
	a := fixture(1, 42, 32)
	for _, name := range []string{"exact", "reencode", "resolution", "trim", "intro", "partial", "reordered", "audio"} {
		t.Run(name, func(t *testing.T) {
			b := fixture(2, 84, 32)
			want := "near-complete-visual"
			switch name {
			case "exact":
				b.SHA256 = a.SHA256
				want = "exact-file"
			case "reencode", "resolution", "audio":
				b.Frames = append([]Frame(nil), a.Frames...)
				if name == "resolution" {
					b.Media.Width, b.Media.Height = 640, 360
				}
				if name == "audio" {
					a.Media.AudioTracks, b.Media.AudioTracks = 1, 1
					a.Media.AudioSHA256, b.Media.AudioSHA256 = "first", "changed"
				}
			case "trim":
				b = segment(a, 2, 5, 25)
				want = "contained-clip"
			case "intro":
				b = fixture(2, 84, 40)
				for i, f := range a.Frames {
					f.Time = float64(i + 8)
					b.Frames[i+8] = f
				}
				want = "contained-clip"
			case "partial":
				for i, f := range a.Frames[20:] {
					f.Time = float64(i)
					b.Frames[i] = f
				}
				want = "partial-overlap"
			case "reordered":
				for i := 0; i < 32; i++ {
					b.Frames[i] = a.Frames[(i+16)%32]
					b.Frames[i].Time = float64(i)
				}
				want = "compilation-segments"
			}
			m, err := Compare(context.Background(), a, b, DefaultMatchOptions())
			require.NoError(t, err)
			require.Equal(t, want, m.Class)
			require.NotEmpty(t, m.Intervals)
			if name == "trim" {
				require.InDelta(t, -5, m.Intervals[0].Offset, .01)
				require.InDelta(t, 1, m.CoverageB, .04)
				require.Less(t, m.CoverageA, .7)
			}
			if name == "intro" {
				require.InDelta(t, 8, m.Intervals[0].Offset, .01)
			}
			if name == "reordered" {
				require.Len(t, m.Intervals, 2)
				require.Greater(t, m.Intervals[0].B.Start, m.Intervals[1].B.Start)
			}
			if name == "audio" {
				require.Contains(t, m.Audio, "digests differ")
			}
			if name == "exact" {
				require.Equal(t, 0.0, m.Tolerance)
				require.Equal(t, 1.0, m.CoverageA)
			}
		})
	}
}

func TestShortClipInsideLongCompilationAndMultipleMappings(t *testing.T) {
	a := fixture(1, 42, 40)
	clip := segment(a, 3, 12, 20)
	b := fixture(2, 84, 300)
	for i, f := range a.Frames[12:20] {
		f.Time = float64(100 + i)
		b.Frames[100+i] = f
	}
	for i, f := range a.Frames[28:36] {
		f.Time = float64(200 + i)
		b.Frames[200+i] = f
	}
	m, err := Compare(context.Background(), clip, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.Equal(t, "contained-clip", m.Class)
	require.InDelta(t, 100, m.Intervals[0].Offset, .01)
	require.Less(t, m.CoverageB, .04)
	m, err = Compare(context.Background(), a, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.Equal(t, "compilation-segments", m.Class)
	require.Len(t, m.Intervals, 2)
	require.Greater(t, len(m.UnmatchedA), 1)
}

func TestSharedTitleBlackAndGapsAreNotWholeFileProof(t *testing.T) {
	a, b := fixture(1, 42, 30), fixture(2, 84, 30)
	for i := 0; i < 10; i++ {
		f := a.Frames[0]
		f.Time = float64(i)
		a.Frames[i], b.Frames[i] = f, f
	}
	DownweightRepeats(a.Frames)
	DownweightRepeats(b.Frames)
	m, err := Compare(context.Background(), a, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.Empty(t, m.Class, "an identical opening/title does not establish a duplicate or varied segment")
	for i := range a.Frames {
		a.Frames[i].Weight = 0
		b.Frames[i].Weight = 0
	}
	m, err = Compare(context.Background(), a, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.Empty(t, m.Class)
	b.SHA256 = a.SHA256
	m, err = Compare(context.Background(), a, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.Equal(t, "exact-file", m.Class, "whole-file hashing can still prove identical blank videos")
	a, b = fixture(1, 42, 30), fixture(2, 84, 30)
	b.Frames = append([]Frame(nil), a.Frames...)
	a.Frames[12].Weight, b.Frames[12].Weight = 0, 0
	m, err = Compare(context.Background(), a, b, DefaultMatchOptions())
	require.NoError(t, err)
	require.NotEmpty(t, m.Intervals[0].GapsA)
	require.NotEmpty(t, m.UnmatchedA)
	require.Less(t, m.CoverageA, 1.0)
}

func TestDistanceMissingDataVersionCancellationAndSpeed(t *testing.T) {
	a, b := fixture(1, 42, 24), fixture(2, 84, 24)
	b.Frames = append([]Frame(nil), a.Frames...)
	for i := range b.Frames {
		b.Frames[i].Hash ^= 7
	}
	o := DefaultMatchOptions()
	o.MaxDistance = 2
	m, err := Compare(context.Background(), a, b, o)
	require.NoError(t, err)
	require.Empty(t, m.Class)
	o.MaxDistance = 3
	m, err = Compare(context.Background(), a, b, o)
	require.NoError(t, err)
	require.Equal(t, "near-complete-visual", m.Class)
	require.Equal(t, "Strong sampled evidence", m.Confidence)
	b.Algorithm = "old"
	_, err = Compare(context.Background(), a, b, o)
	require.ErrorContains(t, err, "reindex")
	b.Algorithm = Algorithm
	b.Frames = nil
	m, err = Compare(context.Background(), a, b, o)
	require.NoError(t, err)
	require.Empty(t, m.Class)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Compare(ctx, a, b, o)
	require.ErrorIs(t, err, context.Canceled)
	b = fixture(2, 84, 24)
	b.Frames = append([]Frame(nil), a.Frames...)
	for i := range b.Frames {
		b.Frames[i].Time *= 1.5
	}
	b.Media.Duration = 36
	m, err = Compare(context.Background(), a, b, o)
	require.NoError(t, err)
	require.Empty(t, m.Class, "systematic speed drift must not be reported as compilation cuts")
	o.MinimumSeconds = math.NaN()
	require.Error(t, o.Validate())
}

func TestFingerprintBarsUniformFramesAndRepeatWeights(t *testing.T) {
	rgb := make([]byte, 32*32*3)
	f, err := Fingerprint(0, rgb)
	require.NoError(t, err)
	require.Zero(t, f.Weight)
	random := rand.New(rand.NewSource(9))
	for y := 4; y < 28; y++ {
		for x := 0; x < 32; x++ {
			for c := 0; c < 3; c++ {
				rgb[(y*32+x)*3+c] = byte(30 + random.Intn(180))
			}
		}
	}
	f, err = Fingerprint(1.25, rgb)
	require.NoError(t, err)
	require.Equal(t, 1.25, f.Time)
	require.Equal(t, 1.0, f.Weight)
	_, err = Fingerprint(0, rgb[:20])
	require.Error(t, err)
	frames := []Frame{f, f, f, f}
	DownweightRepeats(frames)
	for _, f := range frames {
		require.Zero(t, f.Weight)
	}
}
