package sqlite

import (
	"database/sql"
	"math"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/stretchr/testify/require"
)

func TestNormalizeVisualEmbedding(t *testing.T) {
	embedding := make([]float32, VisualEmbeddingDimensions)
	embedding[0] = 3
	embedding[1] = 4

	normalized, err := normalizeVisualEmbedding(embedding)
	require.NoError(t, err)
	require.InDelta(t, 0.6, normalized[0], 1e-6)
	require.InDelta(t, 0.8, normalized[1], 1e-6)
	require.Equal(t, float32(3), embedding[0], "normalization must not mutate the caller's slice")

	var norm float64
	for _, value := range normalized {
		norm += float64(value) * float64(value)
	}
	require.InDelta(t, 1.0, math.Sqrt(norm), 1e-6)
}

func TestNormalizeVisualEmbeddingValidation(t *testing.T) {
	_, err := normalizeVisualEmbedding(make([]float32, VisualEmbeddingDimensions-1))
	require.Error(t, err)

	_, err = normalizeVisualEmbedding(make([]float32, VisualEmbeddingDimensions))
	require.Error(t, err)

	nonFinite := make([]float32, VisualEmbeddingDimensions)
	nonFinite[0] = float32(math.Inf(1))
	_, err = normalizeVisualEmbedding(nonFinite)
	require.Error(t, err)
}

func TestSQLiteVecCosineKNN(t *testing.T) {
	db, err := sql.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	defer db.Close()

	var version string
	require.NoError(t, db.QueryRow("SELECT vec_version()").Scan(&version))
	require.NotEmpty(t, version)

	_, err = db.Exec("CREATE VIRTUAL TABLE vectors USING vec0(embedding FLOAT[3] DISTANCE_METRIC=cosine)")
	require.NoError(t, err)

	vectors := map[int][]float32{
		1: {1, 0, 0},
		2: {0.9, 0.1, 0},
		3: {0, 1, 0},
	}
	for id, vector := range vectors {
		serialized, serializeErr := sqlite_vec.SerializeFloat32(vector)
		require.NoError(t, serializeErr)
		_, err = db.Exec("INSERT INTO vectors(rowid, embedding) VALUES (?, ?)", id, serialized)
		require.NoError(t, err)
	}

	queryVector, err := sqlite_vec.SerializeFloat32([]float32{1, 0, 0})
	require.NoError(t, err)
	rows, err := db.Query(`SELECT rowid, distance
FROM vectors
WHERE embedding MATCH ? AND k = 3
ORDER BY distance`, queryVector)
	require.NoError(t, err)
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		var distance float64
		require.NoError(t, rows.Scan(&id, &distance))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{1, 2, 3}, ids)
}

func TestSQLiteVecCosineBruteForceAllowsLargeLimit(t *testing.T) {
	db, err := sql.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE VIRTUAL TABLE vectors USING vec0(embedding FLOAT[3] DISTANCE_METRIC=cosine)")
	require.NoError(t, err)

	vectors := map[int][]float32{
		1: {1, 0, 0},
		2: {0.9, 0.1, 0},
		3: {0, 1, 0},
	}
	for id, vector := range vectors {
		serialized, serializeErr := sqlite_vec.SerializeFloat32(vector)
		require.NoError(t, serializeErr)
		_, err = db.Exec("INSERT INTO vectors(rowid, embedding) VALUES (?, ?)", id, serialized)
		require.NoError(t, err)
	}

	queryVector, err := sqlite_vec.SerializeFloat32([]float32{1, 0, 0})
	require.NoError(t, err)
	rows, err := db.Query(`SELECT rowid, vec_distance_cosine(embedding, ?) AS distance
FROM vectors
WHERE rowid != ?
ORDER BY distance
LIMIT ?`, queryVector, 1, sqliteVecKNNMaxK+1)
	require.NoError(t, err)
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		var distance float64
		require.NoError(t, rows.Scan(&id, &distance))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 3}, ids)
}
