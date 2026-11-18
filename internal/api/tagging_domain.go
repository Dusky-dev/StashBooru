package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func camiePredictionIncludesFilenameSource(source string) bool {
	for _, part := range strings.Split(strings.ToLower(strings.TrimSpace(source)), "+") {
		if strings.TrimSpace(part) == "filename" {
			return true
		}
	}
	return false
}

// partitionCamieFilenameAuthoritativeSelections enforces local filename
// authority while retaining lower-priority identity predictions for dry-run
// review. The mutation path only receives the kept predictions.
func partitionCamieFilenameAuthoritativeSelections(predictions []camietagger.Tag) ([]camietagger.Tag, []camietagger.Tag) {
	authoritativeCategories := make(map[string]bool, 3)
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if camieFilenameAuthoritativeCategory(prediction.Category) && camiePredictionIncludesFilenameSource(prediction.Source) {
			authoritativeCategories[prediction.Category] = true
		}
	}
	if len(authoritativeCategories) == 0 {
		return predictions, nil
	}

	kept := make([]camietagger.Tag, 0, len(predictions))
	suppressed := make([]camietagger.Tag, 0)
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if authoritativeCategories[prediction.Category] && !camiePredictionIncludesFilenameSource(prediction.Source) {
			suppressed = append(suppressed, prediction)
			continue
		}
		kept = append(kept, prediction)
	}
	return kept, suppressed
}

// filterCamieFilenameAuthoritativeSelections keeps the existing shared
// apply-time contract for callers that do not need suppression details.
func filterCamieFilenameAuthoritativeSelections(predictions []camietagger.Tag) []camietagger.Tag {
	kept, _ := partitionCamieFilenameAuthoritativeSelections(predictions)
	return kept
}

type taggingResolvedEntities struct {
	Characters    []camieAppliedEntity
	Artists       []camieAppliedEntity
	Copyrights    []camieAppliedEntity
	Tags          []camieAppliedEntity
	InheritedTags []taggingInheritedTag

	CharacterIDs []int
	ArtistIDs    []int
	CopyrightIDs []int
	TagIDs       []int

	// CharacterPredictions and CharacterPredictionIDs are parallel: every
	// prediction keeps its Copyright context for linking, even when several
	// spellings resolve to one Character in CharacterIDs.
	CharacterPredictions   []camietagger.Tag
	CharacterPredictionIDs []int

	CreatedCharacters int
	CreatedArtists    int
	CreatedCopyrights int
	CreatedTags       int
}

// taggingConnection returns the native record ID that a provider connection
// assigns to a source name, or 0 when the name is not connected.
type taggingConnection func(ctx context.Context, prediction camietagger.Tag) (int, error)

func resolveTaggingEntities(ctx context.Context, repository models.Repository, predictions []camietagger.Tag) (taggingResolvedEntities, error) {
	return resolveConnectedTaggingEntities(ctx, repository, predictions, nil)
}

func connectedTaggingID(ctx context.Context, connected taggingConnection, prediction camietagger.Tag) (int, error) {
	if connected == nil {
		return 0, nil
	}
	return connected(ctx, prediction)
}

func containsTaggingID(ids []int, id int) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

// resolveConnectedTaggingEntities resolves connected source names to their
// native records first; other names use the native name/alias resolver. Several
// spellings of one record are applied once.
func resolveConnectedTaggingEntities(ctx context.Context, repository models.Repository, predictions []camietagger.Tag, connected taggingConnection) (taggingResolvedEntities, error) {
	resolved := taggingResolvedEntities{
		Characters:    []camieAppliedEntity{},
		Artists:       []camieAppliedEntity{},
		Copyrights:    []camieAppliedEntity{},
		Tags:          []camieAppliedEntity{},
		InheritedTags: []taggingInheritedTag{},
	}

	normalized := make([]camietagger.Tag, 0, len(predictions))
	var artists, copyrights, metadataTags []camietagger.Tag
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		normalized = append(normalized, prediction)
		switch prediction.Category {
		case "character":
			resolved.CharacterPredictions = append(resolved.CharacterPredictions, prediction)
		case "artist":
			artists = append(artists, prediction)
		case "copyright":
			copyrights = append(copyrights, prediction)
		default:
			metadataTags = append(metadataTags, prediction)
		}
	}

	resolved.CharacterIDs = make([]int, 0, len(resolved.CharacterPredictions))
	resolved.CharacterPredictionIDs = make([]int, 0, len(resolved.CharacterPredictions))
	for _, prediction := range resolved.CharacterPredictions {
		var entity *models.Performer
		created := false
		id, err := connectedTaggingID(ctx, connected, prediction)
		if err == nil && id > 0 {
			entity, err = repository.Performer.Find(ctx, id)
		}
		if err == nil && entity == nil {
			entity, created, err = findOrCreateCamiePerformerPredictionWithCopyrightContext(ctx, repository, prediction, normalized)
		}
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving character %q: %w", prediction.Name, err)
		}
		resolved.CharacterPredictionIDs = append(resolved.CharacterPredictionIDs, entity.ID)
		if containsTaggingID(resolved.CharacterIDs, entity.ID) {
			continue
		}
		resolved.CharacterIDs = append(resolved.CharacterIDs, entity.ID)
		resolved.Characters = append(resolved.Characters, camieAppliedEntity{
			ID:       entity.ID,
			Name:     entity.Name,
			Category: prediction.Category,
			Score:    prediction.Score,
			Created:  created,
		})
		if created {
			resolved.CreatedCharacters++
		}
	}

	resolved.ArtistIDs = make([]int, 0, len(artists))
	for _, prediction := range artists {
		var entity *models.Studio
		created := false
		id, err := connectedTaggingID(ctx, connected, prediction)
		if err == nil && id > 0 {
			entity, err = repository.Studio.Find(ctx, id)
		}
		if err == nil && entity == nil {
			entity, created, err = findOrCreateCamieStudioPrediction(ctx, repository, prediction)
		}
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving artist %q: %w", prediction.Name, err)
		}
		if containsTaggingID(resolved.ArtistIDs, entity.ID) {
			continue
		}
		resolved.ArtistIDs = append(resolved.ArtistIDs, entity.ID)
		resolved.Artists = append(resolved.Artists, camieAppliedEntity{
			ID:       entity.ID,
			Name:     entity.Name,
			Category: prediction.Category,
			Score:    prediction.Score,
			Created:  created,
		})
		if created {
			resolved.CreatedArtists++
		}
	}

	resolved.CopyrightIDs = make([]int, 0, len(copyrights))
	for _, prediction := range copyrights {
		var entity *models.Copyright
		created := false
		id, err := connectedTaggingID(ctx, connected, prediction)
		if err == nil && id > 0 {
			entity, err = repository.Copyright.Find(ctx, id)
		}
		if err == nil && entity == nil {
			entity, created, err = findOrCreateNativeCamieCopyright(ctx, repository, prediction)
		}
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving copyright %q: %w", prediction.Name, err)
		}
		if containsTaggingID(resolved.CopyrightIDs, entity.ID) {
			continue
		}
		resolved.CopyrightIDs = append(resolved.CopyrightIDs, entity.ID)
		resolved.Copyrights = append(resolved.Copyrights, camieAppliedEntity{
			ID:       entity.ID,
			Name:     entity.Name,
			Category: prediction.Category,
			Score:    prediction.Score,
			Created:  created,
		})
		if created {
			resolved.CreatedCopyrights++
		}
	}

	resolved.TagIDs = make([]int, 0, len(metadataTags))
	for _, prediction := range metadataTags {
		entity, created, err := findOrCreateCamieTagPrediction(ctx, repository, prediction)
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving %s tag %q: %w", prediction.Category, prediction.Name, err)
		}
		if prediction.Category == "rating" {
			if err := ensureContentRatingHierarchy(ctx, repository, entity); err != nil {
				return taggingResolvedEntities{}, err
			}
		}
		resolved.TagIDs = append(resolved.TagIDs, entity.ID)
		resolved.Tags = append(resolved.Tags, camieAppliedEntity{
			ID:       entity.ID,
			Name:     entity.Name,
			Category: prediction.Category,
			Score:    prediction.Score,
			Created:  created,
		})
		if created {
			resolved.CreatedTags++
		}
	}

	settings := config.GetInstance().GetAssociationInheritanceSettings()
	inheritedTags, err := resolveInheritedTaggingTags(ctx, repository, directMediaAssociationIDs{
		tags:       resolved.TagIDs,
		artists:    resolved.ArtistIDs,
		performers: resolved.CharacterIDs,
		copyrights: resolved.CopyrightIDs,
	}, settings)
	if err != nil {
		return taggingResolvedEntities{}, err
	}
	resolved.InheritedTags = inheritedTags
	return resolved, nil
}

func linkResolvedTaggingCharacterCopyrights(ctx context.Context, repository models.Repository, resolved taggingResolvedEntities) error {
	if err := linkCamieCharacterCopyrights(ctx, repository, resolved.CharacterPredictions, resolved.CharacterPredictionIDs, resolved.CopyrightIDs); err != nil {
		return fmt.Errorf("linking Character Copyrights: %w", err)
	}
	return nil
}
