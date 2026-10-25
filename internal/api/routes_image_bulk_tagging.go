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
	"github.com/stashapp/stash/pkg/visualembedding"
)

// Keep the legacy Camie-only endpoint intact for existing callers. The new
// image-only job is opt-in and requires explicit IDs and one confirmation.
type bulkImageTagRequest struct {
	camieBulkRequestV2
	Confirmed      bool    `json:"confirmed"`
	UseCamie       bool    `json:"useCamie"`
	UseEva02       bool    `json:"useEva02"`
	Eva02Threshold float64 `json:"eva02Threshold"`
}

func (rs imageRoutes) BulkTagImages(w http.ResponseWriter, r *http.Request) {
	var request bulkImageTagRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Image bulk tagging request: %v", err), http.StatusBadRequest)
		return
	}
	if !request.Confirmed {
		http.Error(w, "confirm this bulk tagging selection before starting", http.StatusBadRequest)
		return
	}
	if request.All || len(request.ImageIDs) < 2 || len(request.ImageIDs) > 20000 {
		http.Error(w, "select between 2 and 20000 explicit Image IDs for bulk tagging", http.StatusBadRequest)
		return
	}
	if !request.UseCamie && !request.UseEva02 {
		http.Error(w, "enable Camie, EVA02 or both for bulk tagging", http.StatusBadRequest)
		return
	}
	if !request.ApplyCharacters && !request.ApplyArtist && !request.ApplyCopyright && !request.ApplyTags {
		http.Error(w, "select at least one Image metadata category", http.StatusBadRequest)
		return
	}
	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("loading Image tagging settings: %v", err), http.StatusInternalServerError)
		return
	}
	if !config.BulkTaggingEnabled {
		http.Error(w, "bulk auto-tagging is disabled in tagging settings", http.StatusConflict)
		return
	}
	if request.Threshold == 0 {
		request.Threshold = config.Threshold
	}
	if request.Eva02Threshold == 0 {
		request.Eva02Threshold = config.Eva02Threshold
	}
	if request.Limit == 0 {
		request.Limit = config.Limit
	}
	if request.Threshold <= 0 || request.Threshold >= 1 || request.Eva02Threshold <= 0 || request.Eva02Threshold >= 1 {
		http.Error(w, "Camie and EVA02 thresholds must be greater than 0 and less than 1", http.StatusBadRequest)
		return
	}
	if request.Limit < 1 || request.Limit > camietagger.MaxLimit {
		http.Error(w, fmt.Sprintf("per-category limit must be between 1 and %d", camietagger.MaxLimit), http.StatusBadRequest)
		return
	}

	request.ImageIDs = append([]int(nil), request.ImageIDs...)
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Bulk Image tagging...", job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		ids, err := resolveCamieBulkImageIDs(ctx, mgr, camieBulkSelection{ImageIDs: request.ImageIDs})
		if err != nil {
			return err
		}
		progress.SetTotal(len(ids))

		var camieClient camietagger.Tagger
		var evaClient visualembedding.Embedder
		if request.UseCamie {
			camieClient, _, _, err = newCamieTagger()
			if err != nil {
				return fmt.Errorf("starting Camie tagging worker: %w", err)
			}
			defer camieClient.Close()
			status, statusErr := camieClient.Status(ctx)
			if statusErr != nil {
				return fmt.Errorf("checking Camie tagging worker: %w", statusErr)
			}
			if !status.Installed {
				return fmt.Errorf("Camie v2 model is not installed on the selected worker")
			}
		}
		if request.UseEva02 {
			evaClient, _, _, err = newVisualSimilarityEmbedder()
			if err != nil {
				return fmt.Errorf("starting EVA02 tagging worker: %w", err)
			}
			defer evaClient.Close()
			status, statusErr := evaClient.Status(ctx)
			if statusErr != nil {
				return fmt.Errorf("checking EVA02 tagging worker: %w", statusErr)
			}
			if !status.Installed {
				return fmt.Errorf("EVA02 model is not installed on the selected worker")
			}
		}

		failures := 0
		for _, imageID := range ids {
			if job.IsCancelled(ctx) {
				return nil
			}
			source, sourceErr := loadCamieBulkImageSource(ctx, mgr, imageID)
			if sourceErr != nil {
				failures++
				logger.Warnf("bulk tagging: loading Image %d: %v", imageID, sourceErr)
				progress.Increment()
				continue
			}

			predictions, tagErr := inferBulkImageTags(ctx, source.Path, config, request, camieClient, evaClient)
			if tagErr == nil {
				predictions = filterCamieBulkPredictionsV2(predictions, request.camieBulkRequestV2)
				predictions, tagErr = validateCamiePredictionsV2(predictions)
			}
			if tagErr == nil && len(predictions) > 0 {
				_, tagErr = applyCamieMetadataV2(ctx, imageID, predictions, false)
			}
			if tagErr != nil {
				failures++
				logger.Warnf("bulk tagging: Image %d failed: %v", imageID, tagErr)
			}
			progress.Increment()
		}
		if failures > 0 {
			return fmt.Errorf("bulk tagging failed for %d Image(s); see logs for Image IDs", failures)
		}
		return nil
	}))
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

func inferBulkImageTags(
	ctx context.Context,
	path string,
	config camieConfig,
	request bulkImageTagRequest,
	camieClient camietagger.Tagger,
	evaClient visualembedding.Embedder,
) ([]camietagger.Tag, error) {
	var filenameTags []camietagger.Tag
	var camieTags []camietagger.Tag
	var evaTags []camietagger.Tag
	var err error

	if config.FilenameEnabled {
		filenameTags, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			return nil, fmt.Errorf("parsing local filename metadata: %w", err)
		}
	}
	if request.UseCamie {
		camieTags, err = camieClient.Tag(ctx, path, request.Threshold, request.Limit)
		if err != nil {
			return nil, fmt.Errorf("Camie inference: %w", err)
		}
	}
	if request.UseEva02 {
		tagPredictions, evaErr := evaClient.Tag(ctx, path, request.Eva02Threshold, request.Limit)
		if evaErr != nil {
			return nil, fmt.Errorf("EVA02 inference: %w", evaErr)
		}
		for _, prediction := range tagPredictions {
			converted, ok := eva02TagPredictionToCamie(prediction)
			if ok {
				evaTags = append(evaTags, converted)
			}
		}
	}
	return mergeBulkImageTags(filenameTags, camieTags, evaTags), nil
}

// Source priority is filename → Camie → EVA02. An authoritative filename
// identity blocks conflicting inferred identities in the same category; an
// already-selected higher-priority prediction keeps its score and provenance.
func mergeBulkImageTags(filename, camie, eva []camietagger.Tag) []camietagger.Tag {
	merged := mergeCamiePredictions(camie, filename)
	seen := make(map[string]bool, len(merged))
	authoritative := make(map[string]bool, 3)
	for _, prediction := range filename {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name != "" && camieFilenameAuthoritativeCategory(prediction.Category) {
			authoritative[prediction.Category] = true
		}
	}
	for _, prediction := range merged {
		prediction = normalizeCamiePrediction(prediction)
		seen[prediction.Category+"\x00"+strings.ToLower(prediction.Name)] = true
	}
	for _, prediction := range eva {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name == "" {
			continue
		}
		key := prediction.Category + "\x00" + strings.ToLower(prediction.Name)
		if seen[key] || authoritative[prediction.Category] {
			continue
		}
		seen[key] = true
		merged = append(merged, prediction)
	}
	return merged
}
