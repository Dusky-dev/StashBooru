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
const estimateAutoDefaultGroups = 4
const estimateAutoMaxBytesPerGroup int64 = 96 << 20
const estimateAutoMaxBytesPerFile int64 = 32 << 20

var storageEstimateMutex sync.Mutex
var storageEstimateBusy bool
var storageEstimateJobID int
var storageEstimateResult json.RawMessage

type storageEstimateRequest struct {
	Auto       bool   `json:"auto,omitempty"`
	StackID    int    `json:"stackID,omitempty"`
	StackIDs   []int  `json:"stackIDs,omitempty"`
	SampleSize int    `json:"sampleSize,omitempty"`
	Diverse    bool   `json:"diverse,omitempty"`
	Fidelity   string `json:"fidelity"`
}

type storageEstimateStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type estimateSourceMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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

// The default stays small. Extended sampling remains bounded by the same
// eight-group cap as the read-only P12/P13 batch estimator.
func storageEstimateAutoGroupLimit(req storageEstimateRequest) (int, error) {
	if !req.Auto {
		if req.SampleSize != 0 {
			return 0, fmt.Errorf("sampleSize is only supported for automatic estimates")
		}
		return 0, nil
	}
	switch req.SampleSize {
	case 0, estimateAutoDefaultGroups:
		return estimateAutoDefaultGroups, nil
	case estimateMaxGroups:
		return estimateMaxGroups, nil
	default:
		return 0, fmt.Errorf("automatic sampleSize must be 4 or 8")
	}
}

func validateStorageEstimateRequest(req storageEstimateRequest) error {
	if _, err := storageEstimateSelectedIDs(req); err != nil {
		return err
	}
	if _, err := storageEstimateAutoGroupLimit(req); err != nil {
		return err
	}
	if req.Diverse && !req.Auto {
		return fmt.Errorf("diversity sampling requires automatic mode")
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

// estimatePortableMemberName supplies an ASCII-only export name to the P12/P13
// prototype, whose default is the source basename and excludes Unicode and
// common filename punctuation. This changes only the temporary read-only
// estimator input: source paths, names, bytes and catalogue IDs stay untouched.
func estimatePortableMemberName(imageID int, originalPath string) string {
	extension := filepath.Ext(originalPath)
	if len(extension) < 2 || len(extension) > 12 {
		extension = ".img"
	} else {
		for _, ch := range extension[1:] {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", ch) {
				extension = ".img"
				break
			}
		}
	}
	return "image-" + strconv.Itoa(imageID) + strings.ToLower(extension)
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
			result.Members = append(result.Members, estimateSourceMember{
				ID: item.Media.Key(), Name: estimatePortableMemberName(item.Media.ID, path), Path: path,
			})
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
	Mode               string `json:"mode"`
	TotalImages        int    `json:"totalImages"`
	PHashIndexedImages int    `json:"pHashIndexedImages"`
	RequestedGroups    int    `json:"requestedGroups"`
	SampledGroups      int    `json:"sampledGroups"`
	SampledImages      int    `json:"sampledImages"`
	Diverse            bool   `json:"diverse,omitempty"`
}

// Automatic discovery is a bounded trial, NOT a library-wide savings scan.
// Curated image-only stacks take priority; otherwise use existing primary-file
// pHash clusters as candidate compression groups. pHash does NOT prove that two
// files are duplicates: the Python estimator verifies every reconstructed byte.
// A file can belong to more than one metadata record. Avoid counting it or
// its image identity twice across selected groups, including within one group.
func estimateAutoDisjoint(group estimateSourceGroup, used map[string]bool) bool {
	local := make(map[string]bool, len(group.Members)*2)
	for _, member := range group.Members {
		if member.ID == "" || member.Path == "" {
			return false
		}
		for _, key := range []string{"id:" + member.ID, "path:" + filepath.Clean(member.Path)} {
			if used[key] || local[key] {
				return false
			}
			local[key] = true
		}
	}
	return true
}

func estimateAutoRemember(group estimateSourceGroup, used map[string]bool) {
	for _, member := range group.Members {
		used["id:"+member.ID] = true
		used["path:"+filepath.Clean(member.Path)] = true
	}
}

func estimateAutoSources(ctx context.Context, maxGroups int, diverse bool) ([]estimateSourceGroup, storageEstimateSelection, error) {
	if maxGroups != estimateAutoDefaultGroups && maxGroups != estimateMaxGroups {
		return nil, storageEstimateSelection{}, fmt.Errorf("automatic estimates require 4 or 8 sample groups")
	}
	mgr := manager.GetInstance()
	meta := storageEstimateSelection{Mode: "auto", RequestedGroups: maxGroups}
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
	if diverse {
		meta.Diverse = true
		return estimateDiverseAutoSources(ctx, maxGroups, ids, meta)
	}

	groups := make([]estimateSourceGroup, 0, maxGroups)
	seen := make(map[string]bool, maxGroups*estimateMaxMembers*2)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, meta, err
		}
		group, err := estimateStackSources(ctx, id)
		if err != nil || !estimateAutoWithinBudget(group) || !estimateAutoDisjoint(group, seen) {
			continue // unreadable, changed, oversized, overlapping or unsafe source
		}
		groups = append(groups, group)
		estimateAutoRemember(group, seen)
		if len(groups) == maxGroups {
			break
		}
	}
	if len(groups) < maxGroups {
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
			if len(group.Members) >= 2 && estimateAutoDisjoint(group, seen) {
				groups = append(groups, group)
				estimateAutoRemember(group, seen)
			}
			if len(groups) == maxGroups {
				break
			}
		}
	}
	if len(groups) == 0 {
		return nil, meta, fmt.Errorf("no eligible image groups found: create a Visual Stack of 2–16 images or generate image pHashes in Tasks, then retry")
	}
	meta.SampledGroups = len(groups)
	for _, group := range groups {
		meta.SampledImages += len(group.Members)
	}
	return groups, meta, nil
}

// For an opt-in diagnostic comparison, spread candidate probes across the
// ranked pHash clusters instead of taking only the largest groups. The
// underlying pHash clusters are still biased candidate matches, never a
// statistical sample of all library Images.
func estimateDiverseClusterIndices(clusters [][]int, limit int) []int {
	if limit <= 0 {
		return nil
	}
	eligible := make([]int, 0)
	for index, cluster := range clusters {
		if len(cluster) >= 2 {
			eligible = append(eligible, index)
		}
	}
	if len(eligible) <= limit {
		return eligible
	}
	indices := make([]int, 0, limit)
	if limit == 1 {
		return append(indices, eligible[0])
	}
	for slot := 0; slot < limit; slot++ {
		index := slot * (len(eligible) - 1) / (limit - 1)
		indices = append(indices, eligible[index])
	}
	return indices
}

// These labels describe diversity among eligible *candidate groups*, not
// population-wide format or size distributions. Classification uses bounded
// path extension and file-size metadata, not image decoders or media writes.
func estimateDiversityBuckets(group estimateSourceGroup) (string, string, string) {
	format := ""
	var total int64
	for _, member := range group.Members {
		ext := strings.ToLower(filepath.Ext(member.Path))
		switch ext {
		case ".jpg", ".jpeg":
			ext = "jpeg"
		case ".png", ".webp", ".jxl", ".gif", ".avif":
			ext = strings.TrimPrefix(ext, ".")
		default:
			ext = "other"
		}
		if format == "" {
			format = ext
		} else if format != ext {
			format = "mixed"
		}
		if info, err := os.Lstat(member.Path); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	avg := total / int64(max(1, len(group.Members)))
	size := "large"
	switch {
	case avg < 256<<10:
		size = "small"
	case avg < 2<<20:
		size = "medium"
	}
	count := "many"
	switch {
	case len(group.Members) <= 3:
		count = "few"
	case len(group.Members) <= 8:
		count = "several"
	}
	return format, size, count
}

// Deterministic greedy selection rewards previously unseen source formats,
// byte-size bands and membership sizes; all chosen groups remain disjoint.
func estimateChooseDiverseGroups(candidates []estimateSourceGroup, maxGroups int) []estimateSourceGroup {
	selected := make([]estimateSourceGroup, 0, maxGroups)
	used := make(map[string]bool)
	usedFormats := make(map[string]bool)
	usedSizes := make(map[string]bool)
	usedCounts := make(map[string]bool)
	chosen := make([]bool, len(candidates))
	for len(selected) < maxGroups {
		best, bestScore := -1, -1
		for index, group := range candidates {
			if chosen[index] || !estimateAutoDisjoint(group, used) {
				continue
			}
			format, size, count := estimateDiversityBuckets(group)
			score := 0
			if !usedFormats[format] {
				score += 4
			}
			if !usedSizes[size] {
				score += 2
			}
			if !usedCounts[count] {
				score++
			}
			if score > bestScore {
				best, bestScore = index, score
			}
		}
		if best == -1 {
			break
		}
		group := candidates[best]
		chosen[best] = true
		selected = append(selected, group)
		estimateAutoRemember(group, used)
		format, size, count := estimateDiversityBuckets(group)
		usedFormats[format], usedSizes[size], usedCounts[count] = true, true, true
	}
	return selected
}

// Probe a bounded set of curated and pHash-backed groups, then pick groups
// covering more source-file characteristics. No scan, regrouping or media
// mutation: each chosen source is verified through the same P12/P13 script.
func estimateDiverseAutoSources(ctx context.Context, maxGroups int, ids []int, meta storageEstimateSelection) ([]estimateSourceGroup, storageEstimateSelection, error) {
	const maxStackCandidates = 8
	const maxPHashCandidates = 24
	candidates := make([]estimateSourceGroup, 0, maxStackCandidates+maxPHashCandidates)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, meta, err
		}
		group, err := estimateStackSources(ctx, id)
		if err != nil || !estimateAutoWithinBudget(group) || !estimateAutoDisjoint(group, make(map[string]bool)) {
			continue
		}
		candidates = append(candidates, group)
		if len(candidates) >= maxStackCandidates {
			break
		}
	}
	var clusters [][]int
	err := txn.WithReadTxn(ctx, manager.GetInstance().Repository.TxnManager, func(ctx context.Context) error {
		var err error
		clusters, meta.PHashIndexedImages, err = sqlite.ImagePHashClusters(ctx, 4)
		return err
	})
	if err != nil {
		return nil, meta, err
	}
	for _, index := range estimateDiverseClusterIndices(clusters, maxPHashCandidates) {
		if err := ctx.Err(); err != nil {
			return nil, meta, err
		}
		group, err := estimateAutoPHashGroup(ctx, clusters[index])
		if err != nil {
			return nil, meta, err
		}
		if len(group.Members) >= 2 && estimateAutoWithinBudget(group) && estimateAutoDisjoint(group, make(map[string]bool)) {
			candidates = append(candidates, group)
		}
	}
	groups := estimateChooseDiverseGroups(candidates, maxGroups)
	if len(groups) == 0 {
		return nil, meta, fmt.Errorf("no eligible image groups found for diverse sample: create image-only Visual Stacks or generate image pHashes")
	}
	meta.SampledGroups = len(groups)
	for _, group := range groups {
		meta.SampledImages += len(group.Members)
	}
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
			group.Members = append(group.Members, estimateSourceMember{
				ID: ref.Key(), Name: estimatePortableMemberName(id, primary.Base().Path), Path: primary.Base().Path,
			})
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
			var groups []estimateSourceGroup
			var selection storageEstimateSelection
			if req.Auto {
				limit, err := storageEstimateAutoGroupLimit(req)
				if err != nil {
					return err
				}
				groups, selection, err = estimateAutoSources(ctx, limit, req.Diverse)
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
