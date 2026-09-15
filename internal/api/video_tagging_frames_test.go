package api

import (
	"reflect"
	"testing"
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
