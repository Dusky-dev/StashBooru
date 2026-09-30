package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/tag"
)

const contentRatingRootTag = "rating"

// resolveInheritedTaggingTags calculates profile Tags and Tag ancestors for
// preview and review. These remain derived associations; callers must only
// persist the explicitly selected Tag IDs.
func resolveInheritedTaggingTags(
	ctx context.Context,
	repository models.Repository,
	direct directMediaAssociationIDs,
	settings config.AssociationInheritanceSettings,
) ([]taggingInheritedTag, error) {
	effective, err := resolveEffectiveMediaAssociationIDs(ctx, repository, direct, settings)
	if err != nil {
		return nil, fmt.Errorf("resolving inherited tagging profile Tags: %w", err)
	}
	origins, err := resolveEffectiveAssociationTagOrigins(ctx, repository, direct, settings, effective)
	if err != nil {
		return nil, fmt.Errorf("resolving tagging Tag origins: %w", err)
	}
	tagIDs := append([]int(nil), effective.tags...)
	sort.Ints(tagIDs)
	if len(tagIDs) == 0 {
		return []taggingInheritedTag{}, nil
	}

	tags, err := repository.Tag.FindMany(ctx, tagIDs)
	if err != nil {
		return nil, fmt.Errorf("loading inherited tagging Tags: %w", err)
	}
	tagsByID := make(map[int]*models.Tag, len(tags))
	for _, tag := range tags {
		if tag != nil {
			tagsByID[tag.ID] = tag
		}
	}
	directTagIDs := make(map[int]struct{}, len(direct.tags))
	for _, id := range direct.tags {
		directTagIDs[id] = struct{}{}
	}

	result := make([]taggingInheritedTag, 0, len(tagIDs))
	for _, id := range tagIDs {
		if _, direct := directTagIDs[id]; direct {
			continue
		}
		tag := tagsByID[id]
		if tag == nil {
			continue
		}
		tagOrigins := append([]effectiveAssociationTagOrigin(nil), origins[id]...)
		sort.Slice(tagOrigins, func(i, j int) bool {
			left, right := tagOrigins[i], tagOrigins[j]
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			if left.EntityID != right.EntityID {
				return left.EntityID < right.EntityID
			}
			if left.SourceTagID != right.SourceTagID {
				return left.SourceTagID < right.SourceTagID
			}
			if left.EntityAncestor != right.EntityAncestor {
				return !left.EntityAncestor && right.EntityAncestor
			}
			return !left.TagAncestor && right.TagAncestor
		})
		result = append(result, taggingInheritedTag{ID: id, Name: tag.Name, Origins: tagOrigins})
	}
	return result, nil
}

func taggingDirectAssociationsFromPredictions(predictions []camietagger.Tag) directMediaAssociationIDs {
	var direct directMediaAssociationIDs
	seen := map[string]map[int]struct{}{
		"performers": {},
		"artists":    {},
		"copyrights": {},
		"tags":       {},
	}
	appendID := func(ids *[]int, kind string, id int) {
		if _, exists := seen[kind][id]; exists {
			return
		}
		seen[kind][id] = struct{}{}
		*ids = append(*ids, id)
	}
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if !prediction.TargetExists {
			continue
		}
		parts := strings.Split(strings.Trim(prediction.TargetPath, "/"), "/")
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			continue
		}

		switch prediction.Category {
		case "character":
			if parts[0] == "performers" {
				appendID(&direct.performers, "performers", id)
			}
		case "artist":
			if parts[0] == "studios" {
				appendID(&direct.artists, "artists", id)
			}
		case "copyright":
			if parts[0] == "copyrights" {
				appendID(&direct.copyrights, "copyrights", id)
			}
		default:
			if parts[0] == "tags" {
				appendID(&direct.tags, "tags", id)
			}
		}
	}
	return direct
}

func resolveInheritedTaggingTagsForPredictions(
	ctx context.Context,
	repository models.Repository,
	predictions []camietagger.Tag,
	settings config.AssociationInheritanceSettings,
) ([]taggingInheritedTag, error) {
	direct := taggingDirectAssociationsFromPredictions(predictions)
	return resolveInheritedTaggingTags(ctx, repository, direct, settings)
}

func populateTaggingChangePlanInheritedTags(
	ctx context.Context,
	repository models.Repository,
	predictions []camietagger.Tag,
	plan *taggingChangePlan,
) error {
	settings := config.GetInstance().GetAssociationInheritanceSettings()
	return repository.WithReadTxn(ctx, func(ctx context.Context) error {
		inheritedTags, err := resolveInheritedTaggingTagsForPredictions(ctx, repository, predictions, settings)
		if err != nil {
			return err
		}
		plan.InheritedTags = inheritedTags
		return nil
	})
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
