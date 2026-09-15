package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type sceneTaggingApplyResponse struct {
	SceneID           int                  `json:"sceneID"`
	Characters        []camieAppliedEntity `json:"characters"`
	Artists           []camieAppliedEntity `json:"artists"`
	Copyrights        []camieAppliedEntity `json:"copyrights"`
	Tags              []camieAppliedEntity `json:"tags"`
	CreatedCharacters int                  `json:"createdCharacters"`
	CreatedArtists    int                  `json:"createdArtists"`
	CreatedCopyrights int                  `json:"createdCopyrights"`
	CreatedTags       int                  `json:"createdTags"`
}

func sceneTaggingPrimaryPath(scene *models.Scene) (string, error) {
	primary := scene.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		return "", fmt.Errorf("video has no primary file")
	}
	return primary.Base().Path, nil
}

// SceneLocalMetadata returns only metadata derived from the video's local
// filename. Opening Video Tagging never starts Camie or EVA02 inference.
func (rs sceneRoutes) SceneLocalMetadata(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	path, err := sceneTaggingPrimaryPath(scene)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	predictions := []camietagger.Tag{}
	if config.FilenameEnabled {
		predictions, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing local filename metadata: %v", err), http.StatusBadRequest)
			return
		}
	}
	predictions = enrichNativeCamiePredictionTargets(r.Context(), predictions)

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   "local",
		Model:     "local-filename",
		Threshold: 1,
		Limit:     len(predictions),
		Tags:      predictions,
	})
}

// SceneBooruMetadata performs the same filename/file MD5 lookup used by Image
// Tagging. Supported boorus can contain video posts, so no image transcoding is
// required: the original video's MD5 is the post identity.
func (rs sceneRoutes) SceneBooruMetadata(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	path, err := sceneTaggingPrimaryPath(scene)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	provider, post, hash, hashSource, err := lookupImageBooruMetadata(r.Context(), path, lookupBooruPost)
	if errors.Is(err, errBooruNoMatch) {
		http.Error(w, "no matching booru post found for video MD5", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	categoryCtx, cancel := context.WithTimeout(r.Context(), booruRequestTimeout)
	categories := resolveBooruTagCategories(categoryCtx, provider, post)
	cancel()

	predictions := booruTags(post, provider.name, categories)
	for index := range predictions {
		predictions[index] = normalizeCamiePrediction(predictions[index])
	}
	predictions = enrichNativeCamiePredictionTargets(r.Context(), predictions)

	postURL := ""
	if post.ID != "" && provider.postURL != nil {
		postURL = provider.postURL(post.ID)
	}
	writeVisualSimilarityJSON(w, booruMetadataResponse{
		Source:    provider.name,
		PostID:    post.ID,
		PostURL:   postURL,
		MD5:       hash,
		MD5Source: hashSource,
		Tags:      predictions,
	})
}

func (rs sceneRoutes) SceneKnowledgeTags(w http.ResponseWriter, r *http.Request) {
	if rawBatch := strings.TrimSpace(r.URL.Query().Get("batch")); rawBatch == "1" || strings.EqualFold(rawBatch, "true") {
		handleSceneTaggingBatch(w, r)
		return
	}

	scene := r.Context().Value(sceneKey).(*models.Scene)
	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Video Tagging selection: %v", err), http.StatusBadRequest)
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

	response, err := applySceneTaggingMetadata(r.Context(), scene.ID, selected, request.ReplaceArtist)
	if err != nil {
		http.Error(w, fmt.Sprintf("applying Video Tagging metadata to video %d: %v", scene.ID, err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}

func applySceneTaggingMetadata(ctx context.Context, sceneID int, predictions []camietagger.Tag, replaceArtists bool) (sceneTaggingApplyResponse, error) {
	response := sceneTaggingApplyResponse{
		SceneID:    sceneID,
		Characters: []camieAppliedEntity{},
		Artists:    []camieAppliedEntity{},
		Copyrights: []camieAppliedEntity{},
		Tags:       []camieAppliedEntity{},
	}
	repository := manager.GetInstance().Repository

	var characters, artists, copyrights, metadataTags []camietagger.Tag
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		switch prediction.Category {
		case "character":
			characters = append(characters, prediction)
		case "artist":
			artists = append(artists, prediction)
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
			entity, created, err := findOrCreateCamiePerformerPredictionWithCopyrightContext(ctx, repository, prediction, predictions)
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

		artistIDs := make([]int, 0, len(artists))
		for _, prediction := range artists {
			entity, created, err := findOrCreateCamieStudioPrediction(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving artist %q: %w", prediction.Name, err)
			}
			artistIDs = append(artistIDs, entity.ID)
			response.Artists = append(response.Artists, camieAppliedEntity{
				ID: entity.ID, Name: entity.Name, Category: prediction.Category,
				Score: prediction.Score, Created: created,
			})
			if created {
				response.CreatedArtists++
			}
		}

		copyrightIDs := make([]int, 0, len(copyrights))
		for _, prediction := range copyrights {
			entity, created, err := findOrCreateNativeCamieCopyright(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving copyright %q: %w", prediction.Name, err)
			}
			copyrightIDs = append(copyrightIDs, entity.ID)
			response.Copyrights = append(response.Copyrights, camieAppliedEntity{
				ID: entity.ID, Name: entity.Name, Category: prediction.Category,
				Score: prediction.Score, Created: created,
			})
			if created {
				response.CreatedCopyrights++
			}
		}

		tagIDs := make([]int, 0, len(metadataTags))
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
		if changed {
			if _, err := repository.Scene.UpdatePartial(ctx, sceneID, partial); err != nil {
				return err
			}
		}

		if len(artistIDs) > 0 {
			if replaceArtists {
				if err := repository.SceneArtist.SetSceneArtists(ctx, sceneID, artistIDs); err != nil {
					return err
				}
			} else if err := repository.SceneArtist.AddSceneArtists(ctx, sceneID, artistIDs); err != nil {
				return err
			}
		}
		if len(copyrightIDs) > 0 {
			if err := repository.Copyright.AddSceneCopyrights(ctx, sceneID, copyrightIDs); err != nil {
				return err
			}
		}
		if err := linkCamieCharacterCopyrights(ctx, repository, characters, performerIDs, copyrightIDs); err != nil {
			return fmt.Errorf("linking Character Copyrights: %w", err)
		}
		return nil
	})

	return response, err
}