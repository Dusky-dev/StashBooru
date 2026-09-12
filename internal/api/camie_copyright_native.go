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
				performerEntity, _ := findCamiePerformerPrediction(ctx, repository, prediction)
				if performerEntity != nil {
					targetID = performerEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/performers/%d", targetID)
				} else {
					characterName, _ := camieCharacterIdentity(prediction)
					prediction.TargetPath = "/performers?q=" + url.QueryEscape(characterName)
				}
			case "artist":
				for _, name := range []string{prediction.Name, prediction.RawName} {
					if strings.TrimSpace(name) == "" {
						continue
					}
					studioEntity, _ := repository.Studio.FindByName(ctx, name, true)
					if studioEntity != nil {
						targetID = studioEntity.ID
						break
					}
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
