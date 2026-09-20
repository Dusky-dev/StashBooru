package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/visualembedding"
)

func imageTaggingPrimaryPath(r *http.Request) (string, error) {
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		return "", fmt.Errorf("image has no primary file")
	}
	return primary.Base().Path, nil
}

// ImageLocalMetadata returns only metadata derived from the image's local
// filename. It deliberately performs no model inference so opening Image
// Tagging can show Local metadata without starting Camie or EVA02.
func (rs imageRoutes) ImageLocalMetadata(w http.ResponseWriter, r *http.Request) {
	path, err := imageTaggingPrimaryPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var predictions []camietagger.Tag
	if config.FilenameEnabled {
		predictions, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing local filename metadata: %v", err), http.StatusBadRequest)
			return
		}
	}
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching local Image Tagging metadata targets: %v", err), http.StatusInternalServerError)
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

// ImageCamiePredictions runs only Camie's model predictions. Local filename
// metadata is a separate, higher-priority source in the review UI and must not
// be mixed into this response.
func (rs imageRoutes) ImageCamiePredictions(w http.ResponseWriter, r *http.Request) {
	config, err := resolveCamieRequestOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := imageTaggingPrimaryPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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

	predictions, err := client.Tag(r.Context(), path, config.Threshold, config.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image with %s Camie worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	for index := range predictions {
		if strings.TrimSpace(predictions[index].Source) == "" {
			predictions[index].Source = "model"
		}
	}
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching Camie Image Tagging targets: %v", err), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   backend,
		Model:     camietagger.Model,
		Threshold: config.Threshold,
		Limit:     config.Limit,
		Tags:      predictions,
	})
}

func resolveEva02TagOptions(r *http.Request) (float64, int, error) {
	threshold := visualembedding.DefaultTagThreshold
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed >= 1 {
			return 0, 0, fmt.Errorf("EVA02 threshold must be a number greater than 0 and less than 1")
		}
		threshold = parsed
	}

	limit := visualembedding.DefaultTagLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > visualembedding.MaxTagLimit {
			return 0, 0, fmt.Errorf("EVA02 per-category limit must be between 1 and %d", visualembedding.MaxTagLimit)
		}
		limit = parsed
	}
	return threshold, limit, nil
}

func eva02TagPredictionToCamie(prediction visualembedding.TagPrediction) (camietagger.Tag, bool) {
	category := strings.ToLower(strings.TrimSpace(prediction.Category))
	name := strings.TrimSpace(prediction.Name)
	if name == "" {
		return camietagger.Tag{}, false
	}
	rawName := name

	// WD category 9 contains content-rating labels. Older workers expose that
	// category as "meta"; accept both spellings so server upgrades do not depend
	// on a worker restart. "general" is the model's safe-rating label, so expose
	// it as the clearer booru-style "safe" Tag. Do not retain "general" as an
	// alias because Tag resolution also checks RawName and could otherwise reuse
	// an unrelated Tag literally named "general".
	if category == "meta" || category == "rating" {
		switch strings.ToLower(name) {
		case "general":
			category = "rating"
			name = "safe"
			rawName = name
		case "sensitive", "questionable", "explicit":
			category = "rating"
		default:
			return camietagger.Tag{}, false
		}
	}

	if category != "general" && category != "character" && category != "rating" {
		return camietagger.Tag{}, false
	}

	return camietagger.Tag{
		Name:     name,
		RawName:  rawName,
		Category: category,
		Score:    prediction.Score,
		Source:   "eva02",
	}, true
}

// ImageEva02Predictions exposes tag logits from the already-packaged EVA02
// visual-embedding model. The visualembedding client uses the same persistent
// ONNX session for embeddings and tags; this endpoint never creates a second
// EVA02 model instance.
func (rs imageRoutes) ImageEva02Predictions(w http.ResponseWriter, r *http.Request) {
	threshold, limit, err := resolveEva02TagOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := imageTaggingPrimaryPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	client, backend, _, err := newVisualSimilarityEmbedder()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("checking %s EVA02 worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		http.Error(w, fmt.Sprintf("EVA02 visual model is not installed at %s. Install it explicitly from Visual Similarity settings first", status.ModelPath), http.StatusConflict)
		return
	}

	tagPredictions, err := client.Tag(r.Context(), path, threshold, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image with %s EVA02 worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	predictions := make([]camietagger.Tag, 0, len(tagPredictions))
	for _, prediction := range tagPredictions {
		tag, ok := eva02TagPredictionToCamie(prediction)
		if !ok {
			continue
		}
		predictions = append(predictions, tag)
	}
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching EVA02 Image Tagging targets: %v", err), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   backend,
		Model:     visualembedding.Model,
		Threshold: threshold,
		Limit:     limit,
		Tags:      predictions,
	})
}
