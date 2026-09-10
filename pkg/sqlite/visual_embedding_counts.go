package sqlite

import "context"

func (s *VisualEmbeddingStore) CountImages(ctx context.Context) (int, error) {
	var count int
	if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM image_embedding_vectors"); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *VisualEmbeddingStore) CountScenes(ctx context.Context) (int, error) {
	var count int
	if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM scene_embedding_vectors"); err != nil {
		return 0, err
	}
	return count, nil
}
