package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

// handleMediaFileOpen resolves the database file ID stored in conversion and
// upscaling journals back to the current owning Image or Video entry. Restore
// history can therefore link filenames without confusing file IDs with media IDs.
func handleMediaFileOpen(w http.ResponseWriter, r *http.Request) {
	value, err := strconv.Atoi(chi.URLParam(r, "fileId"))
	if err != nil || value <= 0 {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}
	fileID := models.FileID(value)
	mgr := manager.GetInstance()
	var destination string
	err = txn.WithReadTxn(r.Context(), mgr.Repository.TxnManager, func(ctx context.Context) error {
		images, err := mgr.Repository.Image.FindByFileID(ctx, fileID)
		if err != nil {
			return err
		}
		scenes, err := mgr.Repository.Scene.FindByFileID(ctx, fileID)
		if err != nil {
			return err
		}
		if len(images)+len(scenes) != 1 {
			return fmt.Errorf("file is not owned by exactly one media entry")
		}
		if len(images) == 1 {
			destination = fmt.Sprintf("/images/%d", images[0].ID)
		} else {
			destination = fmt.Sprintf("/scenes/%d", scenes[0].ID)
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}
