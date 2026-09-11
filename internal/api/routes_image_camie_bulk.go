package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/txn"
)

type camieBulkRequest struct {
	ImageIDs        []int   `json:"imageIDs"`
	All             bool    `json:"all"`
	Threshold       float64 `json:"threshold"`
	Limit           int     `json:"limit"`
	ApplyCharacters bool    `json:"applyCharacters"`
	ApplyArtist     bool    `json:"applyArtist"`
	ApplyTags       bool    `json:"applyTags"`
	ReplaceArtist   bool    `json:"replaceArtist"`
}

type camieBulkImageSource struct {
	ID   int
	Path string
}

func (rs imageRoutes) CamieTagImages(w http.ResponseWriter, r *http.Request) {
	var request camieBulkRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Camie bulk tagging request: %v", err), http.StatusBadRequest)
		return
	}

	if !request.All && len(request.ImageIDs) == 0 {
		http.Error(w, "select at least one image or choose all images", http.StatusBadRequest)
		return
	}
	if request.Threshold == 0 {
		request.Threshold = camietagger.DefaultThreshold
	}
	if request.Threshold <= 0 || request.Threshold >= 1 {
		http.Error(w, "threshold must be a number greater than 0 and less than 1", http.StatusBadRequest)
		return
	}
	if request.Limit == 0 {
		request.Limit = camietagger.DefaultLimit
	}
	if request.Limit < 1 || request.Limit > camietagger.MaxLimit {
		http.Error(w, fmt.Sprintf("limit must be between 1 and %d", camietagger.MaxLimit), http.StatusBadRequest)
		return
	}
	if !request.ApplyCharacters && !request.ApplyArtist && !request.ApplyTags {
		http.Error(w, "enable at least one Camie metadata category", http.StatusBadRequest)
		return
	}

	// Copy the slice because the job outlives this HTTP request.
	request.ImageIDs = append([]int(nil), request.ImageIDs...)
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Tagging images with Camie...", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			ids, err := resolveCamieBulkImageIDs(ctx, mgr, request)
			if err != nil {
				return err
			}
			progress.SetTotal(len(ids))
			if len(ids) == 0 {
				return nil
			}

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
				return fmt.Errorf(
					"camie Tagger v2 is not installed; place camie-tagger-v2.onnx at %s and camie-tagger-v2-metadata.json at %s",
					status.ModelPath,
					status.MetadataPath,
				)
			}

			failures := 0
			for _, imageID := range ids {
				if job.IsCancelled(ctx) {
					return nil
				}

				source, err := loadCamieBulkImageSource(ctx, mgr, imageID)
				if err != nil {
					failures++
					logger.Warnf("Camie bulk tagging: loading image %d: %v", imageID, err)
					progress.Increment()
					continue
				}

				predictions, err := client.Tag(ctx, source.Path, request.Threshold, request.Limit)
				if err != nil {
					failures++
					logger.Warnf("Camie bulk tagging: %s tagging image %d (%s): %v", backend, imageID, source.Path, err)
					progress.Increment()
					continue
				}

				selected := filterCamieBulkPredictions(predictions, request)
				selected, err = validateCamieApplyTags(selected)
				if err != nil {
					failures++
					logger.Warnf("Camie bulk tagging: validating predictions for image %d: %v", imageID, err)
					progress.Increment()
					continue
				}
				if len(selected) > 0 {
					if _, err := applyCamieMetadata(ctx, imageID, selected, request.ReplaceArtist); err != nil {
						failures++
						logger.Warnf("Camie bulk tagging: applying metadata to image %d: %v", imageID, err)
						progress.Increment()
						continue
					}
				}

				progress.Increment()
			}

			if failures > 0 {
				return fmt.Errorf("camie tagging skipped %d image(s); see the logs for details", failures)
			}
			return nil
		},
	))

	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

func resolveCamieBulkImageIDs(ctx context.Context, mgr *manager.Manager, request camieBulkRequest) ([]int, error) {
	if !request.All {
		seen := make(map[int]struct{}, len(request.ImageIDs))
		ids := make([]int, 0, len(request.ImageIDs))
		for _, id := range request.ImageIDs {
			if id <= 0 {
				return nil, fmt.Errorf("invalid image id %d", id)
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		return ids, nil
	}

	var ids []int
	if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		images, err := mgr.Repository.Image.All(ctx)
		if err != nil {
			return err
		}
		ids = make([]int, 0, len(images))
		for _, image := range images {
			ids = append(ids, image.ID)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("loading images for Camie tagging: %w", err)
	}
	return ids, nil
}

func loadCamieBulkImageSource(ctx context.Context, mgr *manager.Manager, imageID int) (camieBulkImageSource, error) {
	var source camieBulkImageSource
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		image, err := mgr.Repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("image %d not found", imageID)
		}
		if err := image.LoadPrimaryFile(ctx, mgr.Repository.File); err != nil {
			return err
		}
		primary := image.Files.Primary()
		if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
			return fmt.Errorf("image %d has no primary file", imageID)
		}
		source = camieBulkImageSource{ID: imageID, Path: primary.Base().Path}
		return nil
	})
	return source, err
}

func filterCamieBulkPredictions(predictions []camietagger.Tag, request camieBulkRequest) []camietagger.Tag {
	selected := make([]camietagger.Tag, 0, len(predictions))
	for _, prediction := range predictions {
		switch strings.ToLower(strings.TrimSpace(prediction.Category)) {
		case "character":
			if request.ApplyCharacters {
				selected = append(selected, prediction)
			}
		case "artist":
			if request.ApplyArtist {
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
