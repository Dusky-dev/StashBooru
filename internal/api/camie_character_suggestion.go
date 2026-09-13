package api

import (
	"context"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func selectCamieBarePerformerSuggestion(matches []*models.Performer, characterName, predictedDisambiguation string) *models.Performer {
	characterName = strings.TrimSpace(characterName)
	predictedDisambiguation = strings.TrimSpace(predictedDisambiguation)
	if characterName == "" || predictedDisambiguation == "" {
		return nil
	}

	var candidate *models.Performer
	for _, match := range matches {
		if match == nil || !strings.EqualFold(strings.TrimSpace(match.Name), characterName) {
			continue
		}
		// A differently-disambiguated Character is a distinct identity and must
		// never be offered as the possible existing match.
		if strings.TrimSpace(match.Disambiguation) != "" {
			continue
		}
		if candidate != nil && candidate.ID != match.ID {
			// Do not guess if an inconsistent database somehow contains multiple
			// bare same-name candidates.
			return nil
		}
		candidate = match
	}
	return candidate
}

func findCamieBarePerformerSuggestion(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, error) {
	prediction = normalizeCamiePrediction(prediction)
	characterName, disambiguation := camieCharacterIdentity(prediction)
	if disambiguation == "" {
		return nil, nil
	}

	matches, err := repository.Performer.FindByNames(ctx, []string{characterName}, true)
	if err != nil {
		return nil, err
	}
	return selectCamieBarePerformerSuggestion(matches, characterName, disambiguation), nil
}
