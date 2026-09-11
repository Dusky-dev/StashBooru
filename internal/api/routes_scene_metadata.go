package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type sceneLocalMetadataResponse struct {
	Tags []camietagger.Tag `json:"tags"`
}

type sceneMetadataApplyResponse struct {
	SceneID           int                  `json:"sceneID"`
	Characters        []camieAppliedEntity `json:"characters"`
	Artist            *camieAppliedEntity  `json:"artist,omitempty"`
	Copyrights        []camieAppliedEntity `json:"copyrights"`
	Tags              []camieAppliedEntity `json:"tags"`
	SkippedArtists    []camietagger.Tag    `json:"skippedArtists,omitempty"`
	PreservedArtist   bool                 `json:"preservedArtist"`
	CreatedCharacters int                  `json:"createdCharacters"`
	CreatedArtists    int                  `json:"createdArtists"`
	CreatedCopyrights int                  `json:"createdCopyrights"`
	CreatedTags       int                  `json:"createdTags"`
}

func scenePrimaryMetadataPath(scene *models.Scene) (string, error) {
	primary := scene.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		return "", fmt.Errorf("video has no primary file")
	}
	return primary.Base().Path, nil
}

func collectSceneLocalMetadata(path string) ([]camietagger.Tag, error) {
	config, err := loadCamieConfig()
	if err != nil {
		return nil, err
	}
	if !config.FilenameEnabled {
		return []camietagger.Tag{}, nil
	}

	predictions, err := parseCamieFilename(path, config.FilenameLayout)
	if err != nil {
		return nil, fmt.Errorf("parsing local filename metadata: %w", err)
	}
	for index := range predictions {
		predictions[index] = normalizeCamiePrediction(predictions[index])
	}
	return predictions, nil
}

func (rs sceneRoutes) SceneLocalMetadata(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	path, err := scenePrimaryMetadataPath(scene)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	predictions, err := collectSceneLocalMetadata(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	predictions = enrichCamiePredictionTargets(r.Context(), predictions)
	writeVisualSimilarityJSON(w, sceneLocalMetadataResponse{Tags: predictions})
}

func (rs sceneRoutes) SceneBooruMetadata(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	path, err := scenePrimaryMetadataPath(scene)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	response, err := lookupImageBooruMetadata(r.Context(), path)
	if err != nil {
		if err == errBooruNoMatch {
			http.Error(w, "video was not found on supported boorus", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	response.Tags = enrichCamiePredictionTargets(r.Context(), response.Tags)
	writeVisualSimilarityJSON(w, response)
}

func (rs sceneRoutes) SceneMetadataApply(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding video metadata selection: %v", err), http.StatusBadRequest)
		return
	}

	selected, err := validateCamiePredictionsV2(request.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(selected) == 0 {
		http.Error(w, "select at least one metadata item to apply", http.StatusBadRequest)
		return
	}

	response, err := applySceneMetadata(r.Context(), scene.ID, selected, request.ReplaceArtist)
	if err != nil {
		http.Error(w, fmt.Sprintf("applying metadata to video %d: %v", scene.ID, err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}

func applySceneMetadata(ctx context.Context, sceneID int, predictions []camietagger.Tag, replaceArtist bool) (sceneMetadataApplyResponse, error) {
	response := sceneMetadataApplyResponse{SceneID: sceneID}
	repository := manager.GetInstance().Repository

	var characters, copyrights, metadataTags []camietagger.Tag
	var chosenArtist *camietagger.Tag
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		switch prediction.Category {
		case "character":
			characters = append(characters, prediction)
		case "artist":
			if chosenArtist == nil || prediction.Score > chosenArtist.Score {
				if chosenArtist != nil {
					response.SkippedArtists = append(response.SkippedArtists, *chosenArtist)
				}
				candidate := prediction
				chosenArtist = &candidate
			} else {
				response.SkippedArtists = append(response.SkippedArtists, prediction)
			}
		case "copyright":
			copyrights = append(copyrights, prediction)
		default:
			metadataTags = append(metadataTags, prediction)
		}
	}

	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		currentScene, err := repository.Scene.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if currentScene == nil {
			return fmt.Errorf("video %d not found", sceneID)
		}

		performerIDs := make([]int, 0, len(characters))
		for _, prediction := range characters {
			entity, created, err := findOrCreateCamiePerformerPrediction(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving character %q: %w", prediction.Name, err)
			}
			performerIDs = append(performerIDs, entity.ID)
			response.Characters = append(response.Characters, camieAppliedEntity{
				ID: entity.ID, Name: entity.Name, Category: prediction.Category,
				Score: prediction.Score, Created: created,
			})
			if created {
				response.CreatedCharacters++
			}
		}

		tagIDs := make([]int, 0, len(copyrights)+len(metadataTags))
		for _, prediction := range copyrights {
			entity, created, err := findOrCreateCamieCopyright(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving copyright %q: %w", prediction.Name, err)
			}
			tagIDs = append(tagIDs, entity.ID)
			response.Copyrights = append(response.Copyrights, camieAppliedEntity{
				ID: entity.ID, Name: entity.Name, Category: prediction.Category,
				Score: prediction.Score, Created: created,
			})
			if created {
				response.CreatedCopyrights++
			}
		}
		for _, prediction := range metadataTags {
			entity, created, err := findOrCreateCamieTagPrediction(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving %s tag %q: %w", prediction.Category, prediction.Name, err)
			}
			tagIDs = append(tagIDs, entity.ID)
			response.Tags = append(response.Tags, camieAppliedEntity{
				ID: entity.ID, Name: entity.Name, Category: prediction.Category,
				Score: prediction.Score, Created: created,
			})
			if created {
				response.CreatedTags++
			}
		}

		var artistID *int
		if chosenArtist != nil {
			if currentScene.StudioID != nil && !replaceArtist {
				response.PreservedArtist = true
				response.SkippedArtists = append(response.SkippedArtists, *chosenArtist)
			} else {
				entity, created, err := findOrCreateCamieStudioPrediction(ctx, repository, *chosenArtist)
				if err != nil {
					return fmt.Errorf("resolving artist %q: %w", chosenArtist.Name, err)
				}
				artistID = &entity.ID
				response.Artist = &camieAppliedEntity{
					ID: entity.ID, Name: entity.Name, Category: chosenArtist.Category,
					Score: chosenArtist.Score, Created: created,
				}
				if created {
					response.CreatedArtists++
				}
			}
		}

		partial := models.NewScenePartial()
		changed := false
		if len(performerIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{
				IDs: performerIDs, Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if len(tagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{
				IDs: tagIDs, Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if artistID != nil {
			partial.StudioID = models.NewOptionalInt(*artistID)
			changed = true
		}
		if changed {
			if _, err := repository.Scene.UpdatePartial(ctx, sceneID, partial); err != nil {
				return err
			}
		return nil
	})
	return response, err
}
