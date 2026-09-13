package api

import (
	"context"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

// findCamieCharacterTargetCandidates returns the existing Characters that make
// a bare Character prediction ambiguous. It is intentionally only used after
// automatic resolution has failed: trusted Copyright context and exact
// disambiguation remain authoritative and never require manual selection.
func findCamieCharacterTargetCandidates(ctx context.Context, repository models.Repository, prediction camietagger.Tag) ([]camietagger.TargetCandidate, error) {
	prediction = normalizeCamiePrediction(prediction)
	characterName, disambiguation := camieCharacterIdentity(prediction)
	if strings.TrimSpace(characterName) == "" || strings.TrimSpace(disambiguation) != "" {
		return nil, nil
	}

	matches, err := camieSameNamePerformers(ctx, repository, characterName)
	if err != nil {
		return nil, err
	}
	if len(matches) < 2 {
		return nil, nil
	}

	candidates := make([]camietagger.TargetCandidate, 0, len(matches))
	for _, match := range matches {
		if match == nil {
			continue
		}
		candidates = append(candidates, camietagger.TargetCandidate{
			ID:             match.ID,
			Name:           match.Name,
			Disambiguation: match.Disambiguation,
		})
	}
	if len(candidates) < 2 {
		return nil, nil
	}
	return candidates, nil
}
