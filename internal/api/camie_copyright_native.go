package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func findNativeCamieCopyright(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Copyright, error) {
	prediction = normalizeCamiePrediction(prediction)
	for _, name := range []string{prediction.Name, prediction.RawName} {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		existing, err := repository.Copyright.FindByName(ctx, name, true)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		existing, err = repository.Copyright.FindByAlias(ctx, name, true)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	return nil, nil
}

func findOrCreateNativeCamieCopyright(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Copyright, bool, error) {
	prediction = normalizeCamiePrediction(prediction)
	existing, err := findNativeCamieCopyright(ctx, repository, prediction)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	input := models.CopyrightCreateInput{
		Name:      prediction.Name,
		Aliases:   camieAliases(prediction),
		ParentIDs: []string{},
		ChildIDs:  []string{},
	}
	created, err := repository.Copyright.Create(ctx, input)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// enrichNativeCamiePredictionTargets is the target-enrichment path used by
// metadata review now that Copyright is independent from Tags.
func enrichNativeCamiePredictionTargets(ctx context.Context, predictions []camietagger.Tag) []camietagger.Tag {
	repository := manager.GetInstance().Repository
	result := make([]camietagger.Tag, len(predictions))
	copy(result, predictions)

	_ = repository.WithTxn(ctx, func(ctx context.Context) error {
		for index := range result {
			prediction := normalizeCamiePrediction(result[index])
			var targetID int
			switch prediction.Category {
			case "character":
				performerEntity, _ := findCamiePerformerPredictionWithCopyrightContext(ctx, repository, prediction, result)
				if performerEntity != nil {
					targetID = performerEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/performers/%d", targetID)
					prediction.TargetCandidates = nil
				} else {
					// A bare Character with multiple existing same-name Characters is
					// genuinely ambiguous when Copyright context cannot resolve it.
					// Return those candidates to the review UI instead of silently
					// choosing a database row or waiting until apply to fail.
					candidates, _ := findCamieCharacterTargetCandidates(ctx, repository, prediction)
					prediction.TargetCandidates = candidates

					// If a disambiguated prediction has no exact match but a bare
					// same-name Character exists, expose that Character as a possible
					// match. TargetExists deliberately remains false so the review UI
					// must ask before reusing it. Differently-disambiguated Characters
					// are excluded by findCamieBarePerformerSuggestion.
					suggestion, _ := findCamieBarePerformerSuggestion(ctx, repository, prediction)
					if suggestion != nil {
						prediction.TargetPath = fmt.Sprintf("/performers/%d", suggestion.ID)
					} else {
						characterName, _ := camieCharacterIdentity(prediction)
						prediction.TargetPath = "/performers?q=" + url.QueryEscape(characterName)
					}
				}
			case "artist":
				studioEntity, _ := findCamieStudioPrediction(ctx, repository, prediction)
				if studioEntity != nil {
					targetID = studioEntity.ID
					// Display the canonical Artist while keeping the source value as
					// an alias beside it in the review row. Exact case-insensitive
					// name matches do not need a redundant alias label.
					if strings.EqualFold(strings.TrimSpace(prediction.RawName), studioEntity.Name) {
						prediction.RawName = ""
					}
					prediction.Name = studioEntity.Name
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/studios/%d", targetID)
				} else {
					prediction.TargetPath = "/studios?q=" + url.QueryEscape(prediction.Name)
				}
			case "copyright":
				copyrightEntity, _ := findNativeCamieCopyright(ctx, repository, prediction)
				if copyrightEntity != nil {
					targetID = copyrightEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/copyrights/%d", targetID)
				} else {
					prediction.TargetPath = "/copyrights?q=" + url.QueryEscape(prediction.Name)
				}
			default:
				tagEntity, _ := findCamieTag(ctx, repository, prediction)
				if tagEntity != nil {
					targetID = tagEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/tags/%d", targetID)
				} else {
					prediction.TargetPath = "/tags?q=" + url.QueryEscape(prediction.Name)
				}
			}
			prediction.TargetExists = targetID > 0
			result[index] = prediction
		}
		return nil
	})
	return result
}
