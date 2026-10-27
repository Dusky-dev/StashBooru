package api

import (
	"context"
	"net/http"

	"github.com/stashapp/stash/internal/manager"
)

type nativeMetadataMergeRequest struct {
	SourceID      int `json:"sourceID"`
	DestinationID int `json:"destinationID"`
}

func applyNativeMetadataMerge(w http.ResponseWriter, r *http.Request, kind string) {
	var input nativeMetadataMergeRequest
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if input.SourceID <= 0 || input.DestinationID <= 0 ||
		input.SourceID == input.DestinationID {
		http.Error(w, "Choose distinct existing source and destination records.", http.StatusBadRequest)
		return
	}
	repository := manager.GetInstance().Repository
	err := repository.WithTxn(r.Context(), func(ctx context.Context) error {
		switch kind {
		case "artist":
			return repository.Studio.Merge(ctx, input.SourceID, input.DestinationID)
		default:
			return repository.Copyright.Merge(ctx, input.SourceID, input.DestinationID)
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, map[string]int{"destinationID": input.DestinationID})
}

func (rs studioRoutes) Merge(w http.ResponseWriter, r *http.Request) {
	applyNativeMetadataMerge(w, r, "artist")
}

func (rs tagRoutes) CopyrightMerge(w http.ResponseWriter, r *http.Request) {
	applyNativeMetadataMerge(w, r, "copyright")
}
