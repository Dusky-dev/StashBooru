package sqlite

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestCopyrightMergeTransfersMediaPrimaryAndHierarchy(t *testing.T) {
	ctx, tx, store := copyrightTestContext(t)
	for _, input := range []models.CopyrightCreateInput{
		{Name: "Root"}, {Name: "Old series", ParentIDs: []string{"1"}},
		{Name: "Canonical series", ParentIDs: []string{"1"}},
		{Name: "Subseries", ParentIDs: []string{"2"}},
	} {
		_, err := store.Create(ctx, input)
		require.NoError(t, err)
	}
	_, err := tx.Exec(`INSERT INTO images VALUES(1),(2);
INSERT INTO scenes VALUES(1),(2);
INSERT INTO performers VALUES(1);
INSERT INTO tags VALUES(1);
INSERT INTO images_copyrights VALUES(1,2),(2,3);
INSERT INTO scenes_copyrights VALUES(1,2),(2,3);
INSERT INTO performers_copyrights VALUES(1,2);
INSERT INTO copyrights_tags VALUES(2,1);
INSERT INTO image_primary_copyrights VALUES(1,2),(2,3);
INSERT INTO scene_primary_copyrights VALUES(1,2),(2,3);`)
	require.NoError(t, err)
	require.NoError(t, store.Merge(ctx, 2, 3))
	deleted, err := store.Find(ctx, 2)
	require.NoError(t, err)
	require.Nil(t, deleted)
	for _, item := range []int{1, 2} {
		ids, err := store.FindByImageID(ctx, item)
		require.NoError(t, err)
		require.Len(t, ids, 1)
		require.Equal(t, 3, ids[0].ID)
		primary, err := store.PrimaryImageCopyrightID(ctx, item)
		require.NoError(t, err)
		require.NotNil(t, primary)
		require.Equal(t, 3, *primary)
	}
	primaryScene, err := store.PrimarySceneCopyrightID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 3, *primaryScene)
	performers, err := store.FindPerformerIDs(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, []int{1}, performers)
	tags, err := store.GetTagIDs(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, []int{1}, tags)
	parents, err := store.FindParents(ctx, 4)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	require.Equal(t, 3, parents[0].ID)
	canonical, err := store.Find(ctx, 3)
	require.NoError(t, err)
	require.Contains(t, canonical.Aliases, "Old series")
}

func TestCopyrightMergeRejectsConflictingDescription(t *testing.T) {
	ctx, _, store := copyrightTestContext(t)
	a := "Source information"
	b := "Different information"
	for _, input := range []models.CopyrightCreateInput{
		{Name: "Source", Description: &a}, {Name: "Target", Description: &b},
	} {
		_, err := store.Create(ctx, input)
		require.NoError(t, err)
	}
	err := store.Merge(ctx, 1, 2)
	require.ErrorContains(t, err, "descriptions conflict")
	source, err := store.Find(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, source)
}
