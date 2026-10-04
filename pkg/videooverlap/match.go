package videooverlap

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"sort"
)

type edge struct {
	a, b, ph, dh int
	offset       float64
}

// Compare aligns shortlisted samples at unit speed. Each returned run is
// monotonic; separate runs may map reordered compilation segments.
func Compare(ctx context.Context, a, b Signature, o MatchOptions) (Match, error) {
	ret := Match{A: a.Source.SceneID, B: b.Source.SceneID, Intervals: []Interval{}, UnmatchedA: []Range{}, UnmatchedB: []Range{}, Tolerance: math.Max(a.Step, b.Step) * 1.25, Audio: audioEvidence(a.Media, b.Media)}
	if err := o.Validate(); err != nil {
		return ret, err
	}
	if err := ctx.Err(); err != nil {
		return ret, err
	}
	if a.Algorithm != Algorithm || b.Algorithm != Algorithm {
		return ret, fmt.Errorf("reindex outdated video signatures")
	}
	if a.SHA256 != "" && a.SHA256 == b.SHA256 {
		ret.Class = "exact-file"
		ret.Intervals = []Interval{{A: Range{0, a.Media.Duration}, B: Range{0, b.Media.Duration}, Support: 1, GapsA: []Range{}, GapsB: []Range{}}}
		ret.CoverageA, ret.CoverageB = 1, 1
		ret.Tolerance = 0
		ret.Evidence = "Whole-file SHA-256 equality; no thumbnail or duration inference."
		ret.Confidence = "Whole-file hash equality"
		return ret, nil
	}
	type bandKey struct {
		band  int
		value uint16
	}
	index := map[bandKey][]int{}
	for j, f := range b.Frames {
		if f.Weight < .4 {
			continue
		}
		for band, v := range Bands(f.Hash) {
			key := bandKey{band, v}
			index[key] = append(index[key], j)
		}
	}
	buckets := map[int][]edge{}
	checks := 0
outer:
	for i, af := range a.Frames {
		if err := ctx.Err(); err != nil {
			return ret, err
		}
		if af.Weight < .4 {
			continue
		}
		seen := map[int]bool{}
		var edges []edge
		for band, v := range Bands(af.Hash) {
			for _, j := range index[bandKey{band, v}] {
				if seen[j] {
					continue
				}
				seen[j] = true
				checks++
				if checks > 2_000_000 {
					ret.Limited = true
					break outer
				}
				bf := b.Frames[j]
				ph, dh := bits.OnesCount64(af.Hash^bf.Hash), bits.OnesCount64(af.DHash^bf.DHash)
				var colorDistance float64
				for k := range af.RGB {
					colorDistance += math.Abs(af.RGB[k] - bf.RGB[k])
				}
				if ph <= o.MaxDistance && dh <= 12 && colorDistance/3 <= 30 {
					edges = append(edges, edge{i, j, ph, dh, bf.Time - af.Time})
				}
			}
		}
		if len(edges) > 64 {
			ret.Limited = true
			sort.Slice(edges, func(i, j int) bool {
				if edges[i].ph != edges[j].ph {
					return edges[i].ph < edges[j].ph
				}
				return edges[i].b < edges[j].b
			})
			edges = edges[:64]
		}
		for _, e := range edges {
			key := int(math.Floor(e.offset / ret.Tolerance))
			buckets[key] = append(buckets[key], e)
			buckets[key+1] = append(buckets[key+1], e)
		}
	}
	var runs []Interval
	keys := make([]int, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return ret, err
		}
		edges := buckets[key]
		center := float64(key) * ret.Tolerance
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].a != edges[j].a {
				return edges[i].a < edges[j].a
			}
			left := float64(edges[i].ph) + math.Abs(edges[i].offset-center)/ret.Tolerance
			right := float64(edges[j].ph) + math.Abs(edges[j].offset-center)/ret.Tolerance
			if left != right {
				return left < right
			}
			return edges[i].b < edges[j].b
		})
		var run []edge
		finish := func() {
			if r, ok := makeInterval(a, b, run, o); ok {
				runs = append(runs, r)
			}
			run = nil
		}
		for i := 0; i < len(edges); {
			e := edges[i]
			next := i + 1
			for next < len(edges) && edges[next].a == e.a {
				next++
			}
			if len(run) > 0 {
				previous := run[len(run)-1]
				// Pick the best still-forward match for this timestamp.
				for k := i; k < next; k++ {
					if edges[k].b > previous.b {
						e = edges[k]
						break
					}
				}
				da := a.Frames[e.a].Time - a.Frames[previous.a].Time
				db := b.Frames[e.b].Time - b.Frames[previous.b].Time
				if e.b <= previous.b || da > 3.1*math.Max(a.Step, b.Step) || db > 3.1*math.Max(a.Step, b.Step) || math.Abs(da-db) > ret.Tolerance {
					finish()
				}
			}
			run = append(run, e)
			i = next
		}
		finish()
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Samples != runs[j].Samples {
			return runs[i].Samples > runs[j].Samples
		}
		if runs[i].MeanDistance != runs[j].MeanDistance {
			return runs[i].MeanDistance < runs[j].MeanDistance
		}
		if runs[i].A.Start != runs[j].A.Start {
			return runs[i].A.Start < runs[j].A.Start
		}
		return runs[i].B.Start < runs[j].B.Start
	})
	for _, run := range runs {
		overlaps := false
		for _, chosen := range ret.Intervals {
			if overlap(run.A, chosen.A) > ret.Tolerance || overlap(run.B, chosen.B) > ret.Tolerance {
				overlaps = true
				break
			}
		}
		if !overlaps {
			if len(ret.Intervals) == 64 {
				ret.Limited = true
				break
			}
			ret.Intervals = append(ret.Intervals, run)
		}
	}
	if len(ret.Intervals) == 0 {
		return ret, nil
	}
	sort.Slice(ret.Intervals, func(i, j int) bool { return ret.Intervals[i].A.Start < ret.Intervals[j].A.Start })
	var rangesA, rangesB []Range
	for _, in := range ret.Intervals {
		rangesA = append(rangesA, subtract(in.A, in.GapsA)...)
		rangesB = append(rangesB, subtract(in.B, in.GapsB)...)
	}
	ret.UnmatchedA, ret.CoverageA = complement(rangesA, a.Media.Duration)
	ret.UnmatchedB, ret.CoverageB = complement(rangesB, b.Media.Duration)
	ret.Class = "partial-overlap"
	piecewise := false
	for i := 1; i < len(ret.Intervals); i++ {
		if math.Abs(ret.Intervals[i].Offset-ret.Intervals[0].Offset) > 2*ret.Tolerance || ret.Intervals[i].B.Start < ret.Intervals[i-1].B.Start {
			piecewise = true
		}
	}
	switch {
	case piecewise:
		ret.Class = "compilation-segments"
	case ret.CoverageA >= .9 && ret.CoverageB >= .9:
		ret.Class = "near-complete-visual"
	case ret.CoverageA >= .9 || ret.CoverageB >= .9:
		ret.Class = "contained-clip"
	}
	ret.Evidence = "Timestamped pHash + dHash + mean-color samples, at least four distinct frames per monotonic segment; support is sample coverage, not a duplicate probability."
	ret.Confidence = "Strong sampled evidence"
	for _, in := range ret.Intervals {
		if in.DistinctFrames < 8 || in.Support < .85 || in.MeanDistance > 4 || in.MeanDHashDistance > 8 || ret.Limited {
			ret.Confidence = "Limited sampled evidence"
			break
		}
	}
	return ret, nil
}

func makeInterval(a, b Signature, run []edge, o MatchOptions) (Interval, bool) {
	ret := Interval{GapsA: []Range{}, GapsB: []Range{}}
	if len(run) < 4 {
		return ret, false
	}
	first, last := run[0], run[len(run)-1]
	// Offset bins alone can turn systematic speed drift into fake cuts. Reject
	// runs whose fitted clock rate differs by more than 10%; speed normalization
	// is outside this version. VFR sampling jitter does not imply a new rate.
	var meanA, meanB float64
	for _, e := range run {
		meanA += a.Frames[e.a].Time
		meanB += b.Frames[e.b].Time
	}
	meanA /= float64(len(run))
	meanB /= float64(len(run))
	var covariance, variance float64
	for _, e := range run {
		da := a.Frames[e.a].Time - meanA
		covariance += da * (b.Frames[e.b].Time - meanB)
		variance += da * da
	}
	if variance <= 0 || math.Abs(covariance/variance-1) > .1 {
		return ret, false
	}
	ret.A = Range{math.Max(0, a.Frames[first.a].Time-a.Step/2), math.Min(a.Media.Duration, a.Frames[last.a].Time+a.Step/2)}
	ret.B = Range{math.Max(0, b.Frames[first.b].Time-b.Step/2), math.Min(b.Media.Duration, b.Frames[last.b].Time+b.Step/2)}
	if math.Min(ret.A.End-ret.A.Start, ret.B.End-ret.B.Start) < o.MinimumSeconds {
		return ret, false
	}
	distinct := map[uint64]bool{}
	for i, e := range run {
		distinct[a.Frames[e.a].Hash] = true
		ret.Offset += e.offset
		ret.MeanDistance += float64(e.ph)
		ret.MeanDHashDistance += float64(e.dh)
		if i > 0 {
			previous := run[i-1]
			if a.Frames[e.a].Time-a.Frames[previous.a].Time > 1.5*a.Step {
				ret.GapsA = append(ret.GapsA, Range{a.Frames[previous.a].Time + a.Step/2, a.Frames[e.a].Time - a.Step/2})
			}
			if b.Frames[e.b].Time-b.Frames[previous.b].Time > 1.5*b.Step {
				ret.GapsB = append(ret.GapsB, Range{b.Frames[previous.b].Time + b.Step/2, b.Frames[e.b].Time - b.Step/2})
			}
		}
	}
	ret.Samples = len(run)
	ret.DistinctFrames = len(distinct)
	ret.Offset /= float64(len(run))
	ret.MeanDistance /= float64(len(run))
	ret.MeanDHashDistance /= float64(len(run))
	ret.Support = math.Min(1, math.Min(float64(len(run))*a.Step/(ret.A.End-ret.A.Start), float64(len(run))*b.Step/(ret.B.End-ret.B.Start)))
	return ret, ret.DistinctFrames >= 4 && ret.Support >= .6
}

func overlap(a, b Range) float64 {
	return math.Max(0, math.Min(a.End, b.End)-math.Max(a.Start, b.Start))
}

func subtract(r Range, gaps []Range) []Range {
	ret := []Range{}
	start := r.Start
	for _, gap := range gaps {
		if gap.Start > start {
			ret = append(ret, Range{start, gap.Start})
		}
		start = math.Max(start, gap.End)
	}
	if start < r.End {
		ret = append(ret, Range{start, r.End})
	}
	return ret
}

func complement(ranges []Range, duration float64) ([]Range, float64) {
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Start < ranges[j].Start })
	ret := []Range{}
	end := 0.0
	for _, r := range ranges {
		start := math.Max(0, r.Start)
		if start > end {
			ret = append(ret, Range{end, start})
		}
		end = math.Max(end, math.Min(duration, r.End))
	}
	if end < duration {
		ret = append(ret, Range{end, duration})
	}
	missing := 0.0
	for _, r := range ret {
		missing += r.End - r.Start
	}
	if duration <= 0 {
		return ret, 0
	}
	return ret, math.Max(0, math.Min(1, 1-missing/duration))
}

func audioEvidence(a, b Media) string {
	if a.AudioTracks == 0 && b.AudioTracks == 0 {
		return "Neither file has an audio track."
	}
	if a.AudioTracks != b.AudioTracks {
		return fmt.Sprintf("Different audio track counts (%d / %d); tracks may contain unique content.", a.AudioTracks, b.AudioTracks)
	}
	if a.AudioSHA256 == "" || b.AudioSHA256 == "" {
		return "Audio content not compared; enable the optional whole first-track decoded digest when indexing."
	}
	if a.AudioSHA256 == b.AudioSHA256 {
		return "Whole first audio tracks decode to the same 8 kHz mono PCM digest; other tracks and interval audio are not verified."
	}
	return "Whole first audio-track digests differ; trimming, timing, mixing or re-encoding can also cause this. Interval audio and other tracks are not verified."
}
