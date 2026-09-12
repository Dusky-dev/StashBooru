package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func (rs imageRoutes) ImageLocalTags(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !config.FilenameEnabled {
		writeVisualSimilarityJSON(w, camieTagsResponse{
			Backend: "local",
			Model:   "filename",
			Tags:    []camietagger.Tag{},
		})
		return
	}

	predictions, err := parseCamieFilename(primary.Base().Path, config.FilenameLayout)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing local filename metadata: %v", err), http.StatusBadRequest)
		return
	}
	predictions = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend: "local",
		Model:   "filename",
		Limit:   len(predictions),
		Tags:    predictions,
	})
}
