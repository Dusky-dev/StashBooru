package sqlite

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestImagePHashClustersPrimaryFileOnly(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
CREATE TABLE images (id INTEGER PRIMARY KEY);
CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
CREATE TABLE files_fingerprints (file_id INTEGER, type TEXT, fingerprint INTEGER);
INSERT INTO images(id) VALUES (1), (2), (3), (4), (5), (6), (7), (8);
INSERT INTO images_files VALUES
(1,10,1), (2,20,1), (3,30,1), (4,40,1), (5,50,1),
(6,60,0), (7,70,1), (8,80,1), (8,81,0), (999,90,1);
INSERT INTO files_fingerprints VALUES
(10,'phash',0), (20,'phash',0), (30,'phash',1),
(40,'phash',7), (50,'phash',65535),
(60,'phash',0), (70,'source_phash',0),
(80,'phash',NULL), (81,'phash',0), (90,'phash',0);
`)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), dbKey, db)
	tests := []struct {
		distance int
		groups   [][]int
	}{
		{0, [][]int{{1, 2}, {3}, {4}, {5}}},
		{1, [][]int{{1, 2, 3}, {4}, {5}}},
		{2, [][]int{{1, 2, 3, 4}, {5}}},
	}
	for _, tt := range tests {
		clusters, indexed, err := ImagePHashClusters(ctx, tt.distance)
		require.NoError(t, err)
		require.Equal(t, 5, indexed, "only primary-file Image pHashes count")
		require.Equal(t, tt.groups, clusters)
	}

	_, _, err = ImagePHashClusters(ctx, -1)
	require.ErrorContains(t, err, "between 0 and")
	_, _, err = ImagePHashClusters(ctx, 9)
	require.ErrorContains(t, err, "between 0 and")
}

func TestImagePHashClustersEmptyAndCancelled(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
CREATE TABLE images (id INTEGER PRIMARY KEY);
CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
CREATE TABLE files_fingerprints (file_id INTEGER, type TEXT, fingerprint INTEGER);
INSERT INTO images(id) VALUES (1);
INSERT INTO images_files VALUES (1,10,1);
INSERT INTO files_fingerprints VALUES (10,'source_phash',0);
`)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), dbKey, db)
	clusters, indexed, err := ImagePHashClusters(ctx, 5)
	require.NoError(t, err)
	require.Zero(t, indexed)
	require.Empty(t, clusters)

	_, err = db.Exec("INSERT INTO files_fingerprints VALUES (10,'phash', 0)")
	require.NoError(t, err)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = ImagePHashClusters(cancelled, 5)
	require.Error(t, err)
}
