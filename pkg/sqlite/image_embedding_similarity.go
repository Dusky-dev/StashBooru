package sqlite

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

func findImageEmbeddingSimilarityIDs(
	ctx context.Context,
	query queryBuilder,
	findFilter *models.FindFilterType,
	referenceID int,
) ([]int, int, error) {
	const includeSortPagination = false
	baseSQL := query.toSQL(includeSortPagination)

	var candidates []int
	candidateSQL := fmt.Sprintf("SELECT id FROM (%s) AS visual_similarity_candidates", baseSQL)
	if err := dbWrapper.Select(ctx, &candidates, candidateSQL, query.allArgs()...); err != nil {
		return nil, 0, fmt.Errorf("querying visual similarity candidates: %w", err)
	}

	indexedCount, err := VisualEmbeddings.CountImages(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting indexed image embeddings: %w", err)
	}
	if indexedCount == 0 {
		return nil, 0, fmt.Errorf("no image visual embeddings are indexed; generate them in Settings > System > Visual Similarity")
	}

	matches, err := VisualEmbeddings.FindSimilarImages(ctx, referenceID, indexedCount)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"finding visual matches for image %d: %w; generate embeddings in Settings > System > Visual Similarity",
			referenceID,
			err,
		)
	}

	candidateSet := make(map[int]struct{}, len(candidates))
	for _, id := range candidates {
		candidateSet[id] = struct{}{}
	}

	ids := make([]int, 0, len(matches))
	for _, match := range matches {
		if _, ok := candidateSet[match.ID]; ok {
			ids = append(ids, match.ID)
		}
	}

	total := len(ids)
	return paginatePHashSimilarityIDs(ids, findFilter), total, nil
}
