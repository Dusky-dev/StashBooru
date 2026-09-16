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
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching local Video Tagging metadata targets: %v", err), http.StatusInternalServerError)
		return
	}

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
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching booru Video Tagging targets: %v", err), http.StatusInternalServerError)
		return
	}

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
	scene := r.Context().Value(sceneKey).(*models.Scene)
	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Video Tagging selection: %v", err), http.StatusBadRequest)
		return
	}

	prioritized, suppressed := partitionCamieFilenameAuthoritativeSelections(request.Tags)
	selected, err := validateSceneTaggingPredictions(prioritized)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(selected) == 0 {
		http.Error(w, "select at least one metadata item to apply", http.StatusBadRequest)
		return
	}

	plan := buildTaggingChangePlanWithSuppressed(selected, suppressed, request.ReplaceArtist)
	if rawPreview := strings.TrimSpace(r.URL.Query().Get("preview")); rawPreview == "1" || strings.EqualFold(rawPreview, "true") {
		writeVisualSimilarityJSON(w, plan)
		return
	}
	if !plan.CanApply {
		http.Error(w, "metadata plan contains unresolved review items", http.StatusConflict)
		return
	}

	response, err := applySceneTaggingMetadata(r.Context(), scene.ID, taggingChangePlanPredictions(plan), request.ReplaceArtist)
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

	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		currentScene, err := repository.Scene.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if currentScene == nil {
			return fmt.Errorf("video %d not found", sceneID)
		}

		resolved, err := resolveTaggingEntities(ctx, repository, predictions)
		if err != nil {
			return err
		}
		response.Characters = resolved.Characters
		response.Artists = resolved.Artists
		response.Copyrights = resolved.Copyrights
		response.Tags = resolved.Tags
		response.CreatedCharacters = resolved.CreatedCharacters
		response.CreatedArtists = resolved.CreatedArtists
		response.CreatedCopyrights = resolved.CreatedCopyrights
		response.CreatedTags = resolved.CreatedTags

		partial := models.NewScenePartial()
		changed := false
		if len(resolved.CharacterIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{
				IDs: resolved.CharacterIDs, Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if len(resolved.TagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{
				IDs: resolved.TagIDs, Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if changed {
			if _, err := repository.Scene.UpdatePartial(ctx, sceneID, partial); err != nil {
				return err
			}
		}

		if len(resolved.ArtistIDs) > 0 {
			if replaceArtists {
				if err := repository.SceneArtist.SetSceneArtists(ctx, sceneID, resolved.ArtistIDs); err != nil {
					return err
				}
			} else if err := repository.SceneArtist.AddSceneArtists(ctx, sceneID, resolved.ArtistIDs); err != nil {
				return err
			}
		}
		if len(resolved.CopyrightIDs) > 0 {
			if err := repository.Copyright.AddSceneCopyrights(ctx, sceneID, resolved.CopyrightIDs); err != nil {
				return err
			}
		}
		return linkResolvedTaggingCharacterCopyrights(ctx, repository, resolved)
	})

	return response, err
}
