package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/stashapp/stash/internal/manager"
)

type nativeMetadataMergeRequest struct {
	SourceID      int   `json:"sourceID"`
	SourceIDs     []int `json:"sourceIDs"`
	DestinationID int   `json:"destinationID"`
	// The merge dialog already chose each field's value: conflicting source
	// fields give way to the destination instead of failing the merge.
	KeepDestination bool `json:"keepDestination"`
}

// Batch merges check the hierarchy once, after every source is merged.
type batchMerger interface {
	MergeBatch(ctx context.Context, sourceIDs []int, destinationID int, keepDestination bool) error
}

func applyNativeMetadataMerge(w http.ResponseWriter, r *http.Request, kind string) {
	var input nativeMetadataMergeRequest
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	sources := input.SourceIDs
	if input.SourceID > 0 {
		sources = append([]int{input.SourceID}, sources...)
	}
	seen := map[int]bool{}
	for _, id := range sources {
		if id <= 0 || id == input.DestinationID || seen[id] {
			http.Error(w, "Choose distinct existing source and destination records.", http.StatusBadRequest)
			return
		}
		seen[id] = true
	}
	if len(sources) == 0 || input.DestinationID <= 0 {
		http.Error(w, "Choose distinct existing source and destination records.", http.StatusBadRequest)
		return
	}
	repository := manager.GetInstance().Repository
	err := repository.WithTxn(r.Context(), func(ctx context.Context) error {
		// All sources merge in one transaction: any failure keeps every record.
		store := interface{}(repository.Copyright)
		if kind == "artist" {
			store = repository.Studio
		}
		merger, ok := store.(batchMerger)
		if !ok {
			return fmt.Errorf("this repository cannot merge several records")
		}
		return merger.MergeBatch(ctx, sources, input.DestinationID, input.KeepDestination)
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
