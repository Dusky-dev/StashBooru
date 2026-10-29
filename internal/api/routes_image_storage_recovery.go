package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

type storageRecoveryRequest struct {
	StackID      int  `json:"stackID"`
	Acknowledged bool `json:"acknowledged"`
}

type storageRecoveryStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type storageRecoveryProof struct {
	Verified          bool              `json:"verified"`
	ByteExact         bool              `json:"byteExact"`
	Complete          bool              `json:"complete"`
	SourcesRetained   bool              `json:"sourcesRetained"`
	OriginalEviction  bool              `json:"originalEvictionEnabled"`
	NativeServing     bool              `json:"nativeServingVerified"`
	ArchiveActivated  bool              `json:"archiveActivated"`
	DatabaseModified  bool              `json:"databaseModified"`
	Method            string            `json:"method"`
	Group             string            `json:"group"`
	Snapshot          string            `json:"mediaBackupSnapshot"`
	RestoredFiles     int               `json:"restoredFiles"`
	RestoredSHA256    map[string]string `json:"restoredSHA256"`
	OutputPath        string            `json:"outputPath"`
}

var storageRecoveryMutex sync.Mutex
var storageRecoveryBusy bool
var storageRecoveryJobID int
var storageRecoveryResult json.RawMessage

func storageRecoveryScriptPath() string {
	if override := strings.TrimSpace(os.Getenv("STASH_IMAGE_CANDIDATE_RECONSTRUCT_SCRIPT")); override != "" {
		return override
	}
	return filepath.FromSlash("scripts/image_storage_candidate_reconstruct.py")
}

func storageRecoveryAvailable() bool {
	if !storageCandidateScriptsAvailable() {
		return false
	}
	info, err := os.Stat(storageRecoveryScriptPath())
	return err == nil && info.Mode().IsRegular()
}

func validateStorageRecoveryRequest(request storageRecoveryRequest) error {
	if request.StackID < 1 || !request.Acknowledged {
		return fmt.Errorf("choose a positive stack ID and acknowledge isolated reconstruction")
	}
	return nil
}

// Never restore to live sources: only use the private, validated candidate root
// from native configuration, with a fresh random output name.
func validateStorageRecoveryArchive(path, directory string) error {
	if !filepath.IsAbs(path) || filepath.Dir(path) != directory || filepath.Ext(path) != ".pack" {
		return fmt.Errorf("candidate archive is outside the private candidate directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("candidate archive must be an existing non-symlink regular file")
	}
	return nil
}

func validateStorageRecoveryProof(data []byte, candidate storageCandidateProof, group estimateSourceGroup, output string) (json.RawMessage, error) {
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, fmt.Errorf("invalid reconstruction report length")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid reconstruction JSON")
	}
	for _, key := range []string{"verified", "byteExact", "complete", "sourcesRetained"} {
		if string(fields[key]) != "true" {
			return nil, fmt.Errorf("reconstruction did not establish %s", key)
		}
	}
	for _, key := range []string{"originalEvictionEnabled", "nativeServingVerified", "archiveActivated", "databaseModified"} {
		if string(fields[key]) != "false" {
			return nil, fmt.Errorf("reconstruction unexpectedly enabled %s", key)
		}
	}
	var proof storageRecoveryProof
	if err := json.Unmarshal(data, &proof); err != nil {
		return nil, err
	}
	if proof.Group != group.ID || proof.Method != candidate.Method ||
		proof.Snapshot != candidate.MediaBackupSnapshot ||
		proof.OutputPath != output || proof.RestoredFiles != len(group.Members) ||
		len(proof.RestoredSHA256) != len(candidate.SourceSHA256) {
		return nil, fmt.Errorf("reconstruction did not match selected candidate")
	}
	for _, member := range group.Members {
		hash, ok := proof.RestoredSHA256[member.ID]
		if !ok || hash != candidate.SourceSHA256[member.ID] || !storageBackupSHA256.MatchString(hash) {
			return nil, fmt.Errorf("reconstructed file digest differs from the original")
		}
	}
	receipt, err := os.Lstat(filepath.Join(output, "extraction.json"))
	if err != nil || !receipt.Mode().IsRegular() {
		return nil, fmt.Errorf("missing reconstruction completion receipt")
	}
	return append(json.RawMessage(nil), data...), nil
}

func runStorageRecovery(ctx context.Context, evidence json.RawMessage, candidate storageCandidateProof, group estimateSourceGroup, output string) (json.RawMessage, error) {
	input, err := os.CreateTemp("", "stash-reconstruct-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(input.Name())
	if err := input.Chmod(0600); err != nil {
		input.Close()
		return nil, err
	}
	if _, err := input.Write(evidence); err != nil {
		input.Close()
		return nil, err
	}
	if err := input.Close(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, mediaBackupPython(), storageRecoveryScriptPath(),
		"--evidence", input.Name(), "--output", output)
	result, err := cmd.CombinedOutput()
	if err != nil {
		if len(result) > 2048 {
			result = result[:2048]
		}
		return nil, fmt.Errorf("candidate reconstruction failed: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return validateStorageRecoveryProof(result, candidate, group, output)
}

func storageRecoveryBusyLocked() bool {
	if storageRecoveryBusy && storageRecoveryJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(storageRecoveryJobID)
		if current == nil || current.Status == job.StatusFinished ||
			current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			storageRecoveryBusy = false
		}
	}
	return storageRecoveryBusy
}

func (rs imageRoutes) StorageRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	storageRecoveryMutex.Lock()
	busy := storageRecoveryBusyLocked()
	jobID := storageRecoveryJobID
	result := append(json.RawMessage(nil), storageRecoveryResult...)
	storageRecoveryMutex.Unlock()
	writeVisualSimilarityJSON(w, storageRecoveryStatus{
		Available: storageRecoveryAvailable(),
		Busy:      busy,
		JobID:     jobID,
		Result:    result,
	})
}

func (rs imageRoutes) StorageRecoveryStart(w http.ResponseWriter, r *http.Request) {
	var request storageRecoveryRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid reconstruction request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected reconstruction request fields", http.StatusBadRequest)
		return
	}
	if err := validateStorageRecoveryRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !storageRecoveryAvailable() {
		http.Error(w, "candidate reconstruction scripts unavailable", http.StatusServiceUnavailable)
		return
	}
	storageRecoveryMutex.Lock()
	defer storageRecoveryMutex.Unlock()
	if storageRecoveryBusyLocked() {
		http.Error(w, "a candidate reconstruction is already queued or running", http.StatusConflict)
		return
	}
	storageCandidateMutex.Lock()
	candidateBytes := append(json.RawMessage(nil), storageCandidateResult...)
	storageCandidateMutex.Unlock()
	var candidate storageCandidateProof
	var receipt struct {
		storageCandidateProof
		ArchivePath string `json:"archivePath"`
	}
	if len(candidateBytes) == 0 || json.Unmarshal(candidateBytes, &receipt) != nil {
		http.Error(w, "no verified candidate in this server session", http.StatusConflict)
		return
	}
	candidate = receipt.storageCandidateProof
	if receipt.Group != fmt.Sprintf("stack:%d", request.StackID) {
		http.Error(w, "candidate is not for the selected Visual Stack", http.StatusConflict)
		return
	}
	storageRecoveryResult = nil
	storageRecoveryBusy = true
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Reconstruct verified image candidate into isolated folder", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageRecoveryMutex.Lock()
				storageRecoveryBusy = false
				storageRecoveryMutex.Unlock()
			}()
			progress.SetTotal(3)
			if job.IsCancelled(ctx) {
				return nil
			}
			group, err := estimateStackSources(ctx, request.StackID)
			if err != nil {
				return err
			}
			cfg, err := readMediaBackupConfig()
			if err != nil {
				return err
			}
			roots := configuredBackupRoots()
			store, err := validateBackupDestination(cfg.Destination, roots)
			if err != nil {
				return err
			}
			directory, err := storageCandidateDirectory(mgr.Config.GetConfigPath(), store, roots)
			if err != nil {
				return err
			}
			if err := validateStorageRecoveryArchive(receipt.ArchivePath, directory); err != nil {
				return err
			}
			if _, err := validateStorageCandidateProof(candidateBytes, group,
				receipt.MediaBackupSnapshot, receipt.Method, receipt.ArchivePath); err != nil {
				return err
			}
			progress.SetProcessed(1)
			if job.IsCancelled(ctx) {
				return nil
			}
			output, err := nextStorageCandidateOutput(directory, request.StackID, "restored")
			if err != nil {
				return err
			}
			output = strings.TrimSuffix(output, ".pack")
			result, err := runStorageRecovery(ctx, candidateBytes, candidate, group, output)
			if err != nil {
				return err
			}
			progress.SetProcessed(2)
			if job.IsCancelled(ctx) {
				return nil
			}
			storageRecoveryMutex.Lock()
			storageRecoveryResult = result
			storageRecoveryMutex.Unlock()
			progress.SetProcessed(3)
			return nil
		},
	))
	storageRecoveryJobID = jobID
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
