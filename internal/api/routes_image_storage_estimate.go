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
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/txn"
)

const estimateMaxMembers = 16
const estimateMaxGroups = 8
const estimateAutoMaxGroups = 4
const estimateAutoMaxBytesPerGroup int64 = 96 << 20
const estimateAutoMaxBytesPerFile int64 = 32 << 20

var storageEstimateMutex sync.Mutex
var storageEstimateBusy bool
var storageEstimateJobID int
var storageEstimateResult json.RawMessage

type storageEstimateRequest struct {
	Auto     bool   `json:"auto,omitempty"`
	StackID  int    `json:"stackID,omitempty"`
	StackIDs []int  `json:"stackIDs,omitempty"`
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

func storageEstimateSelectedIDs(req storageEstimateRequest) ([]int, error) {
	if req.Auto {
		if req.StackID != 0 || len(req.StackIDs) != 0 {
			return nil, fmt.Errorf("automatic estimate cannot include manual Visual Stack IDs")
		}
		return nil, nil
	}
	if len(req.StackIDs) != 0 && req.StackID != 0 {
		return nil, fmt.Errorf("provide stackID or stackIDs, not both")
	}
	ids := req.StackIDs
	if len(ids) == 0 {
		ids = []int{req.StackID}
	}
	if len(ids) > estimateMaxGroups {
		return nil, fmt.Errorf("select no more than %d Visual Stacks", estimateMaxGroups)
	}
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id < 1 {
			return nil, fmt.Errorf("all selected Visual Stack IDs must be positive")
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate Visual Stack ID: %d", id)
		}
		seen[id] = true
	}
	return ids, nil
}

func validateStorageEstimateRequest(req storageEstimateRequest) error {
	if _, err := storageEstimateSelectedIDs(req); err != nil {
		return err
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

func estimatePython(ctx context.Context, groups []estimateSourceGroup, fidelity string) (json.RawMessage, error) {
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
	if err := json.NewEncoder(input).Encode(map[string]any{"stacks": groups}); err != nil {
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


type storageEstimateSelection struct {
	Mode              string `json:"mode"`
	TotalImages       int    `json:"totalImages"`
	PHashIndexedImages int    `json:"pHashIndexedImages"`
	SampledGroups     int    `json:"sampledGroups"`
}

// Automatic discovery is a bounded trial, NOT a library-wide savings scan.
// Curated image-only stacks take priority; otherwise use existing primary-file
// pHash clusters as candidate compression groups. pHash does NOT prove that two
// files are duplicates: the Python estimator verifies every reconstructed byte.
func estimateAutoSources(ctx context.Context) ([]estimateSourceGroup, storageEstimateSelection, error) {
	mgr := manager.GetInstance()
	meta := storageEstimateSelection{Mode: "auto"}
	var ids []int
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		var err error
		ids, err = sqlite.StorageEstimateVisualStackIDs(ctx, 64)
		if err != nil {
			return err
		}
		meta.TotalImages, err = mgr.Repository.Image.Count(ctx)
		return err
	})
	if err != nil {
		return nil, meta, err
	}

	groups := make([]estimateSourceGroup, 0, estimateAutoMaxGroups)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, meta, err
		}
		group, err := estimateStackSources(ctx, id)
		if err != nil || !estimateAutoWithinBudget(group) {
			continue // unreadable, changed, oversized or unsafe source
		}
		groups = append(groups, group)
		if len(groups) == estimateAutoMaxGroups {
			break
		}
	}
	if len(groups) < estimateAutoMaxGroups {
		var clusters [][]int
		err = txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
			var err error
			clusters, meta.PHashIndexedImages, err = sqlite.ImagePHashClusters(ctx, 4)
			return err
		})
		if err != nil {
			return nil, meta, err
		}
		for _, cluster := range clusters {
			if err := ctx.Err(); err != nil {
				return nil, meta, err
			}
			if len(cluster) < 2 {
				continue
			}
			group, err := estimateAutoPHashGroup(ctx, cluster)
			if err != nil {
				return nil, meta, err
			}
			if len(group.Members) >= 2 {
				groups = append(groups, group)
			}
			if len(groups) == estimateAutoMaxGroups {
				break
			}
		}
	}
	if len(groups) == 0 {
		return nil, meta, fmt.Errorf("no eligible image groups found: create a Visual Stack of 2–16 images or generate image pHashes in Tasks, then retry")
	}
	meta.SampledGroups = len(groups)
	return groups, meta, nil
}

func estimateAutoWithinBudget(group estimateSourceGroup) bool {
	if len(group.Members) < 2 || len(group.Members) > estimateMaxMembers {
		return false
	}
	var total int64
	for _, member := range group.Members {
		info, err := os.Lstat(member.Path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > estimateAutoMaxBytesPerFile {
			return false
		}
		total += info.Size()
		if total > estimateAutoMaxBytesPerGroup {
			return false
		}
	}
	return true
}

func estimateAutoPHashGroup(ctx context.Context, ids []int) (estimateSourceGroup, error) {
	mgr := manager.GetInstance()
	group := estimateSourceGroup{Members: make([]estimateSourceMember, 0, estimateMaxMembers)}
	var total int64
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		// Bound file and metadata probes even when a pHash cluster is huge.
		for index, id := range ids {
			if index >= 64 || len(group.Members) == estimateMaxMembers {
				break
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			ref := models.MediaReference{Kind: models.MediaKindImage, ID: id}
			stack, err := mgr.Repository.VisualStack.FindByMedia(ctx, ref)
			if err != nil {
				return err
			}
			if stack != nil {
				continue // do not relabel existing curated membership
			}
			image, err := mgr.Repository.Image.Find(ctx, id)
			if err != nil {
				return err
			}
			if image == nil || image.LoadPrimaryFile(ctx, mgr.Repository.File) != nil {
				continue
			}
			primary := image.Files.Primary()
			if primary == nil || estimateRegularSource(primary.Base().Path) != nil {
				continue
			}
			info, err := os.Lstat(primary.Base().Path)
			if err != nil || info.Size() <= 0 || info.Size() > estimateAutoMaxBytesPerFile || total+info.Size() > estimateAutoMaxBytesPerGroup {
				continue
			}
			total += info.Size()
			group.Members = append(group.Members, estimateSourceMember{ID: ref.Key(), Path: primary.Base().Path})
		}
		return nil
	})
	if err != nil {
		return group, err
	}
	if len(group.Members) >= 2 {
		group.ID = "sample:phash:" + strings.TrimPrefix(group.Members[0].ID, "image:")
		group.Representative = group.Members[0].ID
	}
	return group, nil
}

func estimateAnnotateAutoReport(raw json.RawMessage, meta storageEstimateSelection) (json.RawMessage, error) {
	var report map[string]json.RawMessage
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, err
	}
	selection, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	report["selection"] = selection
	return json.Marshal(report)
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
		Busy:      busy,
		JobID:     identifier,
		Result:    result,
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
			groups := []estimateSourceGroup{}
			var selection storageEstimateSelection
			if req.Auto {
				var err error
				groups, selection, err = estimateAutoSources(ctx)
				if err != nil {
					return err
				}
			} else {
				ids, err := storageEstimateSelectedIDs(req)
				if err != nil {
					return err
				}
				groups = make([]estimateSourceGroup, 0, len(ids))
				for _, id := range ids {
					if job.IsCancelled(ctx) {
						return nil
					}
					group, err := estimateStackSources(ctx, id)
					if err != nil {
						return fmt.Errorf("stack %d: %w", id, err)
					}
					groups = append(groups, group)
				}
			}
			if job.IsCancelled(ctx) {
				return nil
			}
			progress.SetTotal(len(groups) + 1)
			report, err := estimatePython(ctx, groups, req.Fidelity)
			if err != nil {
				return err
			}
			if req.Auto {
				report, err = estimateAnnotateAutoReport(report, selection)
				if err != nil {
					return err
				}
			}
			storageEstimateMutex.Lock()
			storageEstimateResult = report
			storageEstimateMutex.Unlock()
			progress.SetProcessed(len(groups) + 1)
			return nil
		},
	))
	storageEstimateJobID = jobID
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
