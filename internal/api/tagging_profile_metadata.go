package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

const contentRatingRootTag = "rating"

func appendUniqueTagIDs(destination []int, seen map[int]struct{}, ids []int) []int {
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		destination = append(destination, id)
	}
	return destination
}

// inheritResolvedProfileTagIDs adds Tags owned by resolved Character and Artist
// profiles to the media mutation. It is deliberately additive: the caller uses
// RelationshipUpdateModeAdd, so neither existing media Tags nor profile Tags are
// removed. Copyright remains a native first-class entity; its current model does
// not expose profile Tag IDs, so there is no synthetic Copyright-as-Tag fallback.
func inheritResolvedProfileTagIDs(ctx context.Context, repository models.Repository, resolved *taggingResolvedEntities) error {
	seen := make(map[int]struct{}, len(resolved.TagIDs))
	for _, id := range resolved.TagIDs {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}

	for _, characterID := range resolved.CharacterIDs {
		tagIDs, err := repository.Performer.GetTagIDs(ctx, characterID)
		if err != nil {
			return fmt.Errorf("loading Character %d profile Tags: %w", characterID, err)
		}
		resolved.TagIDs = appendUniqueTagIDs(resolved.TagIDs, seen, tagIDs)
	}
	for _, artistID := range resolved.ArtistIDs {
		tagIDs, err := repository.Studio.GetTagIDs(ctx, artistID)
		if err != nil {
			return fmt.Errorf("loading Artist %d profile Tags: %w", artistID, err)
		}
		resolved.TagIDs = appendUniqueTagIDs(resolved.TagIDs, seen, tagIDs)
	}
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
	if err := repository.Tag.UpdateParentTags(ctx, child.ID, parentIDs); err != nil {
		return fmt.Errorf("linking rating Tag %q under %q: %w", child.Name, root.Name, err)
	}
	return nil
}
