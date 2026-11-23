//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

// Ignored records are hidden from lists, never added again by downloads or
// tagging, and marked in review.
func TestIgnoredMetadataIsHiddenAndNeverAddedAgain(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	repo := rs.repository
	ctx := context.Background()
	var ignoredTag, keptTag models.Tag
	var character models.Performer
	var copyright *models.Copyright
	ignore := true
	require.NoError(t, repo.WithTxn(ctx, func(ctx context.Context) error {
		ignoredTag, keptTag = models.NewTag(), models.NewTag()
		ignoredTag.Name, ignoredTag.Ignored = "highres", true
		keptTag.Name = "smile"
		for _, tag := range []*models.Tag{&ignoredTag, &keptTag} {
			if err := repo.Tag.Create(ctx, &models.CreateTagInput{Tag: tag}); err != nil {
				return err
			}
		}
		character = models.NewPerformer()
		character.Name, character.Ignored = "Background Person", true
		if err := repo.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &character}); err != nil {
			return err
		}
		var err error
		copyright, err = repo.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Original", Ignored: &ignore})
		return err
	}))

	// Downloads and tagging skip ignored records and create nothing new.
	var resolved taggingResolvedEntities
	require.NoError(t, repo.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		resolved, err = ensureBooruImportMetadata(ctx, repo, booru.Defaults()[0], []booru.Tag{
			{Name: "highres", Category: "general"},
			{Name: "smile", Category: "general"},
			{Name: "Background Person", Category: "character"},
			{Name: "original", Category: "copyright"},
		})
		return err
	}))
	require.Equal(t, []int{keptTag.ID}, resolved.TagIDs)
	require.Empty(t, resolved.CharacterIDs)
	require.Empty(t, resolved.CharacterPredictions)
	require.Empty(t, resolved.CopyrightIDs)
	require.Zero(t, resolved.CreatedCharacters+resolved.CreatedCopyrights+resolved.CreatedTags)

	// Review marks ignored targets so they can be hidden.
	require.NoError(t, repo.WithReadTxn(ctx, func(ctx context.Context) error {
		enriched, err := enrichNativeCamiePredictionTargetsInTxn(ctx, repo, []camietagger.Tag{
			{Name: "highres", Category: "general"},
			{Name: "smile", Category: "general"},
			{Name: "Background Person", Category: "character"},
		})
		require.NoError(t, err)
		require.True(t, enriched[0].Ignored)
		require.False(t, enriched[1].Ignored)
		require.True(t, enriched[2].Ignored)
		return nil
	}))

	// Native Auto Tag never matches ignored records.
	require.NoError(t, repo.WithReadTxn(ctx, func(ctx context.Context) error {
		tags, err := repo.Tag.QueryForAutoTag(ctx, []string{"highres", "smile"})
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "smile", tags[0].Name)
		performers, err := repo.Performer.QueryForAutoTag(ctx, []string{"background"})
		require.NoError(t, err)
		require.Empty(t, performers)
		return nil
	}))

	// Lists hide ignored records unless asked for them.
	r := &Resolver{repository: repo}
	query := r.Query().(*queryResolver)
	tags, err := query.FindTags(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, tags.Count)
	tags, err = query.FindTags(ctx, &models.TagFilterType{Ignored: &ignore}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, tags.Count)
	require.Equal(t, "highres", tags.Tags[0].Name)
	copyrights, err := query.FindCopyrights(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.Zero(t, copyrights.Count)
	copyrights, err = query.FindCopyrights(ctx, nil, nil, &ignore)
	require.NoError(t, err)
	require.Equal(t, copyright.ID, copyrights.Copyrights[0].ID)
	performers, err := query.FindPerformers(ctx, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Zero(t, performers.Count)
}
