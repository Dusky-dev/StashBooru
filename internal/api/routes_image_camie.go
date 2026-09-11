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
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/performer"
	"github.com/stashapp/stash/pkg/studio"
	"github.com/stashapp/stash/pkg/tag"
)

const maxCamieApplyTags = 500

type camieStatusResponse struct {
	Installed      bool   `json:"installed"`
	Loaded         bool   `json:"loaded"`
	WorkerOK       bool   `json:"workerOK"`
	WorkerError    string `json:"workerError,omitempty"`
	Backend        string `json:"backend"`
	RemoteURL      string `json:"remoteURL,omitempty"`
	ModelExists    bool   `json:"modelExists"`
	MetadataExists bool   `json:"metadataExists"`
	ModelPath      string `json:"modelPath,omitempty"`
	MetadataPath   string `json:"metadataPath,omitempty"`
	Model          string `json:"model"`
	TagCount       int    `json:"tagCount"`
}

type camieTagsResponse struct {
	Backend   string            `json:"backend"`
	Model     string            `json:"model"`
	Threshold float64           `json:"threshold"`
	Limit     int               `json:"limit"`
	Tags      []camietagger.Tag `json:"tags"`
}

type camieApplyRequest struct {
	Tags          []camietagger.Tag `json:"tags"`
	ReplaceArtist bool               `json:"replaceArtist"`
}

type camieAppliedEntity struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
	Created  bool    `json:"created"`
}

type camieApplyResponse struct {
	ImageID           int                  `json:"imageID"`
	Characters        []camieAppliedEntity `json:"characters"`
	Artist            *camieAppliedEntity  `json:"artist,omitempty"`
	Tags              []camieAppliedEntity `json:"tags"`
	SkippedArtists    []camietagger.Tag    `json:"skippedArtists,omitempty"`
	PreservedArtist   bool                 `json:"preservedArtist"`
	CreatedCharacters int                  `json:"createdCharacters"`
	CreatedArtists    int                  `json:"createdArtists"`
	CreatedTags       int                  `json:"createdTags"`
}

func newCamieTagger() (camietagger.Tagger, string, string, error) {
	config, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		return nil, "local", "", err
	}
	if strings.TrimSpace(config.URL) == "" {
		return camietagger.New(""), "local", "", nil
	}

	client, err := camietagger.NewRemote(config.URL, config.Token)
	if err != nil {
		return nil, "remote", config.URL, err
	}
	return client, "remote", config.URL, nil
}

func (rs imageRoutes) CamieStatus(w http.ResponseWriter, r *http.Request) {
	response := camieStatusResponse{
		Backend: "local",
		Model:   camietagger.Model,
	}

	client, backend, remoteURL, err := newCamieTagger()
	response.Backend = backend
	response.RemoteURL = remoteURL
	if err != nil {
		response.WorkerError = err.Error()
		writeVisualSimilarityJSON(w, response)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		response.WorkerError = err.Error()
		writeVisualSimilarityJSON(w, response)
		return
	}

	response.WorkerOK = true
	response.Installed = status.Installed
	response.Loaded = status.Loaded
	response.ModelExists = status.ModelExists
	response.MetadataExists = status.MetadataExists
	response.ModelPath = status.ModelPath
	response.MetadataPath = status.MetadataPath
	response.Model = status.Model
	response.TagCount = status.TagCount
	response.WorkerError = status.Error
	writeVisualSimilarityJSON(w, response)
}

func (rs imageRoutes) ImageKnowledgeTags(w http.ResponseWriter, r *http.Request) {
	if raw := strings.TrimSpace(r.URL.Query().Get("apply")); raw == "1" || strings.EqualFold(raw, "true") {
		rs.ImageKnowledgeTagsApply(w, r)
		return
	}

	threshold := camietagger.DefaultThreshold
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed >= 1 {
			http.Error(w, "threshold must be a number greater than 0 and less than 1", http.StatusBadRequest)
			return
		}
		threshold = parsed
	}

	limit := camietagger.DefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > camietagger.MaxLimit {
			http.Error(w, fmt.Sprintf("limit must be between 1 and %d", camietagger.MaxLimit), http.StatusBadRequest)
			return
		}
		limit = parsed
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
		message := fmt.Sprintf(
			"Camie Tagger v2 is optional and is not installed. Place camie-tagger-v2.onnx at %s and camie-tagger-v2-metadata.json at %s",
			status.ModelPath,
			status.MetadataPath,
		)
		if status.Error != "" {
			message += ": " + status.Error
		}
		http.Error(w, message, http.StatusConflict)
		return
	}

	tags, err := client.Tag(r.Context(), primary.Base().Path, threshold, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image %d with %s Camie worker: %v", image.ID, backend, err), http.StatusBadGateway)
		return
	}

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   backend,
		Model:     camietagger.Model,
		Threshold: threshold,
		Limit:     limit,
		Tags:      tags,
	})
}

func (rs imageRoutes) ImageKnowledgeTagsApply(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)

	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Camie metadata selection: %v", err), http.StatusBadRequest)
		return
	}

	selected, err := validateCamieApplyTags(request.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(selected) == 0 {
		http.Error(w, "select at least one Camie prediction to apply", http.StatusBadRequest)
		return
	}

	response, err := applyCamieMetadata(r.Context(), image.ID, selected, request.ReplaceArtist)
	if err != nil {
		http.Error(w, fmt.Sprintf("applying Camie metadata to image %d: %v", image.ID, err), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, response)
}

func validateCamieApplyTags(tags []camietagger.Tag) ([]camietagger.Tag, error) {
	if len(tags) > maxCamieApplyTags {
		return nil, fmt.Errorf("cannot apply more than %d Camie predictions at once", maxCamieApplyTags)
	}

	selected := make([]camietagger.Tag, 0, len(tags))
	seen := make(map[string]int, len(tags))
	for _, prediction := range tags {
		prediction.Name = strings.TrimSpace(prediction.Name)
		prediction.Category = strings.ToLower(strings.TrimSpace(prediction.Category))
		if prediction.Name == "" {
			return nil, fmt.Errorf("Camie prediction name cannot be empty")
		}
		if len(prediction.Name) > 512 {
			return nil, fmt.Errorf("Camie prediction name is too long")
		}
		if prediction.Category == "" {
			prediction.Category = "general"
		}
		if prediction.Score < 0 || prediction.Score > 1 || math.IsNaN(prediction.Score) || math.IsInf(prediction.Score, 0) {
			return nil, fmt.Errorf("invalid Camie score %v for %q", prediction.Score, prediction.Name)
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

func applyCamieMetadata(ctx context.Context, imageID int, predictions []camietagger.Tag, replaceArtist bool) (camieApplyResponse, error) {
	response := camieApplyResponse{ImageID: imageID}
	repository := manager.GetInstance().Repository

	var characterPredictions []camietagger.Tag
	var metadataTagPredictions []camietagger.Tag
	var chosenArtist *camietagger.Tag
	for _, prediction := range predictions {
		switch prediction.Category {
		case "character":
			characterPredictions = append(characterPredictions, prediction)
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
		default:
			metadataTagPredictions = append(metadataTagPredictions, prediction)
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

		performerIDs := make([]int, 0, len(characterPredictions))
		for _, prediction := range characterPredictions {
			entity, created, err := findOrCreateCamiePerformer(ctx, repository, prediction.Name)
			if err != nil {
				return fmt.Errorf("resolving character %q: %w", prediction.Name, err)
			}
			performerIDs = append(performerIDs, entity.ID)
			response.Characters = append(response.Characters, camieAppliedEntity{
				ID:       entity.ID,
				Name:     entity.Name,
				Category: prediction.Category,
				Score:    prediction.Score,
				Created:  created,
			})
			if created {
				response.CreatedCharacters++
			}
		}

		tagIDs := make([]int, 0, len(metadataTagPredictions))
		for _, prediction := range metadataTagPredictions {
			entity, created, err := findOrCreateCamieTag(ctx, repository, prediction.Name)
			if err != nil {
				return fmt.Errorf("resolving %s tag %q: %w", prediction.Category, prediction.Name, err)
			}
			tagIDs = append(tagIDs, entity.ID)
			response.Tags = append(response.Tags, camieAppliedEntity{
				ID:       entity.ID,
				Name:     entity.Name,
				Category: prediction.Category,
				Score:    prediction.Score,
				Created:  created,
			})
			if created {
				response.CreatedTags++
			}
		}

		var artistID *int
		if chosenArtist != nil {
			if currentImage.StudioID != nil && !replaceArtist {
				response.PreservedArtist = true
				response.SkippedArtists = append(response.SkippedArtists, *chosenArtist)
			} else {
				entity, created, err := findOrCreateCamieStudio(ctx, repository, chosenArtist.Name)
				if err != nil {
					return fmt.Errorf("resolving artist %q: %w", chosenArtist.Name, err)
				}
				artistID = &entity.ID
				response.Artist = &camieAppliedEntity{
					ID:       entity.ID,
					Name:     entity.Name,
					Category: chosenArtist.Category,
					Score:    chosenArtist.Score,
					Created:  created,
				}
				if created {
					response.CreatedArtists++
				}
			}
		}

		partial := models.NewImagePartial()
		changed := false
		if len(performerIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{
				IDs:  performerIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if len(tagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{
				IDs:  tagIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
			changed = true
		}
		if artistID != nil {
			partial.StudioID = models.NewOptionalInt(*artistID)
			changed = true
		}
		if changed {
			if _, err := repository.Image.UpdatePartial(ctx, imageID, partial); err != nil {
				return err
			}
		}
		return nil
	})

	return response, err
}

func findOrCreateCamiePerformer(ctx context.Context, repository models.Repository, name string) (*models.Performer, bool, error) {
	matches, err := repository.Performer.FindByNames(ctx, []string{name}, true)
	if err != nil {
		return nil, false, err
	}
	if len(matches) > 0 {
		return matches[0], false, nil
	}

	newPerformer := models.NewPerformer()
	newPerformer.Name = name
	newPerformer.Aliases = models.NewRelatedStrings([]string{})
	newPerformer.URLs = models.NewRelatedStrings([]string{})
	if err := performer.ValidateCreate(ctx, newPerformer, repository.Performer); err != nil {
		return nil, false, err
	}
	input := &models.CreatePerformerInput{Performer: &newPerformer}
	if err := repository.Performer.Create(ctx, input); err != nil {
		return nil, false, err
	}
	return &newPerformer, true, nil
}

func findOrCreateCamieStudio(ctx context.Context, repository models.Repository, name string) (*models.Studio, bool, error) {
	existing, err := repository.Studio.FindByName(ctx, name, true)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	newStudio := models.NewCreateStudioInput()
	newStudio.Name = name
	newStudio.Aliases = models.NewRelatedStrings([]string{})
	newStudio.URLs = models.NewRelatedStrings([]string{})
	if err := studio.ValidateCreate(ctx, newStudio, repository.Studio); err != nil {
		return nil, false, err
	}
	if err := repository.Studio.Create(ctx, &newStudio); err != nil {
		return nil, false, err
	}
	return newStudio.Studio, true, nil
}

func findOrCreateCamieTag(ctx context.Context, repository models.Repository, name string) (*models.Tag, bool, error) {
	existing, err := repository.Tag.FindByName(ctx, name, true)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		existing, err = repository.Tag.FindByAlias(ctx, name, true)
		if err != nil {
			return nil, false, err
		}
	}
	if existing != nil {
		return existing, false, nil
	}

	newTag := models.CreateTagInput{Tag: &models.Tag{}}
	*newTag.Tag = models.NewTag()
	newTag.Name = name
	newTag.Aliases = models.NewRelatedStrings([]string{})
	if err := tag.ValidateCreate(ctx, *newTag.Tag, repository.Tag); err != nil {
		return nil, false, err
	}
	if err := repository.Tag.Create(ctx, &newTag); err != nil {
		return nil, false, err
	}
	return newTag.Tag, true, nil
}
