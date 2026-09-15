package sqlite

import "context"

// ImageEmbeddingIDs returns the Image IDs currently present in the visual
// embedding index. Keeping this query in the embedding store lets higher-level
// cleanup tools operate only on media that can actually participate in KNN
// search instead of probing every library Image for an embedding.
func (s *VisualEmbeddingStore) ImageEmbeddingIDs(ctx context.Context) ([]int, error) {
	var ids []int
	if err := dbWrapper.Select(ctx, &ids, "SELECT image_id FROM image_embedding_vectors ORDER BY image_id"); err != nil {
		return nil, err
	}
	return ids, nil
}
