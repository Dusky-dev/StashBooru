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

func TestSimilarityClusterEdgeKeyIsUndirected(t *testing.T) {
	if got, want := similarityClusterEdgeKey(9, 2), [2]int{2, 9}; got != want {
		t.Fatalf("unexpected edge key: got %v want %v", got, want)
	}
}

func TestPreferredSimilarityClusterMember(t *testing.T) {
	tests := []struct {
		name    string
		members []similarityClusterMember
		want    int
	}{
		{
			name: "resolution wins",
			members: []similarityClusterMember{
				{ID: 1, Width: 1000, Height: 1000, FileSize: 9_000_000},
				{ID: 2, Width: 2000, Height: 1000, FileSize: 1_000_000},
			},
			want: 2,
		},
		{
			name: "file size breaks equal resolution",
			members: []similarityClusterMember{
				{ID: 1, Width: 2000, Height: 1000, FileSize: 1_000_000},
				{ID: 2, Width: 2000, Height: 1000, FileSize: 2_000_000},
			},
			want: 2,
		},
		{
			name: "lower id is deterministic final tie break",
			members: []similarityClusterMember{
				{ID: 9, Width: 2000, Height: 1000, FileSize: 2_000_000},
				{ID: 3, Width: 2000, Height: 1000, FileSize: 2_000_000},
			},
			want: 3,
		},
		{name: "empty", members: nil, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := preferredSimilarityClusterMember(test.members); got != test.want {
				t.Fatalf("preferredSimilarityClusterMember() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestSimilarityClusterScoreClampsCosineDistance(t *testing.T) {
	tests := []struct {
		distance float64
		want     float64
	}{
		{distance: 0, want: 1},
		{distance: 0.08, want: 0.92},
		{distance: 1, want: 0},
		{distance: 2, want: 0},
		{distance: -1, want: 1},
	}
	for _, test := range tests {
		if got := similarityClusterScore(test.distance); got != test.want {
			t.Fatalf("similarityClusterScore(%v) = %v, want %v", test.distance, got, test.want)
		}
	}
}
