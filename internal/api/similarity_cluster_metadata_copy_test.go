package api

import (
	"reflect"
	"testing"
)

func TestNormalizeSimilarityMetadataCopyTargets(t *testing.T) {
	got, err := normalizeSimilarityMetadataCopyTargets(5, []int{9, 3, 5, 9, 7})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{3, 7, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize targets = %v, want %v", got, want)
	}
}

func TestNormalizeSimilarityMetadataCopyTargetsRejectsInvalidTargets(t *testing.T) {
	if _, err := normalizeSimilarityMetadataCopyTargets(5, []int{0, 7}); err == nil {
		t.Fatal("expected invalid target ID error")
	}
	if _, err := normalizeSimilarityMetadataCopyTargets(5, []int{5, 5}); err == nil {
		t.Fatal("expected source-only target selection error")
	}
}

func TestMissingPositiveIDsIsAddOnlyAndUnique(t *testing.T) {
	got := missingPositiveIDs([]int{2, 4, 4}, []int{1, 2, 3, 3, -1, 4, 5})
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing IDs = %v, want %v", got, want)
	}
}

func TestPositiveUniqueIDsPreservesFirstOccurrence(t *testing.T) {
	got := positiveUniqueIDs([]int{4, 2, 4, 0, -3, 5, 2})
	want := []int{4, 2, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("positive unique IDs = %v, want %v", got, want)
	}
}
