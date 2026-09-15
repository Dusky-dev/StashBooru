package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/txn"
)

const (
	sceneTaggingBatchStatusQueued   = "queued"
	sceneTaggingBatchStatusRunning  = "running"
	sceneTaggingBatchStatusComplete = "complete"
	sceneTaggingBatchStatusFailed   = "failed"
)

type sceneTaggingBatchRequest struct {
	Action        string `json:"action"`
	JobID         int    `json:"jobID"`
	SceneIDs      []int  `json:"sceneIDs"`
	All           bool   `json:"all"`
	IncludeBooru  bool   `json:"includeBooru"`
	AutoApply     bool   `json:"autoApply"`
	ReplaceArtist bool   `json:"replaceArtist"`
}

type sceneTaggingBatchSceneResult struct {
	SceneID      int                        `json:"sceneID"`
	Label        string                     `json:"label"`
	AutoApply    []camietagger.Tag          `json:"autoApply"`
	NeedsReview  []sceneTaggingReviewItem   `json:"needsReview"`
	Applied      *sceneTaggingApplyResponse `json:"applied,omitempty"`
	Error        string                     `json:"error,omitempty"`
	BooruMatched bool                       `json:"booruMatched"`
}

type sceneTaggingBatchStatusResponse struct {
	JobID     int                            `json:"jobID"`
	Status    string                         `json:"status"`
	Total     int                            `json:"total"`
	Processed int                            `json:"processed"`
	Items     []sceneTaggingBatchSceneResult `json:"items"`
	Error     string                         `json:"error,omitempty"`
}

type sceneTaggingBatchState struct {
	mu       sync.RWMutex
	response sceneTaggingBatchStatusResponse
}

var sceneTaggingBatchStates sync.Map

func (s *sceneTaggingBatchState) snapshot() sceneTaggingBatchStatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := s.response
	ret.Items = append([]sceneTaggingBatchSceneResult(nil), s.response.Items...)
	return ret
}

func (s *sceneTaggingBatchState) setJobID(jobID int) {
	s.mu.Lock()
	s.response.JobID = jobID
	s.mu.Unlock()
}

func (s *sceneTaggingBatchState) start(total int) {
	s.mu.Lock()
	s.response.Status = sceneTaggingBatchStatusRunning
	s.response.Total = total
	s.mu.Unlock()
}

func (s *sceneTaggingBatchState) appendResult(result sceneTaggingBatchSceneResult) {
	s.mu.Lock()
	s.response.Items = append(s.response.Items, result)
	s.response.Processed = len(s.response.Items)
	s.mu.Unlock()
}

func (s *sceneTaggingBatchState) complete() {
	s.mu.Lock()
	s.response.Status = sceneTaggingBatchStatusComplete
	s.mu.Unlock()
}

func (s *sceneTaggingBatchState) fail(err error) {
	s.mu.Lock()
	s.response.Status = sceneTaggingBatchStatusFailed
	s.response.Error = err.Error()
	s.mu.Unlock()
}

func handleSceneTaggingBatch(w http.ResponseWriter, r *http.Request) {
	var request sceneTaggingBatchRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding batch Video Tagging request: %v", err), http.StatusBadRequest)
		return
	}

	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action == "" {
		action = "start"
	}

	switch action {
	case "start":
		startSceneTaggingBatch(w, r, request)
	case "status":
		writeSceneTaggingBatchStatus(w, request.JobID)
	case "forget":
		if request.JobID <= 0 {
			http.Error(w, "batch Video Tagging jobID must be positive", http.StatusBadRequest)
			return
		}
		sceneTaggingBatchStates.Delete(request.JobID)
		writeVisualSimilarityJSON(w, map[string]bool{"forgotten": true})
	default:
		http.Error(w, fmt.Sprintf("unknown batch Video Tagging action %q", request.Action), http.StatusBadRequest)
	}
}

func startSceneTaggingBatch(w http.ResponseWriter, r *http.Request, request sceneTaggingBatchRequest) {
	if request.All && len(request.SceneIDs) > 0 {
		http.Error(w, "choose either explicit video IDs or all videos, not both", http.StatusBadRequest)
		return
	}
	if !request.All && len(request.SceneIDs) == 0 {
		http.Error(w, "select at least one video or choose all videos", http.StatusBadRequest)
		return
	}

	request.SceneIDs = append([]int(nil), request.SceneIDs...)
	state := &sceneTaggingBatchState{response: sceneTaggingBatchStatusResponse{
		Status: sceneTaggingBatchStatusQueued,
		Items:  []sceneTaggingBatchSceneResult{},
	}}

	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Batch Video Tagging...", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			ids, err := resolveSceneTaggingBatchIDs(ctx, request)
			if err != nil {
				state.fail(err)
				return err
			}
			state.start(len(ids))
			progress.SetTotal(len(ids))
			if len(ids) == 0 {
				state.complete()
				return nil
			}

			config, err := loadCamieConfig()
			if err != nil {
				state.fail(err)
				return err
			}

			for _, sceneID := range ids {
				if job.IsCancelled(ctx) {
					state.fail(context.Canceled)
					return nil
				}

				result := processSceneTaggingBatchItem(ctx, sceneID, config, request)
				state.appendResult(result)
				progress.Increment()
			}

			state.complete()
			return nil
		},
	))
	state.setJobID(jobID)
	sceneTaggingBatchStates.Store(jobID, state)
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

func writeSceneTaggingBatchStatus(w http.ResponseWriter, jobID int) {
	if jobID <= 0 {
		http.Error(w, "batch Video Tagging jobID must be positive", http.StatusBadRequest)
		return
	}
	value, ok := sceneTaggingBatchStates.Load(jobID)
	if !ok {
		http.Error(w, "batch Video Tagging job not found", http.StatusNotFound)
		return
	}
	state, ok := value.(*sceneTaggingBatchState)
	if !ok {
		http.Error(w, "invalid batch Video Tagging job state", http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, state.snapshot())
}

func resolveSceneTaggingBatchIDs(ctx context.Context, request sceneTaggingBatchRequest) ([]int, error) {
	if !request.All {
		return normalizeSceneTaggingBatchIDs(request.SceneIDs)
	}

	mgr := manager.GetInstance()
	var ids []int
	if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		scenes, err := mgr.Repository.Scene.All(ctx)
		if err != nil {
			return err
		}
		ids = make([]int, 0, len(scenes))
		for _, scene := range scenes {
			if scene != nil {
				ids = append(ids, scene.ID)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("loading videos for batch Video Tagging: %w", err)
	}
	sort.Ints(ids)
	return ids, nil
}

func normalizeSceneTaggingBatchIDs(raw []int) ([]int, error) {
	seen := make(map[int]struct{}, len(raw))
	ids := make([]int, 0, len(raw))
	for _, id := range raw {
		if id <= 0 {
			return nil, fmt.Errorf("invalid video id %d", id)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids, nil
}

func processSceneTaggingBatchItem(ctx context.Context, sceneID int, config camieConfig, request sceneTaggingBatchRequest) sceneTaggingBatchSceneResult {
	result := sceneTaggingBatchSceneResult{
		SceneID:     sceneID,
		AutoApply:   []camietagger.Tag{},
		NeedsReview: []sceneTaggingReviewItem{},
	}

	path, err := loadSceneTaggingBatchPath(ctx, sceneID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Label = filepath.Base(path)

	local := []camietagger.Tag{}
	if config.FilenameEnabled {
		local, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			result.Error = fmt.Sprintf("parsing local filename metadata: %v", err)
			return result
		}
		local = enrichNativeCamiePredictionTargets(ctx, local)
	}

	booru := []camietagger.Tag{}
	if request.IncludeBooru {
		booru, result.BooruMatched, err = loadSceneTaggingBatchBooru(ctx, path)
		if err != nil {
			result.Error = err.Error()
			return result
		}
	}

	plan := planSceneTaggingReview(local, booru)
	result.AutoApply = plan.AutoApply
	result.NeedsReview = plan.NeedsReview

	if request.AutoApply && len(plan.AutoApply) > 0 {
		applied, applyErr := applySceneTaggingMetadata(ctx, sceneID, plan.AutoApply, request.ReplaceArtist)
		if applyErr != nil {
			result.Error = fmt.Sprintf("auto-applying metadata: %v", applyErr)
			return result
		}
		result.Applied = &applied
	}
	return result
}

func loadSceneTaggingBatchPath(ctx context.Context, sceneID int) (string, error) {
	mgr := manager.GetInstance()
	var path string
	if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		scene, err := mgr.Repository.Scene.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if scene == nil {
			return fmt.Errorf("video %d not found", sceneID)
		}
		if err := scene.LoadPrimaryFile(ctx, mgr.Repository.File); err != nil {
			return err
		}
		path, err = sceneTaggingPrimaryPath(scene)
		return err
	}); err != nil {
		return "", fmt.Errorf("loading video %d: %w", sceneID, err)
	}
	return path, nil
}

func loadSceneTaggingBatchBooru(ctx context.Context, path string) ([]camietagger.Tag, bool, error) {
	provider, post, _, _, err := lookupImageBooruMetadata(ctx, path, lookupBooruPost)
	if errors.Is(err, errBooruNoMatch) {
		return []camietagger.Tag{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("looking up booru metadata: %w", err)
	}

	categoryCtx, cancel := context.WithTimeout(ctx, booruRequestTimeout)
	categories := resolveBooruTagCategories(categoryCtx, provider, post)
	cancel()

	predictions := booruTags(post, provider.name, categories)
	for index := range predictions {
		predictions[index] = normalizeCamiePrediction(predictions[index])
	}
	predictions = enrichNativeCamiePredictionTargets(ctx, predictions)
	return predictions, true, nil
}
