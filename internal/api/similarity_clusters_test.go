package api

import (
	"reflect"
	"testing"
)

func TestBuildSimilarityClusters(t *testing.T) {
	assetIDs := []int{1, 2, 3, 4, 5}
	edges := []similarityClusterEdge{
		{LeftID: 1, RightID: 2, Similarity: 0.95},
		{LeftID: 2, RightID: 3, Similarity: 0.91},
		{LeftID: 4, RightID: 5, Similarity: 0.70},
	}

	got := buildSimilarityClusters(assetIDs, edges, 0.9)
	want := [][]int{{1, 2, 3}, {4}, {5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected clusters: got %v want %v", got, want)
	}
}

func TestBuildSimilarityClustersKeepsSingletonsAndIgnoresUnknownAssets(t *testing.T) {
	assetIDs := []int{3, 1, 2}
	edges := []similarityClusterEdge{
		{LeftID: 1, RightID: 99, Similarity: 0.99},
		{LeftID: 1, RightID: 2, Similarity: 0.80},
	}

	got := buildSimilarityClusters(assetIDs, edges, 0.9)
	want := [][]int{{1}, {2}, {3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected singleton clusters: got %v want %v", got, want)
	}
}
