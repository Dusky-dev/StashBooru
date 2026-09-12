package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type camieApplyResponseV2 struct {
	ImageID           int                  `json:"imageID"`
	Characters        []camieAppliedEntity `json:"characters"`
	Artists           []camieAppliedEntity `json:"artists"`
	Copyrights        []camieAppliedEntity `json:"copyrights"`
	Tags              []camieAppliedEntity `json:"tags"`
	CreatedCharacters int                  `json:"createdCharacters"`
	CreatedArtists    int                  `json:"createdArtists"`
	CreatedCopyrights int                  `json:"createdCopyrights"`
	CreatedTags       int                  `json:"createdTags"`
}

type camieBulkRequestV2 struct {
	ImageIDs        []int   `json:"imageIDs"`
	All             bool    `json:"all"`
	Threshold       float64 `json:"threshold"`
	Limit           int     `json:"limit"`
	ApplyCharacters bool    `json:"applyCharacters"`
	ApplyArtist     bool    `json:"applyArtist"`
	ApplyCopyright  bool    `json:"applyCopyright"`
	ApplyTags       bool    `json:"applyTags"`
	ReplaceArtist   bool    `json:"replaceArtist"`
}

func resolveCamieRequestOptions(r *http.Request) (camieConfig, error) {
	config, err := loadCamieConfig()
	if err != nil {
		return config, err
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed >= 1 {
			return config, fmt.Errorf("threshold must be a number greater than 0 and less than 1")
		}
		config.Threshold = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > camietagger.MaxLimit {
			return config, fmt.Errorf("limit must be between 1 and %d", camietagger.MaxLimit)
		}
		config.Limit = parsed
	}
	return config, nil
}

func collectCamiePredictions(ctx context.Context, client camietagger.Tagger, path string, config camieConfig) ([]camietagger.Tag, error) {
	modelPredictions, err := client.Tag(ctx, path, config.Threshold, config.Limit)
	if err != nil {
		return nil, err
	}
	var filenamePredictions []camietagger.Tag
	if config.FilenameEnabled {
		filenamePredictions, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			return nil, fmt.Errorf("parsing filename metadata: %w", err)
		}
	}
	return mergeCamiePredictions(modelPredictions, filenamePredictions), nil
}

func (rs imageRoutes) ImageKnowledgeTagsV2(w http.ResponseWriter, r *http.Request) {
	if raw := strings.TrimSpace(r.URL.Query().Get("apply")); raw == "1" || strings.EqualFold(raw, "true") {
		rs.ImageKnowledgeTagsApplyV2(w, r)
		return
	}

	config, err := resolveCamieRequestOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	client, backend, _, err := newCamieTagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("checking %s Camie worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		http.Error(w, fmt.Sprintf("Camie Tagger v2 is not installed. Place camie-tagger-v2.onnx at %s and camie-tagger-v2-metadata.json at %s", status.ModelPath, status.MetadataPath), http.StatusConflict)
		return
	}

	predictions, err := collectCamiePredictions(r.Context(), client, primary.Base().Path, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image %d with %s Camie worker: %v", image.ID, backend, err), http.StatusBadGateway)
		return
	}
	predictions = enrichNativeCamiePredictionTargets(r.Context(), predictions)

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   backend,
		Model:     camietagger.Model,
		Threshold: config.Threshold,
		Limit:     config.Limit,
		Tags:      predictions,
	})
}

func (rs imageRoutes) ImageKnowledgeTagsApplyV2(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)
	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Image Tagging selection: %v", err), http.StatusBadRequest)
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
	response, err := applyCamieMetadataV2(r.Context(), image.ID, selected, request.ReplaceArtist)
	if err != nil {
		http.Error(w, fmt.Sprintf("applying Image Tagging metadata to image %d: %v", image.ID, err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}

func validateCamiePredictionsV2(predictions []camietagger.Tag) ([]camietagger.Tag, error) {
	if len(predictions) > maxCamieApplyTags {
		return nil, fmt.Errorf("cannot apply more than %d metadata predictions at once", maxCamieApplyTags)
	}
	selected := make([]camietagger.Tag, 0, len(predictions))
	seen := make(map[string]int, len(predictions))
	for _, prediction := range predictions {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name == "" {
			return nil, fmt.Errorf("metadata prediction name cannot be empty")
		}
		if len(prediction.Name) > 512 {
			return nil, fmt.Errorf("metadata prediction name is too long")
		}
		if prediction.Score < 0 || prediction.Score > 1 || math.IsNaN(prediction.Score) || math.IsInf(prediction.Score, 0) {
			return nil, fmt.Errorf("invalid metadata score %v for %q", prediction.Score, prediction.Name)
		}
		key := prediction.Category + "\x00" + strings.ToLower(prediction.Name)
		if index, ok := seen[key]; ok {
			if prediction.Score > selected[index].Score {
				selected[index] = prediction
			}
			continue
		}
		seen[key] = len(selected)
		selected = append(selected, prediction)
	}
	return selected, nil
}

func applyCamieMetadataV2(ctx context.Context, imageID int, predictions []camietagger.Tag, replaceArtists bool) (camieApplyResponseV2, error) {
	response := camieApplyResponseV2{ImageID: imageID}
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
		currentImage, err := repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if currentImage == nil {
			return fmt.Errorf("image %d not found", imageID)
		}

		performerIDs := make([]int, 0, len(characters))
		for _, prediction := range characters {
			entity, created, err := findOrCreateCamiePerformerPrediction(ctx, repository, prediction)
			if err != nil {
				return fmt.Errorf("resolving character %q: %w", prediction.Name, err)
			}
			performerIDs = append(performerIDs, entity.ID)
			response.Characters = append(response.Characters, camieAppliedEntity{ID: entity.ID, Name: entity.Name, Category: prediction.Category, Score: prediction.Score, Created: created})
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
			response.Artists = append(response.Artists, camieAppliedEntity{ID: entity.ID, Name: entity.Name, Category: prediction.Category, Score: prediction.Score, Created: created})
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
			response.Copyrights = append(response.Copyrights, camieAppliedEntity{ID: entity.ID, Name: entity.Name, Category: prediction.Category, Score: prediction.Score, Created: created})
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
			response.Tags = append(response.Tags, camieAppliedEntity{ID: entity.ID, Name: entity.Name, Category: prediction.Category, Score: prediction.Score, Created: created})
			if created {
				response.CreatedTags++
			}
		}

		partial := models.NewImagePartial()
		changed := false
		if len(performerIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{IDs: performerIDs, Mode: models.RelationshipUpdateModeAdd}
			changed = true
		}
		if len(tagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{IDs: tagIDs, Mode: models.RelationshipUpdateModeAdd}
			changed = true
		}
		if changed {
			if _, err := repository.Image.UpdatePartial(ctx, imageID, partial); err != nil {
				return err
			}
		}
		if len(artistIDs) > 0 {
			if replaceArtists {
				if err := repository.ImageArtist.SetImageArtists(ctx, imageID, artistIDs); err != nil {
					return err
				}
			} else if err := repository.ImageArtist.AddImageArtists(ctx, imageID, artistIDs); err != nil {
				return err
			}
		}
		if len(copyrightIDs) > 0 {
			if err := repository.Copyright.AddImageCopyrights(ctx, imageID, copyrightIDs); err != nil {
				return err
			}
		}
		return nil
	})
	return response, err
}

func (rs imageRoutes) CamieTagImagesV2(w http.ResponseWriter, r *http.Request) {
	var request camieBulkRequestV2
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Image Tagging bulk request: %v", err), http.StatusBadRequest)
		return
	}
	if !request.All && len(request.ImageIDs) == 0 {
		http.Error(w, "select at least one image or choose all images", http.StatusBadRequest)
		return
	}
	if request.Threshold == 0 {
		config, err := loadCamieConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		request.Threshold = config.Threshold
		request.Limit = config.Limit
	}
	if request.Threshold <= 0 || request.Threshold >= 1 {
		http.Error(w, "threshold must be a number greater than 0 and less than 1", http.StatusBadRequest)
		return
	}
	if request.Limit < 1 || request.Limit > camietagger.MaxLimit {
		http.Error(w, fmt.Sprintf("limit must be between 1 and %d", camietagger.MaxLimit), http.StatusBadRequest)
		return
	}
	if !request.ApplyCharacters && !request.ApplyArtist && !request.ApplyCopyright && !request.ApplyTags {
		http.Error(w, "enable at least one Image Tagging metadata category", http.StatusBadRequest)
		return
	}
	if err := updateCamieInferenceDefaults(request.Threshold, request.Limit); err != nil {
		http.Error(w, fmt.Sprintf("saving Camie inference defaults: %v", err), http.StatusInternalServerError)
		return
	}

	request.ImageIDs = append([]int(nil), request.ImageIDs...)
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Image tagging...", job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		ids, err := resolveCamieBulkImageIDs(ctx, mgr, camieBulkRequest{ImageIDs: request.ImageIDs, All: request.All})
		if err != nil {
			return err
		}
		progress.SetTotal(len(ids))
		if len(ids) == 0 {
			return nil
		}

		config, err := loadCamieConfig()
		if err != nil {
			return err
		}
		config.Threshold = request.Threshold
		config.Limit = request.Limit

		client, backend, _, err := newCamieTagger()
		if err != nil {
			return fmt.Errorf("initializing %s Camie worker: %w", backend, err)
		}
		defer client.Close()
		status, err := client.Status(ctx)
		if err != nil {
			return fmt.Errorf("checking %s Camie worker: %w", backend, err)
		}
		if !status.Installed {
			return fmt.Errorf("camie tagger v2 is not installed; place camie-tagger-v2.onnx at %s and camie-tagger-v2-metadata.json at %s", status.ModelPath, status.MetadataPath)
		}

		failures := 0
		for _, imageID := range ids {
			if job.IsCancelled(ctx) {
				return nil
			}
			source, err := loadCamieBulkImageSource(ctx, mgr, imageID)
			if err != nil {
				failures++
				logger.Warnf("Image tagging (Camie): loading image %d: %v", imageID, err)
				progress.Increment()
				continue
			}
			predictions, err := collectCamiePredictions(ctx, client, source.Path, config)
			if err != nil {
				failures++
				logger.Warnf("Image tagging (Camie): %s tagging image %d (%s): %v", backend, imageID, source.Path, err)
				progress.Increment()
				continue
			}
			filtered := filterCamieBulkPredictionsV2(predictions, request)
			filtered, err = validateCamiePredictionsV2(filtered)
			if err != nil {
				failures++
				logger.Warnf("Image tagging: validating predictions for image %d: %v", imageID, err)
				progress.Increment()
				continue
			}
			if len(filtered) > 0 {
				if _, err := applyCamieMetadataV2(ctx, imageID, filtered, request.ReplaceArtist); err != nil {
					failures++
					logger.Warnf("Image tagging: applying metadata to image %d: %v", imageID, err)
					progress.Increment()
					continue
				}
			}
			progress.Increment()
		}
		if failures > 0 {
			return fmt.Errorf("image tagging skipped %d image(s); see the logs for details", failures)
		}
		return nil
	}))
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

func filterCamieBulkPredictionsV2(predictions []camietagger.Tag, request camieBulkRequestV2) []camietagger.Tag {
	selected := make([]camietagger.Tag, 0, len(predictions))
	for _, prediction := range predictions {
		prediction = normalizeCamiePrediction(prediction)
		switch prediction.Category {
		case "character":
			if request.ApplyCharacters {
				selected = append(selected, prediction)
			}
		case "artist":
			if request.ApplyArtist {
				selected = append(selected, prediction)
			}
		case "copyright":
			if request.ApplyCopyright {
				selected = append(selected, prediction)
			}
		default:
			if request.ApplyTags {
				selected = append(selected, prediction)
			}
		}
	}
	return selected
}
