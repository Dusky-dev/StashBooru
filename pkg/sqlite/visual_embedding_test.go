package sqlite

import (
	"context"
	"database/sql"
	"math"
	"os"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
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

func TestImageEmbeddingIDsUsesVec0RowIDs(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec("CREATE VIRTUAL TABLE image_embedding_vectors USING vec0(embedding FLOAT[1024] DISTANCE_METRIC=cosine)")
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), dbKey, db)

	ids, err := VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Empty(t, ids, "an empty embedding index is valid")

	vector := make([]float32, VisualEmbeddingDimensions)
	vector[0] = 1
	serialized, err := sqlite_vec.SerializeFloat32(vector)
	require.NoError(t, err)
	for _, imageID := range []int{91, 2, 17} {
		_, err = db.Exec("INSERT INTO image_embedding_vectors(rowid, embedding) VALUES (?, ?)", imageID, serialized)
		require.NoError(t, err)
	}

	ids, err = VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{2, 17, 91}, ids, "vec0 rowids are the indexed image IDs in stable order")

	_, err = db.Exec("DELETE FROM image_embedding_vectors WHERE rowid = ?", 17)
	require.NoError(t, err)
	ids, err = VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{2, 91}, ids)
}

func TestImageEmbeddingCleanupMigrationRemovesOrphansAndTracksDeletes(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec("CREATE TABLE images (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO images(id) VALUES (2), (5)")
	require.NoError(t, err)

	// Use the actual schema migrations to cover sqlite-vec and the cleanup
	// trigger together, instead of testing a mocked image ID query.
	embeddingSchema, err := os.ReadFile("migrations/87_visual_embeddings.up.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(embeddingSchema))
	require.NoError(t, err)

	vector := make([]float32, VisualEmbeddingDimensions)
	vector[0] = 1
	serialized, err := sqlite_vec.SerializeFloat32(vector)
	require.NoError(t, err)
	for _, id := range []int{2, 9} {
		_, err = db.Exec("INSERT INTO image_embedding_vectors(rowid, embedding) VALUES (?, ?)", id, serialized)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO visual_embedding_sources(entity_type, entity_id, model, source_key)
VALUES ('image', ?, 'test', 'fixture')`, id)
		require.NoError(t, err)
	}
	// Unmatched metadata should be removed too; unrelated scene data stays.
	_, err = db.Exec(`INSERT INTO visual_embedding_sources(entity_type, entity_id, model, source_key)
VALUES ('image', 10, 'test', 'fixture'), ('scene', 9, 'test', 'fixture')`)
	require.NoError(t, err)

	cleanupSchema, err := os.ReadFile("migrations/102_image_embedding_cleanup.up.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(cleanupSchema))
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), dbKey, db)
	ids, err := VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{2}, ids)
	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM visual_embedding_sources WHERE entity_type = 'image'"))
	require.Equal(t, 1, count)
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM visual_embedding_sources WHERE entity_type = 'scene'"))
	require.Equal(t, 1, count)

	_, err = db.Exec("INSERT INTO image_embedding_vectors(rowid, embedding) VALUES (5, ?)", serialized)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO visual_embedding_sources(entity_type, entity_id, model, source_key)
VALUES ('image', 5, 'test', 'fixture')`)
	require.NoError(t, err)

	// Real Image deletes clean both record types atomically. A rolled-back
	// delete must also roll back its triggered vector and metadata cleanup.
	tx, err := db.Beginx()
	require.NoError(t, err)
	_, err = tx.Exec("DELETE FROM images WHERE id = 2")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	ids, err = VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{2, 5}, ids)

	_, err = db.Exec("DELETE FROM images WHERE id = 2")
	require.NoError(t, err)
	ids, err = VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []int{5}, ids)
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM visual_embedding_sources WHERE entity_type = 'image'"))
	require.Equal(t, 1, count)

	_, err = db.Exec("DELETE FROM images WHERE id = 5")
	require.NoError(t, err)
	ids, err = VisualEmbeddings.ImageEmbeddingIDs(ctx)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM visual_embedding_sources WHERE entity_type = 'image'"))
	require.Zero(t, count)
}
