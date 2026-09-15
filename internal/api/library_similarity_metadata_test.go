package api

import (
	"math"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestAggregateLibrarySimilarityMetadata(t *testing.T) {
	votes := []librarySimilarityMetadataVote{
		{NeighborID: 1, Similarity: 0.95, Prediction: camietagger.Tag{Name: "Lana", Category: "character"}},
		{NeighborID: 2, Similarity: 0.85, Prediction: camietagger.Tag{Name: "lana", Category: "character"}},
		{NeighborID: 3, Similarity: 0.80, Prediction: camietagger.Tag{Name: "solo", Category: "general"}},
	}

	result := aggregateLibrarySimilarityMetadata(votes, 2)
	if len(result) != 1 {
		t.Fatalf("expected one consensus suggestion, got %d", len(result))
	}
	if result[0].Prediction.Name != "Lana" || result[0].Votes != 2 {
		t.Fatalf("unexpected consensus: %+v", result[0])
	}
	if result[0].Prediction.Source != "library-similarity" {
		t.Fatalf("unexpected provenance %q", result[0].Prediction.Source)
	}
	if math.Abs(result[0].MeanSimilarity-0.9) > 1e-9 {
		t.Fatalf("unexpected mean similarity %v", result[0].MeanSimilarity)
	}
	if math.Abs(result[0].Prediction.Score-0.9) > 1e-9 {
		t.Fatalf("expected prediction score to match mean similarity, got %v", result[0].Prediction.Score)
	}
}

func TestAggregateLibrarySimilarityMetadataDeduplicatesNeighborVotes(t *testing.T) {
	votes := []librarySimilarityMetadataVote{
		{NeighborID: 1, Similarity: 0.9, Prediction: camietagger.Tag{Name: "solo", Category: "general"}},
		{NeighborID: 1, Similarity: 0.9, Prediction: camietagger.Tag{Name: "solo", Category: "general"}},
		{NeighborID: 2, Similarity: 0.8, Prediction: camietagger.Tag{Name: "solo", Category: "general"}},
	}

	result := aggregateLibrarySimilarityMetadata(votes, 2)
	if len(result) != 1 || result[0].Votes != 2 {
		t.Fatalf("expected two unique neighbor votes, got %+v", result)
	}
}

func TestAggregateLibrarySimilarityMetadataRejectsInvalidVotes(t *testing.T) {
	votes := []librarySimilarityMetadataVote{
		{NeighborID: 0, Similarity: 0.95, Prediction: camietagger.Tag{Name: "bad-id", Category: "general"}},
		{NeighborID: 1, Similarity: -0.1, Prediction: camietagger.Tag{Name: "negative", Category: "general"}},
		{NeighborID: 2, Similarity: 1.1, Prediction: camietagger.Tag{Name: "too-high", Category: "general"}},
		{NeighborID: 3, Similarity: 0.8, Prediction: camietagger.Tag{Name: "", Category: "general"}},
	}

	if result := aggregateLibrarySimilarityMetadata(votes, 1); len(result) != 0 {
		t.Fatalf("expected invalid votes to be discarded, got %+v", result)
	}
}

func TestLibrarySimilarityScore(t *testing.T) {
	tests := []struct {
		name     string
		distance float64
		want     float64
	}{
		{name: "identical", distance: 0, want: 1},
		{name: "near", distance: 0.2, want: 0.8},
		{name: "orthogonal", distance: 1, want: 0},
		{name: "clamps negative cosine similarity", distance: 1.5, want: 0},
		{name: "clamps malformed negative distance", distance: -0.1, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := librarySimilarityScore(test.distance); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("librarySimilarityScore(%v) = %v, want %v", test.distance, got, test.want)
			}
		})
	}
}
