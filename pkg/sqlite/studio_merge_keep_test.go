//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

// Artist merges refuse conflicting profile fields unless the merge dialog
// already chose them; then the destination's values are kept.
func TestStudioMergeKeepingDestinationResolvesConflicts(t *testing.T) {
	runWithRollbackTxn(t, "artist merge keeps destination", func(t *testing.T, ctx context.Context) {
		create := func(name, details string, rating int) *models.Studio {
			studio := models.NewStudio()
			studio.Name, studio.Details, studio.Rating = name, details, &rating
			require.NoError(t, db.Studio.Create(ctx, &models.CreateStudioInput{Studio: &studio}))
			return &studio
		}
		source := create("Keep merge source", "source details", 20)
		target := create("Keep merge target", "target details", 80)

		require.ErrorContains(t, db.Studio.Merge(ctx, source.ID, target.ID), "conflict")
		require.NoError(t, db.Studio.MergeKeepingDestination(ctx, source.ID, target.ID))

		gone, err := db.Studio.Find(ctx, source.ID)
		require.NoError(t, err)
		require.Nil(t, gone)
		kept, err := db.Studio.Find(ctx, target.ID)
		require.NoError(t, err)
		require.Equal(t, "target details", kept.Details)
		require.Equal(t, 80, *kept.Rating)
		aliases, err := db.Studio.GetAliases(ctx, target.ID)
		require.NoError(t, err)
		require.Contains(t, aliases, "Keep merge source")
	})
}
