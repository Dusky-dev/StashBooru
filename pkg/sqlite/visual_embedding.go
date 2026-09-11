package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

const (
	VisualEmbeddingDimensions = 1024
	VisualEmbeddingModel      = "deepghs/wd14_tagger_with_embeddings:SmilingWolf/wd-eva02-large-tagger-v3"

	visualEmbeddingEntityImage = "image"
	visualEmbeddingEntityScene = "scene"

	// sqlite-vec caps vec0 KNN queries at k=4096. Similarity sorting can
	// legitimately request more rows than that, so larger searches fall back
	// to an exact cosine-distance scan instead of truncating the result set.
	sqliteVecKNNMaxK = 4096
)

type VisualSimilarityMatch struct {
	ID       int     `db:"id"`
	Distance float64 `db:"distance"`
}

type VisualEmbeddingStore struct{}

var VisualEmbeddings = &VisualEmbeddingStore{}

func normalizeVisualEmbedding(embedding []float32) ([]float32, error) {
	if len(embedding) != VisualEmbeddingDimensions {
		return nil, fmt.Errorf("visual embedding has %d dimensions, expected %d", len(embedding), VisualEmbeddingDimensions)
	}

	var normSquared float64
	for _, value := range embedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, errors.New("visual embedding contains a non-finite value")
		}
		normSquared += float64(value) * float64(value)
	}
	if normSquared == 0 {
		return nil, errors.New("visual embedding has zero norm")
	}

	norm := float32(math.Sqrt(normSquared))
	normalized := make([]float32, len(embedding))
	for i, value := range embedding {
		normalized[i] = value / norm
	}
	return normalized, nil
}

func serializeVisualEmbedding(embedding []float32) ([]byte, error) {
	normalized, err := normalizeVisualEmbedding(embedding)
	if err != nil {
		return nil, err
	}
	return sqlite_vec.SerializeFloat32(normalized)
}

func (s *VisualEmbeddingStore) UpsertImage(ctx context.Context, imageID int, embedding []float32, sourceKey string) error {
	return s.upsert(ctx, visualEmbeddingEntityImage, "image_embedding_vectors", imageID, embedding, sourceKey)
}

func (s *VisualEmbeddingStore) UpsertScene(ctx context.Context, sceneID int, embedding []float32, sourceKey string) error {
	return s.upsert(ctx, visualEmbeddingEntityScene, "scene_embedding_vectors", sceneID, embedding, sourceKey)
}

func (s *VisualEmbeddingStore) upsert(ctx context.Context, entityType, vectorTable string, entityID int, embedding []float32, sourceKey string) error {
	if entityID <= 0 {
		return fmt.Errorf("invalid %s id %d", entityType, entityID)
	}

	serialized, err := serializeVisualEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("serializing %s %d visual embedding: %w", entityType, entityID, err)
	}

	if _, err := dbWrapper.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE rowid = ?", vectorTable), entityID); err != nil {
		return fmt.Errorf("deleting previous %s %d visual embedding: %w", entityType, entityID, err)
	}
	if _, err := dbWrapper.Exec(ctx, fmt.Sprintf("INSERT INTO %s(rowid, embedding) VALUES (?, ?)", vectorTable), entityID, serialized); err != nil {
		return fmt.Errorf("inserting %s %d visual embedding: %w", entityType, entityID, err)
	}

	const upsertSource = `INSERT INTO visual_embedding_sources(entity_type, entity_id, model, source_key, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(entity_type, entity_id) DO UPDATE SET
    model = excluded.model,
    source_key = excluded.source_key,
    updated_at = CURRENT_TIMESTAMP`
	if _, err := dbWrapper.Exec(ctx, upsertSource, entityType, entityID, VisualEmbeddingModel, sourceKey); err != nil {
		return fmt.Errorf("updating %s %d visual embedding metadata: %w", entityType, entityID, err)
	}

	return nil
}

func (s *VisualEmbeddingStore) DeleteImage(ctx context.Context, imageID int) error {
	return s.delete(ctx, visualEmbeddingEntityImage, "image_embedding_vectors", imageID)
}

func (s *VisualEmbeddingStore) DeleteScene(ctx context.Context, sceneID int) error {
	return s.delete(ctx, visualEmbeddingEntityScene, "scene_embedding_vectors", sceneID)
}

func (s *VisualEmbeddingStore) delete(ctx context.Context, entityType, vectorTable string, entityID int) error {
	if _, err := dbWrapper.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE rowid = ?", vectorTable), entityID); err != nil {
		return err
	}
	_, err := dbWrapper.Exec(ctx, "DELETE FROM visual_embedding_sources WHERE entity_type = ? AND entity_id = ?", entityType, entityID)
	return err
}

func (s *VisualEmbeddingStore) HasCurrentImage(ctx context.Context, imageID int, sourceKey string) (bool, error) {
	return s.hasCurrent(ctx, visualEmbeddingEntityImage, imageID, sourceKey)
}

func (s *VisualEmbeddingStore) HasCurrentScene(ctx context.Context, sceneID int, sourceKey string) (bool, error) {
	return s.hasCurrent(ctx, visualEmbeddingEntityScene, sceneID, sourceKey)
}

func (s *VisualEmbeddingStore) hasCurrent(ctx context.Context, entityType string, entityID int, sourceKey string) (bool, error) {
	var count int
	const query = `SELECT COUNT(*) FROM visual_embedding_sources
WHERE entity_type = ? AND entity_id = ? AND model = ? AND source_key = ?`
	if err := dbWrapper.Get(ctx, &count, query, entityType, entityID, VisualEmbeddingModel, sourceKey); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *VisualEmbeddingStore) FindSimilarImages(ctx context.Context, referenceID, limit int) ([]VisualSimilarityMatch, error) {
	return s.findSimilar(ctx, "image_embedding_vectors", referenceID, limit)
}

func (s *VisualEmbeddingStore) FindSimilarScenes(ctx context.Context, referenceID, limit int) ([]VisualSimilarityMatch, error) {
	return s.findSimilar(ctx, "scene_embedding_vectors", referenceID, limit)
}

func (s *VisualEmbeddingStore) findSimilar(ctx context.Context, vectorTable string, referenceID, limit int) ([]VisualSimilarityMatch, error) {
	if referenceID <= 0 {
		return nil, fmt.Errorf("invalid visual similarity reference id %d", referenceID)
	}
	if limit <= 0 {
		return []VisualSimilarityMatch{}, nil
	}

	var reference []byte
	if err := dbWrapper.Get(ctx, &reference, fmt.Sprintf("SELECT embedding FROM %s WHERE rowid = ?", vectorTable), referenceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("visual similarity reference %d has no embedding", referenceID)
		}
		return nil, err
	}

	// The vec0 MATCH query includes the reference row itself, so request one
	// extra candidate while it fits within sqlite-vec's hard k limit.
	if limit+1 <= sqliteVecKNNMaxK {
		query := fmt.Sprintf(`SELECT rowid AS id, distance
FROM %s
WHERE embedding MATCH ? AND k = ?
ORDER BY distance`, vectorTable)

		var candidates []VisualSimilarityMatch
		if err := dbWrapper.Select(ctx, &candidates, query, reference, limit+1); err != nil {
			return nil, err
		}

		matches := make([]VisualSimilarityMatch, 0, limit)
		for _, candidate := range candidates {
			if candidate.ID == referenceID {
				continue
			}
			matches = append(matches, candidate)
			if len(matches) == limit {
				break
			}
		}
		return matches, nil
	}

	// sqlite-vec rejects k values above 4096. For large libraries we still
	// need the complete ordering so filters and pagination remain correct;
	// use its scalar cosine-distance function for an exact brute-force scan.
	query := fmt.Sprintf(`SELECT rowid AS id, vec_distance_cosine(embedding, ?) AS distance
FROM %s
WHERE rowid != ?
ORDER BY distance
LIMIT ?`, vectorTable)

	var matches []VisualSimilarityMatch
	if err := dbWrapper.Select(ctx, &matches, query, reference, referenceID, limit); err != nil {
		return nil, err
	}
	return matches, nil
}
