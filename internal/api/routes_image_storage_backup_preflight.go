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
	"regexp"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

// Backup preflight is intentionally separate from the packing and estimate
// endpoints. It cannot activate packed media, change paths or delete originals.
var storageBackupPreflightMutex sync.Mutex
var storageBackupPreflightBusy bool
var storageBackupPreflightJobID int
var storageBackupPreflightResult json.RawMessage

var storageBackupSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

type storageBackupPreflightRequest struct {
	StackID  int    `json:"stackID"`
	Snapshot string `json:"snapshot"`
}

type storageBackupPreflightStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type storageBackupPreflightReport struct {
	Experimental             bool              `json:"experimental"`
	ReadOnly                 bool              `json:"readOnly"`
	Verified                 bool              `json:"verified"`
	MediaBackupVerified      bool              `json:"mediaBackupVerified"`
	Snapshot                 string            `json:"snapshot"`
	Group                    string            `json:"group"`
	SelectedImagesCovered    int               `json:"selectedImagesCovered"`
	SnapshotFilesVerified    int               `json:"snapshotFilesVerified"`
	SnapshotObjectsVerified  int               `json:"snapshotObjectsVerified"`
	ImageSHA256              map[string]string `json:"imageSHA256"`
	SourcesRetained          bool              `json:"sourcesRetained"`
	NativePackingEnabled     bool              `json:"nativePackingEnabled"`
	NativeServingVerified    bool              `json:"nativeServingVerified"`
	DatabaseBackupVerified   bool              `json:"databaseBackupVerified"`
	OriginalEvictionEnabled  bool              `json:"originalEvictionEnabled"`
}

func validateStorageBackupPreflightRequest(req storageBackupPreflightRequest) error {
	if req.StackID < 1 {
		return fmt.Errorf("select a positive Visual Stack ID")
	}
	if !mediaBackupSnapshotID.MatchString(req.Snapshot) {
		return fmt.Errorf("select a valid media backup snapshot ID")
	}
	return nil
}

func storageBackupPreflightScriptPath() string {
	if override := strings.TrimSpace(os.Getenv("STASH_IMAGE_BACKUP_PREFLIGHT_SCRIPT")); override != "" {
		return override
	}
	return filepath.FromSlash("scripts/image_storage_backup_preflight.py")
}

func storageBackupPreflightScriptsAvailable() bool {
	for _, path := range []string{storageBackupPreflightScriptPath(), mediaBackupScriptPath()} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func validateStorageBackupPreflightReport(data []byte, group estimateSourceGroup, snapshot string) (json.RawMessage, error) {
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, fmt.Errorf("unexpected backup preflight report length")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("invalid backup preflight JSON: %w", err)
	}
	// Require explicit true/false fields; omitting a safety flag must never
	// silently default to false and appear to authorize future operations.
	for _, key := range []string{"experimental", "readOnly", "verified", "mediaBackupVerified", "sourcesRetained"} {
		if string(fields[key]) != "true" {
			return nil, fmt.Errorf("backup preflight did not explicitly confirm %s", key)
		}
	}
	for _, key := range []string{"nativePackingEnabled", "nativeServingVerified", "databaseBackupVerified", "originalEvictionEnabled"} {
		if string(fields[key]) != "false" {
			return nil, fmt.Errorf("backup preflight must explicitly disable %s", key)
		}
	}
	var report storageBackupPreflightReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("invalid backup preflight response: %w", err)
	}
	if !report.Experimental || !report.ReadOnly || !report.Verified || !report.MediaBackupVerified ||
		!report.SourcesRetained || report.NativePackingEnabled || report.NativeServingVerified ||
		report.DatabaseBackupVerified || report.OriginalEvictionEnabled {
		return nil, fmt.Errorf("backup preflight failed its non-destructive safety contract")
	}
	if report.Group != group.ID || report.Snapshot != snapshot ||
		report.SelectedImagesCovered != len(group.Members) ||
		len(report.ImageSHA256) != len(group.Members) ||
		report.SnapshotFilesVerified < len(group.Members) || report.SnapshotObjectsVerified < 1 {
		return nil, fmt.Errorf("backup preflight returned mismatched selection evidence")
	}
	for _, member := range group.Members {
		hash, found := report.ImageSHA256[member.ID]
		if !found || !storageBackupSHA256.MatchString(hash) {
			return nil, fmt.Errorf("backup preflight did not verify every selected image")
		}
	}
	return append(json.RawMessage(nil), data...), nil
}

func runStorageBackupPreflight(ctx context.Context, group estimateSourceGroup, store, snapshot string) (json.RawMessage, error) {
	// The only media paths passed to the verifier come from estimateStackSources,
	// which queries the native repository. No request body can supply paths.
	input, err := os.CreateTemp("", "stash-backup-preflight-*.json")
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
	command := exec.CommandContext(ctx, mediaBackupPython(), storageBackupPreflightScriptPath(),
		"--stacks", input.Name(), "--group", group.ID, "--store", store, "--snapshot", snapshot)
	output, err := command.CombinedOutput()
	if err != nil {
		if len(output) > 2048 {
			output = output[:2048]
		}
		return nil, fmt.Errorf("media backup preflight failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return validateStorageBackupPreflightReport(output, group, snapshot)
}

func storageBackupPreflightBusyLocked() bool {
	if storageBackupPreflightBusy && storageBackupPreflightJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(storageBackupPreflightJobID)
		if current == nil || current.Status == job.StatusFinished || current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			storageBackupPreflightBusy = false
		}
	}
	return storageBackupPreflightBusy
}

func (rs imageRoutes) StorageBackupPreflightStatus(w http.ResponseWriter, r *http.Request) {
	storageBackupPreflightMutex.Lock()
	busy := storageBackupPreflightBusyLocked()
	jobID := storageBackupPreflightJobID
	result := append(json.RawMessage(nil), storageBackupPreflightResult...)
	storageBackupPreflightMutex.Unlock()
	writeVisualSimilarityJSON(w, storageBackupPreflightStatus{
		Available: storageBackupPreflightScriptsAvailable(),
		Busy: busy,
		JobID: jobID,
		Result: result,
	})
}

func (rs imageRoutes) StorageBackupPreflightStart(w http.ResponseWriter, r *http.Request) {
	var req storageBackupPreflightRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid backup preflight request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected extra backup preflight data", http.StatusBadRequest)
		return
	}
	if err := validateStorageBackupPreflightRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !storageBackupPreflightScriptsAvailable() {
		http.Error(w, "backup preflight or media backup script not installed", http.StatusServiceUnavailable)
		return
	}
	storageBackupPreflightMutex.Lock()
	defer storageBackupPreflightMutex.Unlock()
	if storageBackupPreflightBusyLocked() {
		http.Error(w, "a backup preflight is already queued or running", http.StatusConflict)
		return
	}
	storageBackupPreflightResult = nil
	storageBackupPreflightBusy = true
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Verify Visual Stack media backup coverage", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageBackupPreflightMutex.Lock()
				storageBackupPreflightBusy = false
				storageBackupPreflightMutex.Unlock()
			}()
			progress.SetTotal(3)
			if job.IsCancelled(ctx) {
				return nil
			}
			group, err := estimateStackSources(ctx, req.StackID)
			if err != nil {
				return err
			}
			progress.SetProcessed(1)
			cfg, err := readMediaBackupConfig()
			if err != nil {
				return err
			}
			store, err := validateBackupDestination(cfg.Destination, configuredBackupRoots())
			if err != nil {
				return err
			}
			progress.SetProcessed(2)
			if job.IsCancelled(ctx) {
				return nil
			}
			evidence, err := runStorageBackupPreflight(ctx, group, store, req.Snapshot)
			if err != nil {
				return err
			}
			if job.IsCancelled(ctx) {
				return nil
			}
			storageBackupPreflightMutex.Lock()
			storageBackupPreflightResult = evidence
			storageBackupPreflightMutex.Unlock()
			progress.SetProcessed(3)
			return nil
		},
	))
	storageBackupPreflightJobID = jobID
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
