package sqlite

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func copyrightTestContext(t *testing.T) (context.Context, *sqlx.Tx, *CopyrightStore) {
	t.Helper()
	db, err := sqlx.Open(sqlite3Driver, ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`PRAGMA foreign_keys = ON;
CREATE TABLE images (id INTEGER PRIMARY KEY);
CREATE TABLE scenes (id INTEGER PRIMARY KEY);
CREATE TABLE performers (id INTEGER PRIMARY KEY);
CREATE TABLE studios (id INTEGER PRIMARY KEY);`)
	require.NoError(t, err)
	for _, path := range []string{"migrations/88_copyrights.up.sql", "migrations/90_copyright_media.up.sql"} {
		schema, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = db.Exec(string(schema))
		require.NoError(t, err)
	}
	tx, err := db.Beginx()
	require.NoError(t, err)
	t.Cleanup(func() { tx.Rollback() })
	return context.WithValue(context.Background(), txnKey, tx), tx, NewCopyrightStore()
}

func TestCopyrightHierarchyValidation(t *testing.T) {
	ctx, tx, store := copyrightTestContext(t)
	for _, name := range []string{"Shueisha", "My Hero Academia", "Vigilantes", "Other parent"} {
		_, err := store.Create(ctx, models.CopyrightCreateInput{Name: name})
		require.NoError(t, err)
	}
	_, err := store.Update(ctx, models.CopyrightUpdateInput{ID: "2", ParentIDs: []string{"1"}})
	require.NoError(t, err)
	_, err = store.Update(ctx, models.CopyrightUpdateInput{ID: "3", ParentIDs: []string{"2", "4", "4"}})
	require.NoError(t, err)
	parents, err := store.FindParents(ctx, 3)
	require.NoError(t, err)
	require.Len(t, parents, 2, "multi-parent hierarchies remain supported")

	for _, input := range []models.CopyrightUpdateInput{
		{ID: "1", ParentIDs: []string{"3"}},
		{ID: "3", ChildIDs: []string{"1"}},
		{ID: "1", ParentIDs: []string{"1"}},
		{ID: "1", ChildIDs: []string{"1"}},
		{ID: "1", ParentIDs: []string{"999"}},
		{ID: "1", ParentIDs: []string{"2junk"}},
		{ID: "1", ParentIDs: []string{"0"}},
	} {
		name := "must not be saved"
		input.Name = &name
		_, err := store.Update(ctx, input)
		require.Error(t, err)
		itemID, _ := strconv.Atoi(input.ID)
		item, err := store.Find(ctx, itemID)
		require.NoError(t, err)
		require.NotEqual(t, name, item.Name, "validation must precede metadata writes")
	}

	// A new node can also close a cycle between existing ancestors/descendants.
	_, err = tx.Exec("SAVEPOINT create_attempt")
	require.NoError(t, err)
	_, err = store.Create(ctx, models.CopyrightCreateInput{Name: "Invalid bridge", ParentIDs: []string{"3"}, ChildIDs: []string{"1"}})
	require.ErrorContains(t, err, "cycle")
	_, err = tx.Exec("ROLLBACK TO create_attempt")
	require.NoError(t, err)
	missing, err := store.FindByName(ctx, "Invalid bridge", false)
	require.NoError(t, err)
	require.Nil(t, missing)

	// Reverse 1 -> 2 in one edit. Validating either side against the old graph
	// would incorrectly reject this operation.
	_, err = store.Update(ctx, models.CopyrightUpdateInput{ID: "1", ParentIDs: []string{"2"}, ChildIDs: []string{}})
	require.NoError(t, err)
	parents, err = store.FindParents(ctx, 1)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	require.Equal(t, 2, parents[0].ID)
	children, err := store.FindChildren(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, children)
	_, err = store.Update(ctx, models.CopyrightUpdateInput{ID: "3", ParentIDs: []string{"1", "4"}})
	require.NoError(t, err)
	_, err = tx.Exec("INSERT INTO images VALUES (100); INSERT INTO images_copyrights VALUES (100, 1), (100, 3)")
	require.NoError(t, err)
	require.NoError(t, store.Destroy(ctx, 1))
	child, err := store.Find(ctx, 3)
	require.NoError(t, err)
	require.NotNil(t, child)
	parents, err = store.FindParents(ctx, 3)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	require.Equal(t, 4, parents[0].ID)
	var count int
	require.NoError(t, tx.Get(&count, "SELECT COUNT(*) FROM images WHERE id = 100"))
	require.Equal(t, 1, count, "deleting a Copyright must not delete media")
}

func TestCopyrightDirectorySorting(t *testing.T) {
	ctx, tx, store := copyrightTestContext(t)
	for _, fixture := range []struct{ name, sortName string }{{"Zulu", "same"}, {"Alpha", "SAME"}, {"Beta", ""}, {"Omega", "aardvark"}} {
		_, err := store.Create(ctx, models.CopyrightCreateInput{Name: fixture.name, SortName: &fixture.sortName})
		require.NoError(t, err)
	}
	_, err := tx.Exec(`INSERT INTO images VALUES (1), (2), (3);
INSERT INTO images_copyrights VALUES (1, 1), (2, 1), (3, 2);
INSERT INTO scenes VALUES (1), (2), (3);
INSERT INTO scenes_copyrights VALUES (1, 1), (1, 2), (2, 2), (3, 2);
INSERT INTO performers VALUES (1), (2);
INSERT INTO performers_copyrights VALUES (1, 3), (2, 3), (1, 4);
UPDATE copyrights SET created_at = '2026-01-01 00:00:00', updated_at = '2026-01-01 00:00:00';`)
	require.NoError(t, err)
	for _, tt := range []struct {
		sort string
		dir  models.SortDirectionEnum
		want []int
	}{
		{"name", models.SortDirectionEnumAsc, []int{2, 3, 4, 1}},
		{"sort_name", models.SortDirectionEnumAsc, []int{4, 3, 1, 2}},
		{"image_count", models.SortDirectionEnumDesc, []int{1, 2, 3, 4}},
		{"scene_count", models.SortDirectionEnumDesc, []int{2, 1, 3, 4}},
		{"performer_count", models.SortDirectionEnumDesc, []int{3, 4, 1, 2}},
		{"created_at", models.SortDirectionEnumAsc, []int{1, 2, 3, 4}},
		{"updated_at", models.SortDirectionEnumDesc, []int{1, 2, 3, 4}},
	} {
		t.Run(tt.sort, func(t *testing.T) {
			size := 2
			var got []int
			for page := 1; page <= 2; page++ {
				items, total, err := store.Query(ctx, &models.FindFilterType{Sort: &tt.sort, Direction: &tt.dir, Page: &page, PerPage: &size})
				require.NoError(t, err)
				require.Equal(t, 4, total)
				for _, item := range items {
					got = append(got, item.ID)
				}
			}
			require.Equal(t, tt.want, got)
		})
	}
	// Searching still applies before count sorting and pagination.
	q, sortName := "Alpha", "image_count"
	items, total, err := store.Query(ctx, &models.FindFilterType{Q: &q, Sort: &sortName})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 2, items[0].ID)
}
