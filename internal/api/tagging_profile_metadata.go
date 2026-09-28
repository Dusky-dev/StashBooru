package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/tag"
)

const contentRatingRootTag = "rating"

// inheritResolvedProfileTagIDs adds Tags owned by resolved Character, Artist,
// and Copyright profiles and their ancestors to the tagging result. It also
// includes the ancestor Tags of predicted and profile Tags, matching the
// effective association projection used by media detail views.
func inheritResolvedProfileTagIDs(ctx context.Context, repository models.Repository, resolved *taggingResolvedEntities) error {
	effective, err := resolveEffectiveMediaAssociationIDs(ctx, repository, directMediaAssociationIDs{
		tags:       resolved.TagIDs,
		artists:    resolved.ArtistIDs,
		performers: resolved.CharacterIDs,
		copyrights: resolved.CopyrightIDs,
	})
	if err != nil {
		return fmt.Errorf("resolving inherited tagging profile Tags: %w", err)
	}
	resolved.TagIDs = effective.tags
	return nil
}

// ensureContentRatingHierarchy keeps content ratings in the normal native Tag
// hierarchy. The structural root Tag is not applied to media; applying a child
// such as safe/questionable/explicit is enough, and any pre-existing parents on
// that child are preserved.
func ensureContentRatingHierarchy(ctx context.Context, repository models.Repository, child *models.Tag) error {
	if child == nil || strings.EqualFold(strings.TrimSpace(child.Name), contentRatingRootTag) {
		return nil
	}

	root, _, err := findOrCreateCamieTagPrediction(ctx, repository, camietagger.Tag{
		Name:     contentRatingRootTag,
		RawName:  contentRatingRootTag,
		Category: "general",
		Score:    1,
		Source:   "system",
	})
	if err != nil {
		return fmt.Errorf("resolving content-rating root Tag: %w", err)
	}
	if root == nil || root.ID <= 0 || root.ID == child.ID {
		return nil
	}

	parentIDs, err := repository.Tag.GetParentIDs(ctx, child.ID)
	if err != nil {
		return fmt.Errorf("loading parents for rating Tag %q: %w", child.Name, err)
	}
	for _, parentID := range parentIDs {
		if parentID == root.ID {
			return nil
		}
	}
	parentIDs = append(parentIDs, root.ID)
	if err := tag.ValidateHierarchyExisting(ctx, child, parentIDs, nil, repository.Tag); err != nil {
		return fmt.Errorf("validating content-rating hierarchy for %q: %w", child.Name, err)
	}
	if err := repository.Tag.UpdateParentTags(ctx, child.ID, parentIDs); err != nil {
		return fmt.Errorf("linking rating Tag %q under %q: %w", child.Name, root.Name, err)
	}
	return nil
}
