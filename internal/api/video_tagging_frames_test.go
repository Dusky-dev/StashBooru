package api

import (
	"math"
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestVideoTaggingFrameSampleTimes(t *testing.T) {
	got := videoTaggingFrameSampleTimes(100, 3)
	want := []float64{25, 50, 75}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sample times: got %v want %v", got, want)
	}
}

func TestVideoTaggingFrameSampleTimesDefaultsAndCaps(t *testing.T) {
	if got := videoTaggingFrameSampleTimes(40, 0); !reflect.DeepEqual(got, []float64{10, 20, 30}) {
		t.Fatalf("unexpected default sample times: %v", got)
	}
	got := videoTaggingFrameSampleTimes(60, 99)
	want := []float64{10, 20, 30, 40, 50}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected capped sample times: got %v want %v", got, want)
	}
}

func TestVideoTaggingFrameSampleTimesRejectsInvalidDuration(t *testing.T) {
	if got := videoTaggingFrameSampleTimes(0, 3); got != nil {
		t.Fatalf("expected nil for zero duration, got %v", got)
	}
	if got := videoTaggingFrameSampleTimes(-1, 3); got != nil {
		t.Fatalf("expected nil for negative duration, got %v", got)
	}
}

func TestAggregateVideoFramePredictionsDeduplicatesPerFrameAndTracksConsensus(t *testing.T) {
	frames := [][]camietagger.Tag{
		{
			{Name: "blue_hair", Category: "general", Score: 0.9},
			{Name: "blue hair", Category: "general", Score: 0.2},
		},
		{{Name: "blue hair", Category: "general", Score: 0.8}},
	}

	got := aggregateVideoFramePredictions(frames, 50)
	if len(got) != 1 {
		t.Fatalf("expected one aggregate, got %#v", got)
	}
	if got[0].Source != "frame-analysis:2/2" {
		t.Fatalf("unexpected provenance: %q", got[0].Source)
	}
	if math.Abs(got[0].Score-0.85) > 1e-9 {
		t.Fatalf("unexpected consensus score: %v", got[0].Score)
	}
}

func TestAggregateVideoFramePredictionsPenalizesSingleFrameHits(t *testing.T) {
	frames := [][]camietagger.Tag{
		{{Name: "character_a", Category: "character", Score: 0.9}},
		{},
	}

	got := aggregateVideoFramePredictions(frames, 50)
	if len(got) != 1 {
		t.Fatalf("expected one aggregate, got %#v", got)
	}
	if got[0].Source != "frame-analysis:1/2" || math.Abs(got[0].Score-0.45) > 1e-9 {
		t.Fatalf("unexpected single-frame aggregate: %#v", got[0])
	}
}

func TestFilterVideoFramePredictionsForLocalPriority(t *testing.T) {
	local := []camietagger.Tag{
		{Name: "Alice", Category: "character", Score: 1},
	}
	frames := []camietagger.Tag{
		{Name: "Alice", Category: "character", Score: 0.8},
		{Name: "Bob", Category: "character", Score: 0.9},
		{Name: "Artist One", Category: "artist", Score: 0.7},
		{Name: "outdoors", Category: "general", Score: 0.6},
	}

	got := filterVideoFramePredictionsForLocalPriority(local, frames)
	if len(got) != 3 {
		t.Fatalf("expected local-matching Character plus non-conflicting metadata, got %#v", got)
	}
	for _, prediction := range got {
		if prediction.Category == "character" && prediction.Name == "Bob" {
			t.Fatalf("conflicting frame Character was not suppressed: %#v", got)
		}
	}
}
