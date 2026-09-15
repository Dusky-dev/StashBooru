package api

import (
	"sort"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
)

type librarySimilarityMetadataVote struct {
	NeighborID int
	Similarity float64
	Prediction camietagger.Tag
}

type librarySimilarityMetadataConsensus struct {
	Prediction     camietagger.Tag
	Votes          int
	MeanSimilarity float64
}

func librarySimilarityMetadataKey(prediction camietagger.Tag) string {
	prediction = normalizeCamiePrediction(prediction)
	return prediction.Category + "\x00" + strings.ToLower(strings.TrimSpace(prediction.Name))
}

// aggregateLibrarySimilarityMetadata converts metadata from nearest indexed
// library neighbors into review-only consensus suggestions. One neighbor gets
// at most one vote for a canonical prediction, preventing tag duplication on a
// single media item from inflating consensus.
func aggregateLibrarySimilarityMetadata(votes []librarySimilarityMetadataVote, minVotes int) []librarySimilarityMetadataConsensus {
	if minVotes < 1 {
		minVotes = 1
	}

	type aggregate struct {
		prediction camietagger.Tag
		neighbors  map[int]struct{}
		total      float64
	}
	byKey := make(map[string]*aggregate)
	for _, vote := range votes {
		if vote.NeighborID <= 0 || vote.Similarity < 0 || vote.Similarity > 1 {
			continue
		}
		prediction := normalizeCamiePrediction(vote.Prediction)
		key := librarySimilarityMetadataKey(prediction)
		if strings.TrimSpace(prediction.Name) == "" || strings.TrimSpace(prediction.Category) == "" {
			continue
		}
		entry := byKey[key]
		if entry == nil {
			entry = &aggregate{prediction: prediction, neighbors: make(map[int]struct{})}
			byKey[key] = entry
		}
		if _, duplicate := entry.neighbors[vote.NeighborID]; duplicate {
			continue
		}
		entry.neighbors[vote.NeighborID] = struct{}{}
		entry.total += vote.Similarity
	}

	result := make([]librarySimilarityMetadataConsensus, 0, len(byKey))
	for _, entry := range byKey {
		count := len(entry.neighbors)
		if count < minVotes {
			continue
		}
		prediction := entry.prediction
		prediction.Source = "library-similarity"
		result = append(result, librarySimilarityMetadataConsensus{
			Prediction:     prediction,
			Votes:          count,
			MeanSimilarity: entry.total / float64(count),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Votes != result[j].Votes {
			return result[i].Votes > result[j].Votes
		}
		if result[i].MeanSimilarity != result[j].MeanSimilarity {
			return result[i].MeanSimilarity > result[j].MeanSimilarity
		}
		return librarySimilarityMetadataKey(result[i].Prediction) < librarySimilarityMetadataKey(result[j].Prediction)
	})
	return result
}
