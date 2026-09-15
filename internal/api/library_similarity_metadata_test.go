package api

import (
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
	if result[0].MeanSimilarity != 0.9 {
		t.Fatalf("unexpected mean similarity %v", result[0].MeanSimilarity)
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
