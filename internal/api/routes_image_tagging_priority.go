package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func camiePredictionIncludesFilenameSource(source string) bool {
	for _, part := range strings.Split(strings.ToLower(strings.TrimSpace(source)), "+") {
		if strings.TrimSpace(part) == "filename" {
			return true
		}
	}
	return false
}

// filterCamieFilenameAuthoritativeSelections mirrors mergeCamiePredictions'
// filename-authority rule at apply time. The multi-source Image Tagging dialog
// loads Local, booru, Camie, and EVA02 independently, so lower-priority
// predictions can otherwise reach findOrCreate even when a selected Local
// Character, Artist, or Copyright already owns that category.
func filterCamieFilenameAuthoritativeSelections(predictions []camietagger.Tag) []camietagger.Tag {
	authoritativeCategories := make(map[string]bool, 3)
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if camieFilenameAuthoritativeCategory(prediction.Category) && camiePredictionIncludesFilenameSource(prediction.Source) {
			authoritativeCategories[prediction.Category] = true
		}
	}
	if len(authoritativeCategories) == 0 {
		return predictions
	}

	filtered := make([]camietagger.Tag, 0, len(predictions))
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if authoritativeCategories[prediction.Category] && !camiePredictionIncludesFilenameSource(prediction.Source) {
			continue
		}
		filtered = append(filtered, prediction)
	}
	return filtered
}

// ImageKnowledgeTagsWithLocalPriorityV2 keeps the existing per-image Image
// Tagging behavior, but enforces filename-authoritative identity categories
// before applyCamieMetadataV2 can resolve or create native entities.
func (rs imageRoutes) ImageKnowledgeTagsWithLocalPriorityV2(w http.ResponseWriter, r *http.Request) {
	rawApply := strings.TrimSpace(r.URL.Query().Get("apply"))
	if rawApply != "1" && !strings.EqualFold(rawApply, "true") {
		rs.ImageKnowledgeTagsV2(w, r)
		return
	}

	image := r.Context().Value(imageKey).(*models.Image)
	var request camieApplyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Image Tagging selection: %v", err), http.StatusBadRequest)
		return
	}
	if len(request.Tags) > maxCamieApplyTags {
		http.Error(w, fmt.Sprintf("cannot apply more than %d metadata predictions at once", maxCamieApplyTags), http.StatusBadRequest)
		return
	}

	prioritized := filterCamieFilenameAuthoritativeSelections(request.Tags)
	selected, err := validateCamiePredictionsV2(prioritized)
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
