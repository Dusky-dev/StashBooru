package api

import (
	"context"
	"fmt"
	"strings"

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
	Characters []camieAppliedEntity
	Artists    []camieAppliedEntity
	Copyrights []camieAppliedEntity
	Tags       []camieAppliedEntity

	CharacterIDs []int
	ArtistIDs    []int
	CopyrightIDs []int
	TagIDs       []int

	CharacterPredictions []camietagger.Tag

	CreatedCharacters int
	CreatedArtists    int
	CreatedCopyrights int
	CreatedTags       int
}

func resolveTaggingEntities(ctx context.Context, repository models.Repository, predictions []camietagger.Tag) (taggingResolvedEntities, error) {
	resolved := taggingResolvedEntities{
		Characters: []camieAppliedEntity{},
		Artists:    []camieAppliedEntity{},
		Copyrights: []camieAppliedEntity{},
		Tags:       []camieAppliedEntity{},
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
	for _, prediction := range resolved.CharacterPredictions {
		entity, created, err := findOrCreateCamiePerformerPredictionWithCopyrightContext(ctx, repository, prediction, normalized)
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving character %q: %w", prediction.Name, err)
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
		entity, created, err := findOrCreateCamieStudioPrediction(ctx, repository, prediction)
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving artist %q: %w", prediction.Name, err)
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
		entity, created, err := findOrCreateNativeCamieCopyright(ctx, repository, prediction)
		if err != nil {
			return taggingResolvedEntities{}, fmt.Errorf("resolving copyright %q: %w", prediction.Name, err)
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

	return resolved, nil
}

func linkResolvedTaggingCharacterCopyrights(ctx context.Context, repository models.Repository, resolved taggingResolvedEntities) error {
	if err := linkCamieCharacterCopyrights(ctx, repository, resolved.CharacterPredictions, resolved.CharacterIDs, resolved.CopyrightIDs); err != nil {
		return fmt.Errorf("linking Character Copyrights: %w", err)
	}
	return nil
}
