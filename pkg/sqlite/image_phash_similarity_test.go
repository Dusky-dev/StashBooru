package sqlite

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestImagePHashReferenceSearch(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
CREATE TABLE images (id INTEGER PRIMARY KEY, title TEXT);
CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
CREATE TABLE files_fingerprints (file_id INTEGER, type TEXT, fingerprint BLOB);
INSERT INTO images VALUES (1, 'reference'), (2, 'match'), (3, 'match'), (4, 'match'),
 (5, 'match'), (6, 'match'), (7, 'excluded'), (8, 'match');
INSERT INTO images_files VALUES (1, 10, 1), (2, 20, 1), (3, 30, 1), (4, 40, 1),
 (5, 50, 1), (5, 51, 0), (6, 60, 1), (7, 70, 1), (8, 80, 1);
INSERT INTO files_fingerprints VALUES
 (10, 'phash', 0), (20, 'phash', 0), (30, 'phash', 1), (40, 'phash', 2),
 (50, 'source_phash', 0), (51, 'phash', 0), (60, 'phash', 7), (70, 'phash', 0);
`)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), dbKey, db)
	query := imageRepository.newQuery()
	query.from = imageTable
	query.columns = []string{"images.id"}
	query.addWhere("images.title = ?")
	query.whereArgs = []interface{}{"match"} // excludes the reference itself
	ref, page, size := 1, 1, 2
	filter := &models.FindFilterType{Page: &page, PerPage: &size}
	opts := &pHashSimilarityOptions{ReferenceID: &ref, Distance: 1, Method: "phash"}
	search := func() ([]int, int, error) {
		// No embedding tables exist: this also checks method dispatch.
		return findPHashSimilarityIDs(ctx, query, imagesFilesTable, imageIDColumn, filter, opts)
	}

	ids, count, err := search()
	require.NoError(t, err)
	require.Equal(t, []int{2, 3}, ids)
	require.Equal(t, 3, count)
	page = 2
	ids, count, err = search()
	require.NoError(t, err)
	require.Equal(t, []int{4}, ids) // same distance: stable ID order across pages
	require.Equal(t, 3, count)
	page = 3
	ids, count, err = search()
	require.NoError(t, err)
	require.Empty(t, ids)
	require.Equal(t, 3, count)

	page, size, opts.Distance = 1, -1, 0
	ids, count, err = search()
	require.NoError(t, err)
	require.Equal(t, []int{2}, ids)
	require.Equal(t, 1, count)
	query.whereClauses, query.whereArgs = nil, nil
	ids, _, err = search()
	require.NoError(t, err)
	require.Equal(t, []int{2, 7}, ids) // reference is always excluded

	// Source lineage and secondary-file hashes cannot stand in for a missing
	// primary hash. Missing entities and missing fingerprints are actionable errors.
	for _, missing := range []int{5, 8, 999} {
		ref = missing
		_, _, err = search()
		require.ErrorContains(t, err, "no current primary-file pHash")
	}

	// Updating the active file fingerprint takes effect without an embedding rebuild.
	ref = 1
	_, err = db.Exec("UPDATE files_fingerprints SET fingerprint = 7 WHERE file_id = 20")
	require.NoError(t, err)
	ids, _, err = search()
	require.NoError(t, err)
	require.Equal(t, []int{7}, ids)

	// The schema permits multiple fingerprints of a type. Return each image once.
	_, err = db.Exec("INSERT INTO files_fingerprints VALUES (70, 'phash', 1)")
	require.NoError(t, err)
	opts.Distance = 1
	ids, count, err = search()
	require.NoError(t, err)
	require.Equal(t, []int{7, 3, 4}, ids)
	require.Equal(t, 3, count)
}
