package api

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

// MediaEffectiveAssociations are derived at read time from direct media links.
// Direct GraphQL fields remain the editable, stored relations; this projection
// adds active parents and profile Tags without persisting sticky copies.
type MediaEffectiveAssociations struct {
	Tags       []*models.Tag       `json:"tags"`
	Artists    []*models.Studio    `json:"artists"`
	Performers []*models.Performer `json:"performers"`
	Copyrights []*models.Copyright `json:"copyrights"`
}

type directMediaAssociationIDs struct {
	tags       []int
	artists    []int
	performers []int
	copyrights []int
}

type effectiveMediaAssociationIDs struct {
	tags       []int
	artists    []int
	performers []int
	copyrights []int
}

type effectiveAssociationTagOrigin struct {
	Kind           string `json:"kind"`
	EntityID       int    `json:"entityID,omitempty"`
	EntityAncestor bool   `json:"entityAncestor,omitempty"`
	SourceTagID    int    `json:"sourceTagID,omitempty"`
	TagAncestor    bool   `json:"tagAncestor,omitempty"`
}

type taggingInheritedTag struct {
	ID      int                             `json:"id"`
	Name    string                          `json:"name"`
	Origins []effectiveAssociationTagOrigin `json:"origins"`
}

type effectiveTagOriginTraversalKey struct {
	TagID          int
	EntityID       int
	SourceTagID    int
	Kind           string
	EntityAncestor bool
	TagAncestor    bool
}

type effectiveTagOriginTraversal struct {
	TagID  int
	Origin effectiveAssociationTagOrigin
}

func appendUniqueIDs(destination []int, seen map[int]struct{}, ids []int) []int {
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

func expandIDsWithParents(
	ctx context.Context,
	directIDs []int,
	findParents func(context.Context, int) ([]int, error),
) ([]int, error) {
	seen := make(map[int]struct{}, len(directIDs))
	ids := appendUniqueIDs(nil, seen, directIDs)
	for index := 0; index < len(ids); index++ {
		parents, err := findParents(ctx, ids[index])
		if err != nil {
			return nil, err
		}
		ids = appendUniqueIDs(ids, seen, parents)
	}
	return ids, nil
}

func expandIDsWithParentsWhenEnabled(
	ctx context.Context,
	directIDs []int,
	includeParents bool,
	findParents func(context.Context, int) ([]int, error),
) ([]int, error) {
	if includeParents {
		return expandIDsWithParents(ctx, directIDs, findParents)
	}
	seen := make(map[int]struct{}, len(directIDs))
	return appendUniqueIDs(nil, seen, directIDs), nil
}

func resolveEffectiveMediaAssociationIDs(
	ctx context.Context,
	repository models.Repository,
	direct directMediaAssociationIDs,
	settings config.AssociationInheritanceSettings,
) (effectiveMediaAssociationIDs, error) {
	var effective effectiveMediaAssociationIDs
	var err error

	effective.performers, err = expandIDsWithParentsWhenEnabled(
		ctx, direct.performers, settings.Characters, func(ctx context.Context, id int) ([]int, error) {
			performer, err := repository.Performer.Find(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("loading Character %d: %w", id, err)
			}
			if performer == nil || performer.ParentID == nil {
				return nil, nil
			}
			return []int{*performer.ParentID}, nil
		},
	)
	if err != nil {
		return effectiveMediaAssociationIDs{}, fmt.Errorf("resolving Character ancestors: %w", err)
	}

	effective.artists, err = expandIDsWithParentsWhenEnabled(
		ctx, direct.artists, settings.Artists, func(ctx context.Context, id int) ([]int, error) {
			artist, err := repository.Studio.Find(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("loading Artist %d: %w", id, err)
			}
			if artist == nil || artist.ParentID == nil {
				return nil, nil
			}
			return []int{*artist.ParentID}, nil
		},
	)
	if err != nil {
		return effectiveMediaAssociationIDs{}, fmt.Errorf("resolving Artist ancestors: %w", err)
	}

	effective.copyrights, err = expandIDsWithParentsWhenEnabled(
		ctx, direct.copyrights, settings.Copyrights, func(ctx context.Context, id int) ([]int, error) {
			parents, err := repository.Copyright.FindParents(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("loading Copyright %d parents: %w", id, err)
			}
			parentIDs := make([]int, 0, len(parents))
			for _, parent := range parents {
				if parent != nil {
					parentIDs = append(parentIDs, parent.ID)
				}
			}
			return parentIDs, nil
		},
	)
	if err != nil {
		return effectiveMediaAssociationIDs{}, fmt.Errorf("resolving Copyright ancestors: %w", err)
	}

	profileTagIDs := make([]int, 0, len(direct.tags))
	seenProfileTags := make(map[int]struct{}, len(direct.tags))
	profileTagIDs = appendUniqueIDs(profileTagIDs, seenProfileTags, direct.tags)
	for _, id := range effective.performers {
		tagIDs, err := repository.Performer.GetTagIDs(ctx, id)
		if err != nil {
			return effectiveMediaAssociationIDs{}, fmt.Errorf("loading Character %d profile Tags: %w", id, err)
		}
		profileTagIDs = appendUniqueIDs(profileTagIDs, seenProfileTags, tagIDs)
	}
	for _, id := range effective.artists {
		tagIDs, err := repository.Studio.GetTagIDs(ctx, id)
		if err != nil {
			return effectiveMediaAssociationIDs{}, fmt.Errorf("loading Artist %d profile Tags: %w", id, err)
		}
		profileTagIDs = appendUniqueIDs(profileTagIDs, seenProfileTags, tagIDs)
	}
	for _, id := range effective.copyrights {
		tagIDs, err := repository.Copyright.GetTagIDs(ctx, id)
		if err != nil {
			return effectiveMediaAssociationIDs{}, fmt.Errorf("loading Copyright %d profile Tags: %w", id, err)
		}
		profileTagIDs = appendUniqueIDs(profileTagIDs, seenProfileTags, tagIDs)
	}

	effective.tags, err = expandIDsWithParentsWhenEnabled(
		ctx, profileTagIDs, settings.Tags, func(ctx context.Context, id int) ([]int, error) {
			parents, err := repository.Tag.FindByChildTagID(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("loading Tag %d parents: %w", id, err)
			}
			parentIDs := make([]int, 0, len(parents))
			for _, parent := range parents {
				if parent != nil {
					parentIDs = append(parentIDs, parent.ID)
				}
			}
			return parentIDs, nil
		},
	)
	if err != nil {
		return effectiveMediaAssociationIDs{}, fmt.Errorf("resolving Tag ancestors: %w", err)
	}

	return effective, nil
}

func resolveEffectiveAssociationTagOrigins(
	ctx context.Context,
	repository models.Repository,
	direct directMediaAssociationIDs,
	settings config.AssociationInheritanceSettings,
	effective effectiveMediaAssociationIDs,
) (map[int][]effectiveAssociationTagOrigin, error) {
	origins := make(map[int][]effectiveAssociationTagOrigin)
	seen := make(map[effectiveTagOriginTraversalKey]struct{})
	queue := make([]effectiveTagOriginTraversal, 0)

	enqueue := func(tagID int, origin effectiveAssociationTagOrigin) {
		if tagID <= 0 {
			return
		}
		key := effectiveTagOriginTraversalKey{
			TagID:          tagID,
			EntityID:       origin.EntityID,
			SourceTagID:    origin.SourceTagID,
			Kind:           origin.Kind,
			EntityAncestor: origin.EntityAncestor,
			TagAncestor:    origin.TagAncestor,
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		origins[tagID] = append(origins[tagID], origin)
		queue = append(queue, effectiveTagOriginTraversal{TagID: tagID, Origin: origin})
	}

	for _, id := range direct.tags {
		enqueue(id, effectiveAssociationTagOrigin{Kind: "selected_tag", SourceTagID: id})
	}

	directPerformers := make(map[int]struct{}, len(direct.performers))
	for _, id := range direct.performers {
		directPerformers[id] = struct{}{}
	}
	for _, id := range effective.performers {
		performer, err := repository.Performer.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Character %d for Tag origin: %w", id, err)
		}
		if performer == nil {
			continue
		}
		tagIDs, err := repository.Performer.GetTagIDs(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Character %d profile Tags: %w", id, err)
		}
		_, isDirect := directPerformers[id]
		for _, tagID := range tagIDs {
			enqueue(tagID, effectiveAssociationTagOrigin{
				Kind:           "character_profile",
				EntityID:       id,
				EntityAncestor: !isDirect,
				SourceTagID:    tagID,
			})
		}
	}

	directArtists := make(map[int]struct{}, len(direct.artists))
	for _, id := range direct.artists {
		directArtists[id] = struct{}{}
	}
	for _, id := range effective.artists {
		artist, err := repository.Studio.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Artist %d for Tag origin: %w", id, err)
		}
		if artist == nil {
			continue
		}
		tagIDs, err := repository.Studio.GetTagIDs(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Artist %d profile Tags: %w", id, err)
		}
		_, isDirect := directArtists[id]
		for _, tagID := range tagIDs {
			enqueue(tagID, effectiveAssociationTagOrigin{
				Kind:           "artist_profile",
				EntityID:       id,
				EntityAncestor: !isDirect,
				SourceTagID:    tagID,
			})
		}
	}

	directCopyrights := make(map[int]struct{}, len(direct.copyrights))
	for _, id := range direct.copyrights {
		directCopyrights[id] = struct{}{}
	}
	for _, id := range effective.copyrights {
		tagIDs, err := repository.Copyright.GetTagIDs(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Copyright %d profile Tags: %w", id, err)
		}
		_, isDirect := directCopyrights[id]
		for _, tagID := range tagIDs {
			enqueue(tagID, effectiveAssociationTagOrigin{
				Kind:           "copyright_profile",
				EntityID:       id,
				EntityAncestor: !isDirect,
				SourceTagID:    tagID,
			})
		}
	}

	if settings.Tags {
		for index := 0; index < len(queue); index++ {
			current := queue[index]
			parents, err := repository.Tag.FindByChildTagID(ctx, current.TagID)
			if err != nil {
				return nil, fmt.Errorf("loading Tag %d parents for origin: %w", current.TagID, err)
			}
			for _, parent := range parents {
				if parent == nil {
					continue
				}
				origin := current.Origin
				origin.TagAncestor = true
				enqueue(parent.ID, origin)
			}
		}
	}

	return origins, nil
}

func buildMediaEffectiveAssociations(
	ctx context.Context,
	repository models.Repository,
	direct directMediaAssociationIDs,
) (*MediaEffectiveAssociations, error) {
	settings := config.GetInstance().GetAssociationInheritanceSettings()
	ids, err := resolveEffectiveMediaAssociationIDs(ctx, repository, direct, settings)
	if err != nil {
		return nil, err
	}
	result := &MediaEffectiveAssociations{
		Tags:       []*models.Tag{},
		Artists:    []*models.Studio{},
		Performers: []*models.Performer{},
		Copyrights: []*models.Copyright{},
	}
	if len(ids.tags) > 0 {
		result.Tags, err = repository.Tag.FindMany(ctx, ids.tags)
		if err != nil {
			return nil, fmt.Errorf("loading effective Tags: %w", err)
		}
	}
	if len(ids.artists) > 0 {
		result.Artists, err = repository.Studio.FindMany(ctx, ids.artists)
		if err != nil {
			return nil, fmt.Errorf("loading effective Artists: %w", err)
		}
	}
	if len(ids.performers) > 0 {
		result.Performers, err = repository.Performer.FindMany(ctx, ids.performers)
		if err != nil {
			return nil, fmt.Errorf("loading effective Characters: %w", err)
		}
	}
	if len(ids.copyrights) > 0 {
		result.Copyrights, err = repository.Copyright.FindMany(ctx, ids.copyrights)
		if err != nil {
			return nil, fmt.Errorf("loading effective Copyrights: %w", err)
		}
	}
	return result, nil
}

func uniqueRelatedModelIDs[T any](items []*T, getID func(*T) int) []int {
	ids := make([]int, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item != nil {
			ids = appendUniqueIDs(ids, seen, []int{getID(item)})
		}
	}
	return ids
}
