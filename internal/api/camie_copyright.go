package api

import (
	"context"
	"net/http"

	"github.com/stashapp/stash/internal/manager"
)

type camieCopyrightRootResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (rs imageRoutes) CamieCopyrightRoot(w http.ResponseWriter, r *http.Request) {
	repository := manager.GetInstance().Repository
	var response camieCopyrightRootResponse
	if err := repository.WithTxn(r.Context(), func(ctx context.Context) error {
		root, err := ensureCamieCopyrightRoot(ctx, repository)
		if err != nil {
			return err
		}
		response.ID = root.ID
		response.Name = root.Name
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
