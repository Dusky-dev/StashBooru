package sqlite

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestStorageEstimateVisualStackIDs(t *testing.T) {
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
CREATE TABLE visual_stacks (id INTEGER PRIMARY KEY);
CREATE TABLE visual_stack_members (stack_id INTEGER, image_id INTEGER, scene_id INTEGER);
INSERT INTO visual_stacks(id) VALUES (1), (2), (3), (4), (5);
INSERT INTO visual_stack_members VALUES
(1, 10, NULL), (1, 11, NULL),
(2, 12, NULL), (2, NULL, 3),
(3, 13, NULL),
(4, 14, NULL), (4, 15, NULL), (4, 16, NULL),
(5, 17, NULL), (5, 18, NULL);
`)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), dbKey, db)
	ids, err := StorageEstimateVisualStackIDs(ctx, 64)
	require.NoError(t, err)
	require.Equal(t, []int{1, 4, 5}, ids)
	ids, err = StorageEstimateVisualStackIDs(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, []int{1, 4}, ids)
	for _, limit := range []int{0, -1, 65} {
		_, err = StorageEstimateVisualStackIDs(ctx, limit)
		require.Error(t, err)
	}
}
