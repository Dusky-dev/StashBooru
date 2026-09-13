package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func camieCharacterLookupKey(value string) string {
	return strings.ToLower(canonicalCamieName(value, "character"))
}

func findCamiePerformerAliasPrediction(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, error) {
	prediction = normalizeCamiePrediction(prediction)
	_, predictedDisambiguation := camieCharacterIdentity(prediction)

	candidates := make(map[string]struct{}, 2)
	for _, value := range []string{prediction.Name, prediction.RawName} {
		if key := camieCharacterLookupKey(value); key != "" {
			candidates[key] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	performers, err := repository.Performer.All(ctx)
	if err != nil {
		return nil, err
	}

	var candidate *models.Performer
	for _, performerEntity := range performers {
		if performerEntity == nil {
			continue
		}

		// A Character with an explicit, different disambiguation is a distinct
		// identity even if stale data happens to give it the same alias.
		if predictedDisambiguation != "" {
			existingDisambiguation := strings.TrimSpace(performerEntity.Disambiguation)
			if existingDisambiguation != "" && !strings.EqualFold(existingDisambiguation, predictedDisambiguation) {
				continue
			}
		}

		aliases, err := repository.Performer.GetAliases(ctx, performerEntity.ID)
		if err != nil {
			return nil, err
		}
		matched := false
		for _, alias := range aliases {
			if _, ok := candidates[camieCharacterLookupKey(alias)]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		if candidate != nil && candidate.ID != performerEntity.ID {
			return nil, fmt.Errorf("character alias %q matches multiple existing Characters", prediction.Name)
		}
		candidate = performerEntity
	}

	return candidate, nil
}
