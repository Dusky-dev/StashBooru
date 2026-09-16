package api

import (
	"reflect"
	"testing"
)

func TestNormalizeSceneTaggingBatchIDs(t *testing.T) {
	got, err := normalizeSceneTaggingBatchIDs([]int{9, 3, 9, 5, 3})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{3, 5, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSceneTaggingBatchIDs() = %v, want %v", got, want)
	}
}

func TestNormalizeSceneTaggingBatchIDsRejectsInvalidID(t *testing.T) {
	if _, err := normalizeSceneTaggingBatchIDs([]int{1, 0, 2}); err == nil {
		t.Fatal("expected invalid video ID error")
	}
}
