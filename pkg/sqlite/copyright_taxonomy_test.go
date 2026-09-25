package sqlite

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func createCopyrightFixture(t *testing.T, ctx interface {
	Done() <-chan struct{}
}, store *CopyrightStore) {
	t.Helper()
}

func TestCopyrightSubtreeCountsDeduplicateMultiParentPaths(t *testing.T) {
	ctx, tx, store := copyrightTestContext(t)

	for _, input := range []models.CopyrightCreateInput{
		{Name: "Root"},
		{Name: "Branch B", ParentIDs: []string{"1"}},
		{Name: "Branch C", ParentIDs: []string{"1"}},
		{Name: "Leaf D", ParentIDs: []string{"2", "3"}},
		{Name: "Other root"},
	} {
		_, err := store.Create(ctx, input)
		require.NoError(t, err)
	}

	_, err := tx.Exec(`
INSERT INTO images VALUES (1), (2), (3), (4), (5);
INSERT INTO images_copyrights VALUES
  (1, 2), (1, 4),
  (2, 3),
  (3, 4),
  (4, 5), (5, 5);
INSERT INTO scenes VALUES (1), (2);
INSERT INTO scenes_copyrights VALUES (1, 3), (1, 4), (2, 4);
INSERT INTO performers VALUES (1), (2);
INSERT INTO performers_copyrights VALUES (1, 2), (1, 4), (2, 4);`)
	require.NoError(t, err)

	count, err := store.ImageCountDepth(ctx, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	count, err = store.ImageCountDepth(ctx, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = store.ImageCountDepth(ctx, 1, -1)
	require.NoError(t, err)
	require.Equal(t, 3, count, "an image linked through multiple nodes in one branch must count once")

	ids, err := store.FindImageIDsDepth(ctx, 1, -1)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, ids)

	sceneCount, err := store.SceneCountDepth(ctx, 1, -1)
	require.NoError(t, err)
	require.Equal(t, 2, sceneCount)
	performerCount, err := store.PerformerCountDepth(ctx, 1, -1)
	require.NoError(t, err)
	require.Equal(t, 2, performerCount)

	sortName := "image_count"
	direction := models.SortDirectionEnumDesc
	items, total, err := store.Query(ctx, &models.FindFilterType{Sort: &sortName, Direction: &direction})
	require.NoError(t, err)
	require.Equal(t, 5, total)
	require.Len(t, items, 5)

	got := make([]int, 0, len(items))
	for _, item := range items {
		got = append(got, item.ID)
	}
	// Root and Branch C both see three distinct images; ties fall back to ID.
	// Branch B, Leaf D and Other root each see two.
	require.Equal(t, []int{1, 3, 2, 4, 5}, got)
}

func TestCopyrightTaxonomyOrderingRolesAndPrimary(t *testing.T) {
	ctx, tx, store := copyrightTestContext(t)

	for _, input := range []models.CopyrightCreateInput{
		{Name: "Root"},
		{Name: "Beta", ParentIDs: []string{"1"}},
		{Name: "Alpha", ParentIDs: []string{"1"}},
		{Name: "Leaf", ParentIDs: []string{"2", "3"}},
	} {
		_, err := store.Create(ctx, input)
		require.NoError(t, err)
	}

	children, err := store.FindOrderedChildren(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, []int{children[0].ID, children[1].ID})

	require.NoError(t, store.SetChildOrder(ctx, 1, []int{2, 3}))
	children, err = store.FindOrderedChildren(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []int{2, 3}, []int{children[0].ID, children[1].ID})
	require.Error(t, store.SetChildOrder(ctx, 1, []int{2, 2}))
	require.Error(t, store.SetChildOrder(ctx, 1, []int{2}))
	children, err = store.FindOrderedChildren(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []int{2, 3}, []int{children[0].ID, children[1].ID}, "invalid reorder attempts must preserve the last valid order")

	breadcrumb, err := store.Breadcrumb(ctx, 4)
	require.NoError(t, err)
	breadcrumbIDs := make([]int, 0, len(breadcrumb))
	for _, item := range breadcrumb {
		breadcrumbIDs = append(breadcrumbIDs, item.ID)
	}
	// Alpha sorts before Beta, so the canonical multi-parent breadcrumb is Root > Alpha > Leaf.
	require.Equal(t, []int{1, 3, 4}, breadcrumbIDs)

	require.NoError(t, store.SetStructuralRole(ctx, 1, " Publisher "))
	role, err := store.StructuralRole(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Publisher", role)
	require.NoError(t, store.SetStructuralRole(ctx, 1, "  "))
	role, err = store.StructuralRole(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, role)

	_, err = tx.Exec("INSERT INTO tags (id) VALUES (10)")
	require.NoError(t, err)
	require.NoError(t, store.SetTagStructuralRole(ctx, 10, " Genre "))
	role, err = store.TagStructuralRole(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, "Genre", role)
	require.NoError(t, store.SetTagStructuralRole(ctx, 10, ""))
	role, err = store.TagStructuralRole(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, role)

	_, err = tx.Exec(`
INSERT INTO images VALUES (20);
INSERT INTO images_copyrights VALUES (20, 2), (20, 3);
INSERT INTO scenes VALUES (30);
INSERT INTO scenes_copyrights VALUES (30, 2), (30, 3);`)
	require.NoError(t, err)

	primary := 2
	require.NoError(t, store.SetPrimaryImageCopyright(ctx, 20, &primary))
	ordered, err := store.FindByImageIDOrdered(ctx, 20)
	require.NoError(t, err)
	require.Equal(t, []int{2, 3}, []int{ordered[0].ID, ordered[1].ID}, "primary Copyright must sort first")

	invalidPrimary := 4
	require.Error(t, store.SetPrimaryImageCopyright(ctx, 20, &invalidPrimary), "primary Copyright must already be associated with the image")
	storedPrimary, err := store.PrimaryImageCopyrightID(ctx, 20)
	require.NoError(t, err)
	require.Nil(t, storedPrimary, "failed replacement must not leave a primary Copyright behind")

	primary = 3
	require.NoError(t, store.SetPrimarySceneCopyright(ctx, 30, &primary))
	orderedScenes, err := store.FindBySceneIDOrdered(ctx, 30)
	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, []int{orderedScenes[0].ID, orderedScenes[1].ID})

	require.NoError(t, store.SetImageCopyrights(ctx, 20, []int{2}))
	storedPrimary, err = store.PrimaryImageCopyrightID(ctx, 20)
	require.NoError(t, err)
	require.Nil(t, storedPrimary, "removing an association must cascade-delete the matching primary row")
}
