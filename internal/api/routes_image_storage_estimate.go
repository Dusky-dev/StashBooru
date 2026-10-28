package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

const estimateMaxMembers = 16

var storageEstimateMutex sync.Mutex
var storageEstimateBusy bool
var storageEstimateJobID int
var storageEstimateResult json.RawMessage

type storageEstimateRequest struct {
	StackID  int    `json:"stackID"`
	Fidelity string `json:"fidelity"`
}

type storageEstimateStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type estimateSourceMember struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type estimateSourceGroup struct {
	ID             string                 `json:"id"`
	Representative string                 `json:"representative"`
	Members        []estimateSourceMember `json:"members"`
}

func validateStorageEstimateRequest(req storageEstimateRequest) error {
	if req.StackID < 1 {
		return fmt.Errorf("select a positive Visual Stack ID")
	}
	if req.Fidelity != "byte-exact" && req.Fidelity != "pixel-exact" {
		return fmt.Errorf("unsupported storage fidelity")
	}
	return nil
}

func storageEstimateScriptPath() string {
	if value := strings.TrimSpace(os.Getenv("STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT")); value != "" {
		return value
	}
	return filepath.FromSlash("scripts/image_storage_estimate.py")
}

// Keep access confined to files in an actual configured Visual Stack, not
// arbitrary paths received from the browser. Reject symlink and device nodes.
func estimateRegularSource(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("source image path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source image must be a regular file, not a symlink or device")
	}
	return nil
}

func estimateStackSources(ctx context.Context, id int) (estimateSourceGroup, error) {
	mgr := manager.GetInstance()
	var result estimateSourceGroup
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		stack, err := mgr.Repository.VisualStack.Find(ctx, id)
		if err != nil {
			return err
		}
		if stack == nil {
			return fmt.Errorf("visual stack %d was not found", id)
		}
		if len(stack.Members) < 2 || len(stack.Members) > estimateMaxMembers {
			return fmt.Errorf("visual stack must contain 2–16 images for this bounded prototype")
		}
		result = estimateSourceGroup{
			ID:             "stack:" + strconv.Itoa(stack.ID),
			Representative: stack.Representative,
			Members:        make([]estimateSourceMember, 0, len(stack.Members)),
		}
		for _, item := range stack.Members {
			if item.Media.Kind != models.MediaKindImage {
				return fmt.Errorf("storage estimation supports image-only Visual Stacks")
			}
			image, err := mgr.Repository.Image.Find(ctx, item.Media.ID)
			if err != nil {
				return err
			}
			if image == nil {
				return fmt.Errorf("visual stack contains a missing image")
			}
			if err := image.LoadPrimaryFile(ctx, mgr.Repository.File); err != nil {
				return err
			}
			file := image.Files.Primary()
			if file == nil {
				return fmt.Errorf("visual stack image has no primary file")
			}
			path := file.Base().Path
			if err := estimateRegularSource(path); err != nil {
				return fmt.Errorf("invalid image source %d: %w", item.Media.ID, err)
			}
			result.Members = append(result.Members, estimateSourceMember{ID: item.Media.Key(), Path: path})
		}
		return nil
	})
	return result, err
}

func estimatePython(ctx context.Context, group estimateSourceGroup, fidelity string) (json.RawMessage, error) {
	// Temporary specifications contain private local media paths. They are
	// mode 0600 and deleted immediately after the estimator exits.
	input, err := os.CreateTemp("", "stash-storage-estimate-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(input.Name())
	if err := input.Chmod(0600); err != nil {
		input.Close()
		return nil, err
	}
	if err := json.NewEncoder(input).Encode(map[string]any{"stacks": []estimateSourceGroup{group}}); err != nil {
		input.Close()
		return nil, err
	}
	if err := input.Close(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, mediaBackupPython(), storageEstimateScriptPath(),
		"--stacks", input.Name(), "--fidelity", fidelity,
		"--min-bytes", "4096", "--min-percent", "5")
	data, err := cmd.CombinedOutput()
	if err != nil {
		if len(data) > 4096 {
			data = data[:4096]
		}
		return nil, fmt.Errorf("storage estimate failed: %w: %s", err, strings.TrimSpace(string(data)))
	}
	var report struct {
		ReadOnly        bool `json:"readOnly"`
		SourcesRetained bool `json:"sourcesRetained"`
		GroupsFailed    int  `json:"groupsFailed"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid estimate response: %w", err)
	}
	if !report.ReadOnly || !report.SourcesRetained || report.GroupsFailed != 0 {
		return nil, fmt.Errorf("storage estimate did not complete as a read-only verified comparison")
	}
	return append(json.RawMessage(nil), data...), nil
}

func estimateBusyLocked() bool {
	if storageEstimateBusy && storageEstimateJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(storageEstimateJobID)
		if current == nil || current.Status == job.StatusFinished || current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			storageEstimateBusy = false
		}
	}
	return storageEstimateBusy
}

func (rs imageRoutes) StorageEstimateStatus(w http.ResponseWriter, r *http.Request) {
	storageEstimateMutex.Lock()
	busy := estimateBusyLocked()
	identifier := storageEstimateJobID
	result := append(json.RawMessage(nil), storageEstimateResult...)
	storageEstimateMutex.Unlock()
	path := storageEstimateScriptPath()
	info, err := os.Stat(path)
	writeVisualSimilarityJSON(w, storageEstimateStatus{
		Available: err == nil && info.Mode().IsRegular(),
		Busy: busy, JobID: identifier, Result: result,
	})
}

func (rs imageRoutes) StorageEstimateStart(w http.ResponseWriter, r *http.Request) {
	var req storageEstimateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "invalid estimate request", http.StatusBadRequest)
		return
	}
	if err := validateStorageEstimateRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := storageEstimateScriptPath()
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "storage estimator script not installed", http.StatusServiceUnavailable)
		return
	}
	storageEstimateMutex.Lock()
	defer storageEstimateMutex.Unlock()
	if estimateBusyLocked() {
		http.Error(w, "a storage estimate is already queued or running", http.StatusConflict)
		return
	}
	storageEstimateResult = nil
	storageEstimateBusy = true
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Estimate Visual Stack storage savings", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageEstimateMutex.Lock()
				storageEstimateBusy = false
				storageEstimateMutex.Unlock()
			}()
			progress.SetTotal(1)
			group, err := estimateStackSources(ctx, req.StackID)
			if err != nil {
				return err
			}
			report, err := estimatePython(ctx, group, req.Fidelity)
			if err != nil {
				return err
			}
			storageEstimateMutex.Lock()
			storageEstimateResult = report
			storageEstimateMutex.Unlock()
			progress.SetProcessed(1)
			return nil
		},
	))
	storageEstimateJobID = jobID
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
