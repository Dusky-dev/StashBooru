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
	"strconv"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

var missingOriginalMutex sync.Mutex
var missingOriginalBusy bool
var missingOriginalJobID int
var missingOriginalResult json.RawMessage

type missingOriginalRequest struct {
	CandidateID  string `json:"candidateID"`
	Acknowledged bool   `json:"acknowledged"`
}

type missingOriginalStatus struct {
	Available    bool                          `json:"available"`
	Busy         bool                          `json:"busy"`
	JobID        int                           `json:"jobID,omitempty"`
	Result       json.RawMessage               `json:"result,omitempty"`
	Journal      *missingOriginalJournalStatus `json:"journal,omitempty"`
	JournalError string                        `json:"journalError,omitempty"`
}

type missingOriginalReport struct {
	Verified                          bool              `json:"verified"`
	ReadOnlyAgainstLibrary            bool              `json:"readOnlyAgainstLibrary"`
	OriginalSourcesNotRead            bool              `json:"originalSourcesNotRead"`
	RequiresOriginalSources           bool              `json:"requiresOriginalSources"`
	ArchiveSHA256Verified             bool              `json:"archiveSHA256Verified"`
	MediaBackupObjectsVerified        bool              `json:"mediaBackupObjectsVerified"`
	BackupContentDigestsCovered       bool              `json:"backupContentDigestsCovered"`
	SourceToBackupPathMappingVerified bool              `json:"sourceToBackupPathMappingVerified"`
	DatabaseRecoveryVerified          bool              `json:"databaseRecoveryVerified"`
	NativeMediaServingVerified        bool              `json:"nativeMediaServingVerified"`
	CrashSafeLiveRollbackVerified     bool              `json:"crashSafeLiveRollbackVerified"`
	CanActivate                       bool              `json:"canActivate"`
	OriginalEvictionEnabled           bool              `json:"originalEvictionEnabled"`
	CandidateID                       string            `json:"candidateID"`
	Snapshot                          string            `json:"snapshot"`
	OutputPath                        string            `json:"outputPath"`
	RestoredFiles                     int               `json:"restoredFiles"`
	RestoredSHA256                    map[string]string `json:"restoredSHA256"`
}

func missingOriginalScriptPath() string {
	if value := strings.TrimSpace(os.Getenv("STASH_IMAGE_MISSING_ORIGINAL_SCRIPT")); value != "" {
		return value
	}
	return filepath.FromSlash("scripts/image_storage_missing_original_rehearsal.py")
}

func missingOriginalAvailable() bool {
	if !storageRecoveryAvailable() {
		return false
	}
	info, err := os.Lstat(missingOriginalScriptPath())
	return err == nil && info.Mode().IsRegular()
}

func validateMissingOriginalRequest(req missingOriginalRequest) error {
	if !storageCandidateIDPattern.MatchString(req.CandidateID) || !req.Acknowledged {
		return fmt.Errorf("choose one saved candidate and explicitly acknowledge private scratch reconstruction")
	}
	return nil
}

// A valid archive does not imply native library restoration. Every required
// independent recovery proof flag must be present with its exact JSON type.
func validateMissingOriginalReport(data []byte, id, snapshot, output string, hashes map[string]string) (json.RawMessage, error) {
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, fmt.Errorf("invalid isolated recovery report length")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid missing-original recovery JSON")
	}
	for _, key := range []string{
		"verified", "readOnlyAgainstLibrary", "originalSourcesNotRead",
		"archiveSHA256Verified", "mediaBackupObjectsVerified", "backupContentDigestsCovered",
	} {
		if string(fields[key]) != "true" {
			return nil, fmt.Errorf("missing strict positive recovery proof: %s", key)
		}
	}
	for _, key := range []string{
		"requiresOriginalSources", "sourceToBackupPathMappingVerified",
		"databaseRecoveryVerified", "nativeMediaServingVerified",
		"crashSafeLiveRollbackVerified", "canActivate", "originalEvictionEnabled",
	} {
		if string(fields[key]) != "false" {
			return nil, fmt.Errorf("unsafe or missing recovery blocker: %s", key)
		}
	}
	var report missingOriginalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	if report.CandidateID != id || report.Snapshot != snapshot ||
		report.OutputPath != output || report.RestoredFiles != len(hashes) ||
		len(report.RestoredSHA256) != len(hashes) {
		return nil, fmt.Errorf("isolated recovery receipt does not match selected candidate")
	}
	for imageID, hash := range hashes {
		if !storageBackupSHA256.MatchString(hash) || report.RestoredSHA256[imageID] != hash {
			return nil, fmt.Errorf("missing-original recovery changed original image bytes")
		}
	}
	info, err := os.Lstat(output)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("missing private recovery output directory")
	}
	extraction, err := os.Lstat(filepath.Join(output, "extraction.json"))
	if err != nil || !extraction.Mode().IsRegular() {
		return nil, fmt.Errorf("missing complete scratch extraction receipt")
	}
	return append(json.RawMessage(nil), data...), nil
}

func runMissingOriginalRecovery(ctx context.Context, directory, store, id, snapshot, output string, hashes map[string]string) (json.RawMessage, error) {
	receipt, err := storageCandidateReceiptPath(directory, id)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, mediaBackupPython(), missingOriginalScriptPath(),
		"--receipt", receipt, "--store", store, "--output", output)
	data, err := cmd.CombinedOutput()
	if err != nil {
		if len(data) > 2048 {
			data = data[:2048]
		}
		return nil, fmt.Errorf("private missing-original recovery failed: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return validateMissingOriginalReport(data, id, snapshot, output, hashes)
}

func missingOriginalBusyLocked() bool {
	if missingOriginalBusy && missingOriginalJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(missingOriginalJobID)
		if current == nil || current.Status == job.StatusFinished ||
			current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			missingOriginalBusy = false
		}
	}
	return missingOriginalBusy
}

func (rs imageRoutes) MissingOriginalRehearsalStatus(w http.ResponseWriter, r *http.Request) {
	missingOriginalMutex.Lock()
	busy := missingOriginalBusyLocked()
	id := missingOriginalJobID
	result := append(json.RawMessage(nil), missingOriginalResult...)
	missingOriginalMutex.Unlock()
	status := missingOriginalStatus{Available: missingOriginalAvailable(), Busy: busy, JobID: id, Result: result}
	if cfg, err := readMediaBackupConfig(); err == nil {
		roots := configuredBackupRoots()
		if store, err := validateBackupDestination(cfg.Destination, roots); err == nil {
			if directory, err := storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, roots); err == nil {
				if record, err := missingOriginalReadJournal(directory); err != nil {
					status.JournalError = "Private recovery journal is unreadable; no persisted success may be trusted"
				} else {
					status.Journal = missingOriginalJournalForDisplay(record, busy)
				}
			}
		}
	}
	writeVisualSimilarityJSON(w, status)
}

func (rs imageRoutes) MissingOriginalRehearsalStart(w http.ResponseWriter, r *http.Request) {
	var request missingOriginalRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid recovery rehearsal request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected recovery request fields", http.StatusBadRequest)
		return
	}
	if err := validateMissingOriginalRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !missingOriginalAvailable() {
		http.Error(w, "isolated recovery scripts unavailable", http.StatusServiceUnavailable)
		return
	}
	missingOriginalMutex.Lock()
	defer missingOriginalMutex.Unlock()
	if missingOriginalBusyLocked() {
		http.Error(w, "another missing-original recovery is queued or running", http.StatusConflict)
		return
	}
	missingOriginalResult = nil
	missingOriginalBusy = true
	mgr := manager.GetInstance()
	id := mgr.JobManager.Add(r.Context(), "Rehearse missing-original recovery into private scratch directory", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				missingOriginalMutex.Lock()
				missingOriginalBusy = false
				missingOriginalMutex.Unlock()
			}()
			progress.SetTotal(3)
			if job.IsCancelled(ctx) {
				return nil
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
			raw, summary, err := storageCandidateReadReceipt(directory, request.CandidateID, true)
			if err != nil {
				return err
			}
			var candidate storageCandidateProof
			if err := json.Unmarshal(raw, &candidate); err != nil {
				return err
			}
			stackID, err := strconv.Atoi(strings.TrimPrefix(summary.Group, "stack:"))
			if err != nil || stackID < 1 || !strings.HasPrefix(request.CandidateID, fmt.Sprintf("stack-%d-", stackID)) {
				return fmt.Errorf("saved candidate lacks a valid Visual Stack identifier")
			}
			progress.SetProcessed(1)
			if job.IsCancelled(ctx) {
				return nil
			}
			output, err := nextStorageCandidateOutput(directory, stackID, "orphan-rehearsal")
			if err != nil {
				return err
			}
			output = strings.TrimSuffix(output, ".pack")
			journal := missingOriginalNewJournal(request.CandidateID, summary.Snapshot, output)
			if err := missingOriginalWriteJournal(directory, journal); err != nil {
				return fmt.Errorf("cannot durably record recovery attempt before scratch writes: %w", err)
			}
			result, err := runMissingOriginalRecovery(ctx, directory, store, request.CandidateID,
				summary.Snapshot, output, candidate.SourceSHA256)
			if err != nil {
				missingOriginalJournalFail(&journal)
				if journalErr := missingOriginalWriteJournal(directory, journal); journalErr != nil {
					return fmt.Errorf("scratch recovery failed: %v; failed to journal error: %w", err, journalErr)
				}
				return err
			}
			progress.SetProcessed(2)
			if job.IsCancelled(ctx) {
				// Leave running in the journal: the scratch result must be
				// independently inspected after an interrupted operation.
				return nil
			}
			missingOriginalJournalFinish(&journal, result, len(candidate.SourceSHA256))
			if err := missingOriginalWriteJournal(directory, journal); err != nil {
				return fmt.Errorf("scratch completed but durable recovery journal failed: %w", err)
			}
			if job.IsCancelled(ctx) {
				return nil
			}
			missingOriginalMutex.Lock()
			missingOriginalResult = result
			missingOriginalMutex.Unlock()
			progress.SetProcessed(3)
			return nil
		},
	))
	missingOriginalJobID = id
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: id})
}
