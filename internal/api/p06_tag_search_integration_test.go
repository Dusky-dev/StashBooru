//go:build integration

package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestP06ProfileTagsAreSearchableOnImagesAndVideos(t *testing.T) {
	f := newP06SQLiteFixture(t)
	for _, video := range []bool{false, true} {
		for _, tag := range []*models.Tag{f.profileTag, f.parentTag} {
			tags := &models.HierarchicalMultiCriterionInput{
				Value: []string{strconv.Itoa(tag.ID)}, Modifier: models.CriterionModifierIncludes,
			}
			require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
				if video {
					result, err := f.repository.Scene.Query(ctx, models.SceneQueryOptions{
						QueryOptions: models.QueryOptions{Count: true}, SceneFilter: &models.SceneFilterType{Tags: tags},
					})
					require.NoError(t, err)
					require.Equal(t, []int{f.video.ID}, result.IDs, "effective Video Tag must match its native filter")
					require.Equal(t, 1, result.Count)
				} else {
					result, err := f.repository.Image.Query(ctx, models.ImageQueryOptions{
						QueryOptions: models.QueryOptions{Count: true}, ImageFilter: &models.ImageFilterType{Tags: tags},
					})
					require.NoError(t, err)
					require.Equal(t, []int{f.image.ID}, result.IDs, "effective Image Tag must match its native filter")
					require.Equal(t, 1, result.Count)
				}
				return nil
			}))
		}
		require.Empty(t, f.direct(t, video).tags, "searching must not materialize profile Tags")
	}
}

func p06SearchTags(t *testing.T, f *p06SQLiteFixture, video bool, tags *models.HierarchicalMultiCriterionInput, filter *models.FindFilterType) ([]int, int) {
	t.Helper()
	var ids []int
	var total int
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.repository.WithReadTxn(ctx, func(ctx context.Context) error {
		options := models.QueryOptions{Count: true, FindFilter: filter}
		if video {
			mediaFilter := &models.SceneFilterType{Tags: tags}
			result, err := f.repository.Scene.Query(ctx, models.SceneQueryOptions{QueryOptions: options, SceneFilter: mediaFilter})
			require.NoError(t, err)
			ids, total = result.IDs, result.Count
			count, err := f.repository.Scene.QueryCount(ctx, mediaFilter, filter)
			require.NoError(t, err)
			require.Equal(t, total, count, "separate count and paginated search must agree")
		} else {
			mediaFilter := &models.ImageFilterType{Tags: tags}
			result, err := f.repository.Image.Query(ctx, models.ImageQueryOptions{QueryOptions: options, ImageFilter: mediaFilter})
			require.NoError(t, err)
			ids, total = result.IDs, result.Count
			count, err := f.repository.Image.QueryCount(ctx, mediaFilter, filter)
			require.NoError(t, err)
			require.Equal(t, total, count, "separate count and paginated search must agree")
		}
		return nil
	}))
	return ids, total
}

func TestP06TagSearchAgreesWithEffectiveMembershipsAndCountsForEveryDefault(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, true)
	knownTags := []*models.Tag{f.tag, f.tagLeft, f.profileTag, f.parentTag}
	// Each profile gets its own Tag so another enabled domain cannot mask a
	// disabled Character, Artist or Copyright ancestor switch.
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		for _, owner := range []struct {
			domain string
			id     int
		}{
			{"character", f.character.ID}, {"character", f.characterRoot.ID},
			{"artist", f.artist.ID}, {"artist", f.artistRoot.ID},
			{"copyright", f.copyright.ID}, {"copyright", f.copyrightRoot.ID},
		} {
			tag := models.NewTag()
			tag.Name = fmt.Sprintf("%s %d profile", owner.domain, owner.id)
			tag.ParentIDs = models.NewRelatedIDs([]int{f.parentTag.ID})
			require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
			knownTags = append(knownTags, &tag)
			ids := &models.UpdateIDs{IDs: []int{tag.ID}, Mode: models.RelationshipUpdateModeSet}
			switch owner.domain {
			case "character":
				_, err := f.repository.Performer.UpdatePartial(ctx, owner.id, models.PerformerPartial{TagIDs: ids})
				require.NoError(t, err)
			case "artist":
				_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: owner.id, TagIDs: ids})
				require.NoError(t, err)
			case "copyright":
				require.NoError(t, f.repository.Copyright.UpdateTags(ctx, owner.id, ids.IDs))
			}
		}
		unrelated := models.NewTag()
		unrelated.Name = "Unrelated Tag"
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &unrelated}))
		knownTags = append(knownTags, &unrelated)
		return nil
	}))
	zero := 0
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprintf("defaults_%04b", mask), func(t *testing.T) {
			cfg := config.GetInstance()
			cfg.SetBool(config.AssociationInheritanceCharacters, mask&1 != 0)
			cfg.SetBool(config.AssociationInheritanceArtists, mask&2 != 0)
			cfg.SetBool(config.AssociationInheritanceCopyrights, mask&4 != 0)
			cfg.SetBool(config.AssociationInheritanceTags, mask&8 != 0)
			for _, video := range []bool{false, true} {
				effective := p06Memberships(f.effective(t, video), "tag")
				for _, tag := range knownTags {
					ids, total := p06SearchTags(t, f.p06SQLiteFixture, video, &models.HierarchicalMultiCriterionInput{
						Value: []string{strconv.Itoa(tag.ID)}, Modifier: models.CriterionModifierIncludes, Depth: &zero,
					}, nil)
					want := 0
					if slices.Contains(effective, tag.ID) {
						want = 2
					}
					require.Len(t, ids, want, "video=%t Tag=%s", video, tag.Name)
					require.Equal(t, want, total)
					resolver := (&Resolver{repository: f.repository}).Tag()
					var count int
					var err error
					if video {
						count, err = resolver.SceneCount(context.Background(), tag, &zero)
					} else {
						count, err = resolver.ImageCount(context.Background(), tag, &zero)
					}
					require.NoError(t, err)
					require.Equal(t, want, count, "Tag detail count must agree with search")
				}
				require.Equal(t, []int{f.tag.ID}, f.direct(t, video).tags)
			}
		})
	}
}

func TestP06TagSearchSharedSourcesExclusionsAndPagination(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, true)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		image := models.NewImage()
		require.NoError(t, f.repository.Image.Create(ctx, &models.CreateImageInput{Image: &image}))
		video := models.NewScene()
		require.NoError(t, f.repository.Scene.Create(ctx, &video, nil))
		return nil
	}))
	parent, child := strconv.Itoa(f.parentTag.ID), strconv.Itoa(f.profileTag.ID)
	for _, video := range []bool{false, true} {
		for _, criterion := range []*models.HierarchicalMultiCriterionInput{
			{Value: []string{parent}, Modifier: models.CriterionModifierIncludes},
			{Value: []string{parent, child}, Modifier: models.CriterionModifierIncludesAll},
		} {
			one, page, sortBy := 1, 1, "id"
			filter := &models.FindFilterType{PerPage: &one, Page: &page, Sort: &sortBy}
			first, total := p06SearchTags(t, f.p06SQLiteFixture, video, criterion, filter)
			require.Len(t, first, 1)
			require.Equal(t, 2, total, "diamonds, profile origins and primary/multi-Artist paths count each media once")
			page = 2
			second, total := p06SearchTags(t, f.p06SQLiteFixture, video, criterion, filter)
			require.Len(t, second, 1)
			require.Greater(t, second[0], first[0])
			require.Equal(t, 2, total)
			page = 3
			last, total := p06SearchTags(t, f.p06SQLiteFixture, video, criterion, filter)
			require.Empty(t, last)
			require.Equal(t, 2, total)
		}
		for _, criterion := range []*models.HierarchicalMultiCriterionInput{
			{Value: []string{parent}, Modifier: models.CriterionModifierExcludes},
			{Value: []string{parent}, Excludes: []string{child}, Modifier: models.CriterionModifierExcludes},
		} {
			ids, total := p06SearchTags(t, f.p06SQLiteFixture, video, criterion, nil)
			require.Len(t, ids, 1, "only the media without inherited Tags passes exclusion")
			require.Equal(t, 1, total)
		}
		ids, total := p06SearchTags(t, f.p06SQLiteFixture, video, &models.HierarchicalMultiCriterionInput{
			Value: []string{parent}, Excludes: []string{child}, Modifier: models.CriterionModifierIncludes,
		}, nil)
		require.Empty(t, ids)
		require.Zero(t, total)
	}
}

func TestP06TagSearchRecalculatesAfterProfileReparentingAndDetach(t *testing.T) {
	f := newP06SQLiteFixture(t)
	var oldParent, newParent models.Studio
	var newTag models.Tag
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		newTag = models.NewTag()
		newTag.Name = "New profile Tag"
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &newTag}))
		oldParent, newParent = models.NewStudio(), models.NewStudio()
		oldParent.Name, newParent.Name = "Old parent", "New parent"
		oldParent.TagIDs = models.NewRelatedIDs([]int{f.profileTag.ID})
		newParent.TagIDs = models.NewRelatedIDs([]int{newTag.ID})
		require.NoError(t, f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &oldParent}))
		require.NoError(t, f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &newParent}))
		_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{
			ID: f.artist.ID, ParentID: models.NewOptionalInt(oldParent.ID),
			TagIDs: &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet},
		})
		return err
	}))
	expect := func(tag *models.Tag, want int) {
		t.Helper()
		for _, video := range []bool{false, true} {
			ids, total := p06SearchTags(t, f, video, &models.HierarchicalMultiCriterionInput{
				Value: []string{strconv.Itoa(tag.ID)}, Modifier: models.CriterionModifierIncludes,
			}, nil)
			require.Len(t, ids, want)
			require.Equal(t, want, total)
		}
	}
	expect(f.profileTag, 1)
	expect(&newTag, 0)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: f.artist.ID, ParentID: models.NewOptionalInt(newParent.ID)})
		return err
	}))
	expect(f.profileTag, 0)
	expect(&newTag, 1)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{
			ID: newParent.ID, TagIDs: &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet},
		})
		return err
	}))
	expect(&newTag, 0)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{
			ID: f.artist.ID, TagIDs: &models.UpdateIDs{IDs: []int{newTag.ID}, Mode: models.RelationshipUpdateModeSet},
		})
		return err
	}))
	expect(&newTag, 1)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		require.NoError(t, f.repository.Image.UpdateTags(ctx, f.image.ID, []int{f.profileTag.ID}))
		_, err := f.repository.Scene.UpdatePartial(ctx, f.video.ID, models.ScenePartial{
			TagIDs: &models.UpdateIDs{IDs: []int{f.profileTag.ID}, Mode: models.RelationshipUpdateModeSet},
		})
		require.NoError(t, err)
		require.NoError(t, f.repository.ImageArtist.SetImageArtists(ctx, f.image.ID, nil))
		return f.repository.SceneArtist.SetSceneArtists(ctx, f.video.ID, nil)
	}))
	expect(&newTag, 0)
	expect(f.profileTag, 1)
	for _, video := range []bool{false, true} {
		require.Equal(t, []int{f.profileTag.ID}, f.direct(t, video).tags, "an explicit assignment survives losing its profile source")
	}
}

func TestP06TagSearchTerminatesOnLegacyTagCycles(t *testing.T) {
	f := newP06SQLiteFixture(t)
	// Simulate malformed legacy/imported storage, bypassing service validation.
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		return f.repository.Tag.UpdateParentTags(ctx, f.parentTag.ID, []int{f.profileTag.ID})
	}))
	zero := 0
	for _, video := range []bool{false, true} {
		ids, total := p06SearchTags(t, f, video, &models.HierarchicalMultiCriterionInput{
			Value: []string{strconv.Itoa(f.parentTag.ID)}, Modifier: models.CriterionModifierIncludes, Depth: &zero,
		}, nil)
		require.Len(t, ids, 1)
		require.Equal(t, 1, total)
	}
}
