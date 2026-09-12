package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/visualembedding"
)

func (rs imageRoutes) ImageEva02Tags(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	client := visualembedding.New("")
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("checking EVA02 worker: %v", err), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		http.Error(
			w,
			"EVA02 is not installed. Install the Visual Similarity model from Settings > System > Visual Similarity first.",
			http.StatusConflict,
		)
		return
	}

	tags, err := client.Tag(r.Context(), primary.Base().Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image %d with EVA02: %v", image.ID, err), http.StatusBadGateway)
		return
	}

	predictions := make([]camietagger.Tag, 0, len(tags))
	for _, tag := range tags {
		predictions = append(predictions, camietagger.Tag{
			Name:     tag.Name,
			RawName:  tag.RawName,
			Category: tag.Category,
			Score:    tag.Score,
			Source:   "eva02",
		})
	}
	predictions = enrichNativeCamiePredictionTargets(r.Context(), predictions)

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend: "local",
		Model:   visualembedding.Model,
		Limit:   len(predictions),
		Tags:    predictions,
	})
}
