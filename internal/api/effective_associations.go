package api

import (
	"context"
	"fmt"
	"sort"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

// MediaEffectiveAssociations are derived at read time from direct media links.
// Direct GraphQL fields remain the editable, stored relations; this projection
// adds active parents and profile Tags without persisting sticky copies.
type MediaEffectiveAssociations struct {
	Tags       []*models.Tag                `json:"tags"`
	Artists    []*models.Studio             `json:"artists"`
	Performers []*models.Performer          `json:"performers"`
	Copyrights []*models.Copyright          `json:"copyrights"`
	Provenance []MediaAssociationProvenance `json:"provenance"`
}

// MediaAssociationProvenance describes why one entity is present in an
// effective media association. Origins are computed from current direct links
// and hierarchy/profile relations, so reparenting is reflected on the next
// read without rewriting media rows.
type MediaAssociationProvenance struct {
	AssociationType string                   `json:"associationType"`
	AssociationID   int                      `json:"associationID"`
	Origins         []MediaAssociationOrigin `json:"origins"`
}

// MediaAssociationOrigin identifies one direct selection or derivation path.
// SourceID is the direct entity selected on the media; ViaID identifies the
// parent/profile node or Tag edge that supplied the effective association.
type MediaAssociationOrigin struct {
	Kind        string `json:"kind"`
	SourceType  string `json:"sourceType"`
	SourceID    int    `json:"sourceID"`
	ViaType     string `json:"viaType,omitempty"`
	ViaID       int    `json:"viaID,omitempty"`
	SourceTagID int    `json:"sourceTagID,omitempty"`
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

type mediaAssociationOriginKey struct {
	kind        string
	sourceType  string
	sourceID    int
	viaType     string
	viaID       int
	sourceTagID int
}

type mediaAssociationKey struct {
	associationType string
	associationID   int
}

type hierarchyOriginTraversal struct {
	entityID int
	sourceID int
}

type tagAssociationOriginTraversal struct {
	tagID  int
	origin MediaAssociationOrigin
}

type tagAssociationOriginKey struct {
	tagID       int
	sourceType  string
	sourceID    int
	sourceTagID int
}

func addMediaAssociationOrigin(
	provenance map[mediaAssociationKey]map[mediaAssociationOriginKey]MediaAssociationOrigin,
	associationType string,
	associationID int,
	origin MediaAssociationOrigin,
) {
	if associationID <= 0 || origin.SourceID <= 0 {
		return
	}
	association := mediaAssociationKey{associationType: associationType, associationID: associationID}
	origins := provenance[association]
	if origins == nil {
		origins = make(map[mediaAssociationOriginKey]MediaAssociationOrigin)
		provenance[association] = origins
	}
	key := mediaAssociationOriginKey{
		kind:        origin.Kind,
		sourceType:  origin.SourceType,
		sourceID:    origin.SourceID,
		viaType:     origin.ViaType,
		viaID:       origin.ViaID,
		sourceTagID: origin.SourceTagID,
	}
	origins[key] = origin
}

// addHierarchyAssociationProvenance records direct selections and all
// ancestor support while retaining which direct selection supports each
// effective ancestor. The seen set is per direct source so shared ancestors
// keep one origin per independent child without looping on malformed cycles.
func addHierarchyAssociationProvenance(
	ctx context.Context,
	provenance map[mediaAssociationKey]map[mediaAssociationOriginKey]MediaAssociationOrigin,
	associationType string,
	directIDs []int,
	includeAncestors bool,
	findParents func(context.Context, int) ([]int, error),
) (map[int][]int, error) {
	support := make(map[int]map[int]struct{})
	addSupport := func(entityID, sourceID int) {
		if support[entityID] == nil {
			support[entityID] = make(map[int]struct{})
		}
		support[entityID][sourceID] = struct{}{}
	}

	directSeen := make(map[int]struct{}, len(directIDs))
	queue := make([]hierarchyOriginTraversal, 0, len(directIDs))
	for _, id := range directIDs {
		if id <= 0 {
			continue
		}
		if _, exists := directSeen[id]; exists {
			continue
		}
		directSeen[id] = struct{}{}
		addSupport(id, id)
		addMediaAssociationOrigin(provenance, associationType, id, MediaAssociationOrigin{
			Kind: "direct", SourceType: associationType, SourceID: id,
		})
		queue = append(queue, hierarchyOriginTraversal{entityID: id, sourceID: id})
	}

	if includeAncestors {
		seen := make(map[hierarchyOriginTraversal]struct{})
		for index := 0; index < len(queue); index++ {
			current := queue[index]
			if _, exists := seen[current]; exists {
				continue
			}
			seen[current] = struct{}{}
			parents, err := findParents(ctx, current.entityID)
			if err != nil {
				return nil, err
			}
			for _, parentID := range parents {
				if parentID <= 0 {
					continue
				}
				addSupport(parentID, current.sourceID)
				if parentID != current.sourceID {
					addMediaAssociationOrigin(provenance, associationType, parentID, MediaAssociationOrigin{
						Kind: "ancestor", SourceType: associationType, SourceID: current.sourceID,
						ViaType: associationType, ViaID: current.entityID,
					})
				}
				queue = append(queue, hierarchyOriginTraversal{entityID: parentID, sourceID: current.sourceID})
			}
		}
	}

	result := make(map[int][]int, len(support))
	for entityID, sourceSet := range support {
		sources := make([]int, 0, len(sourceSet))
		for sourceID := range sourceSet {
			sources = append(sources, sourceID)
		}
		sort.Ints(sources)
		result[entityID] = sources
	}
	return result, nil
}

func effectiveAssociationProvenance(
	ctx context.Context,
	repository models.Repository,
	direct directMediaAssociationIDs,
	settings config.AssociationInheritanceSettings,
) (effectiveMediaAssociationIDs, []MediaAssociationProvenance, error) {
	provenance := make(map[mediaAssociationKey]map[mediaAssociationOriginKey]MediaAssociationOrigin)
	performerParents := func(ctx context.Context, id int) ([]int, error) {
		performer, err := repository.Performer.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Character %d: %w", id, err)
		}
		if performer == nil || performer.ParentID == nil {
			return nil, nil
		}
		return []int{*performer.ParentID}, nil
	}
	performerSources, err := addHierarchyAssociationProvenance(ctx, provenance, "character", direct.performers, settings.Characters, performerParents)
	if err != nil {
		return effectiveMediaAssociationIDs{}, nil, fmt.Errorf("resolving Character provenance: %w", err)
	}

	artistParents := func(ctx context.Context, id int) ([]int, error) {
		artist, err := repository.Studio.Find(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading Artist %d: %w", id, err)
		}
		if artist == nil || artist.ParentID == nil {
			return nil, nil
		}
		return []int{*artist.ParentID}, nil
	}
	artistSources, err := addHierarchyAssociationProvenance(ctx, provenance, "artist", direct.artists, settings.Artists, artistParents)
	if err != nil {
		return effectiveMediaAssociationIDs{}, nil, fmt.Errorf("resolving Artist provenance: %w", err)
	}

	copyrightParents := func(ctx context.Context, id int) ([]int, error) {
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
	}
	copyrightSources, err := addHierarchyAssociationProvenance(ctx, provenance, "copyright", direct.copyrights, settings.Copyrights, copyrightParents)
	if err != nil {
		return effectiveMediaAssociationIDs{}, nil, fmt.Errorf("resolving Copyright provenance: %w", err)
	}

	tagQueue := make([]tagAssociationOriginTraversal, 0, len(direct.tags))
	seenTagSeeds := make(map[mediaAssociationOriginKey]struct{})
	addTagSeed := func(tagID int, origin MediaAssociationOrigin) {
		if tagID <= 0 {
			return
		}
		key := mediaAssociationOriginKey{
			kind: origin.Kind, sourceType: origin.SourceType, sourceID: origin.SourceID,
			viaType: origin.ViaType, viaID: origin.ViaID, sourceTagID: origin.SourceTagID,
		}
		if _, exists := seenTagSeeds[key]; exists {
			return
		}
		seenTagSeeds[key] = struct{}{}
		addMediaAssociationOrigin(provenance, "tag", tagID, origin)
		tagQueue = append(tagQueue, tagAssociationOriginTraversal{tagID: tagID, origin: origin})
	}

	for _, id := range direct.tags {
		addTagSeed(id, MediaAssociationOrigin{Kind: "direct", SourceType: "tag", SourceID: id, SourceTagID: id})
	}
	for _, profile := range []struct {
		typeName string
		sources  map[int][]int
		getTags  func(context.Context, int) ([]int, error)
	}{
		{typeName: "character", sources: performerSources, getTags: repository.Performer.GetTagIDs},
		{typeName: "artist", sources: artistSources, getTags: repository.Studio.GetTagIDs},
		{typeName: "copyright", sources: copyrightSources, getTags: repository.Copyright.GetTagIDs},
	} {
		ownerIDs := make([]int, 0, len(profile.sources))
		for ownerID := range profile.sources {
			ownerIDs = append(ownerIDs, ownerID)
		}
		sort.Ints(ownerIDs)
		for _, ownerID := range ownerIDs {
			tagIDs, err := profile.getTags(ctx, ownerID)
			if err != nil {
				return effectiveMediaAssociationIDs{}, nil, fmt.Errorf("loading %s %d profile Tags: %w", profile.typeName, ownerID, err)
			}
			for _, sourceID := range profile.sources[ownerID] {
				kind := "ancestor_profile_tag"
				if ownerID == sourceID {
					kind = "profile_tag"
				}
				for _, tagID := range tagIDs {
					addTagSeed(tagID, MediaAssociationOrigin{
						Kind: kind, SourceType: profile.typeName, SourceID: sourceID,
						ViaType: profile.typeName, ViaID: ownerID, SourceTagID: tagID,
					})
				}
			}
		}
	}

	if settings.Tags {
		seen := make(map[tagAssociationOriginKey]struct{})
		for index := 0; index < len(tagQueue); index++ {
			current := tagQueue[index]
			traversalKey := tagAssociationOriginKey{
				tagID: current.tagID, sourceType: current.origin.SourceType,
				sourceID: current.origin.SourceID, sourceTagID: current.origin.SourceTagID,
			}
			if _, exists := seen[traversalKey]; exists {
				continue
			}
			seen[traversalKey] = struct{}{}
			parents, err := repository.Tag.FindByChildTagID(ctx, current.tagID)
			if err != nil {
				return effectiveMediaAssociationIDs{}, nil, fmt.Errorf("loading Tag %d parents: %w", current.tagID, err)
			}
			for _, parent := range parents {
				if parent == nil || parent.ID <= 0 {
					continue
				}
				origin := current.origin
				origin.Kind = "tag_ancestor"
				origin.ViaType = "tag"
				origin.ViaID = current.tagID
				addMediaAssociationOrigin(provenance, "tag", parent.ID, origin)
				tagQueue = append(tagQueue, tagAssociationOriginTraversal{tagID: parent.ID, origin: origin})
			}
		}
	}

	result := make([]MediaAssociationProvenance, 0, len(provenance))
	for association, origins := range provenance {
		originList := make([]MediaAssociationOrigin, 0, len(origins))
		for _, origin := range origins {
			originList = append(originList, origin)
		}
		sort.Slice(originList, func(i, j int) bool {
			left, right := originList[i], originList[j]
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			if left.SourceType != right.SourceType {
				return left.SourceType < right.SourceType
			}
			if left.SourceID != right.SourceID {
				return left.SourceID < right.SourceID
			}
			if left.ViaType != right.ViaType {
				return left.ViaType < right.ViaType
			}
			if left.ViaID != right.ViaID {
				return left.ViaID < right.ViaID
			}
			return left.SourceTagID < right.SourceTagID
		})
		result = append(result, MediaAssociationProvenance{
			AssociationType: association.associationType,
			AssociationID:   association.associationID,
			Origins:         originList,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].AssociationType != result[j].AssociationType {
			return result[i].AssociationType < result[j].AssociationType
		}
		return result[i].AssociationID < result[j].AssociationID
	})
	ids := effectiveMediaAssociationIDs{}
	for _, association := range result {
		switch association.AssociationType {
		case "tag":
			ids.tags = append(ids.tags, association.AssociationID)
		case "artist":
			ids.artists = append(ids.artists, association.AssociationID)
		case "character":
			ids.performers = append(ids.performers, association.AssociationID)
		case "copyright":
			ids.copyrights = append(ids.copyrights, association.AssociationID)
		}
	}
	return ids, result, nil
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
	ids, provenance, err := effectiveAssociationProvenance(ctx, repository, direct, settings)
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
	result.Provenance = provenance
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
