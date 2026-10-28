package sqlite

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
)

// ImageEmbeddingVector is an existing Image's stored EVA02 vector.
type ImageEmbeddingVector struct {
	ID     int
	Values []float32
}

type imageEmbeddingBlob struct {
	ID        int    `db:"id"`
	Embedding []byte `db:"embedding"`
}

// ImageEmbeddingVectors loads indexed Images in one query rather than issuing
// one sqlite-vec nearest-neighbor query per Image. The join excludes orphaned
// vectors even before an older database has run migration 102.
func (s *VisualEmbeddingStore) ImageEmbeddingVectors(ctx context.Context) ([]ImageEmbeddingVector, error) {
	var rows []imageEmbeddingBlob
	const query = `SELECT v.rowid AS id, v.embedding AS embedding
FROM image_embedding_vectors AS v
JOIN images AS i ON i.id = v.rowid
ORDER BY v.rowid`
	if err := dbWrapper.Select(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("loading Image embeddings: %w", err)
	}

	vectors := make([]ImageEmbeddingVector, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(row.Embedding) != 4*VisualEmbeddingDimensions {
			return nil, fmt.Errorf("Image %d embedding has %d bytes, expected %d", row.ID, len(row.Embedding), 4*VisualEmbeddingDimensions)
		}
		values := make([]float32, VisualEmbeddingDimensions)
		for i := range values {
			values[i] = math.Float32frombits(binary.LittleEndian.Uint32(row.Embedding[4*i:]))
			if math.IsNaN(float64(values[i])) || math.IsInf(float64(values[i]), 0) {
				return nil, fmt.Errorf("Image %d embedding contains a non-finite value", row.ID)
			}
		}
		vectors = append(vectors, ImageEmbeddingVector{ID: row.ID, Values: values})
	}
	return vectors, nil
}
