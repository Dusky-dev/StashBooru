package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

var storageRestorePlanMutex sync.Mutex
var storageRestorePlanBusy bool
var storageRestorePlanJobID int
var storageRestorePlanResult json.RawMessage

type storageRestorePlanRequest struct {
	StackID     int    `json:"stackID"`
	CandidateID string `json:"candidateID"`
}

type storageRestorePlanStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type storageRestorePlanReport struct {
	Experimental            bool     `json:"experimental"`
	ReadOnly                bool     `json:"readOnly"`
	Inspected               bool     `json:"inspected"`
	CandidateID             string   `json:"candidateID"`
	Group                   string   `json:"group"`
	Method                  string   `json:"method"`
	Snapshot                string   `json:"snapshot"`
	Images                  int      `json:"images"`
	SourceBytes             int64    `json:"sourceBytes"`
	ArchiveBytes            int64    `json:"archiveBytes"`
	ArchiveSHA256Verified   bool     `json:"archiveSHA256Verified"`
	MediaBackupVerified     bool     `json:"mediaBackupVerified"`
	CurrentSourcesMatch     bool     `json:"currentSourcesMatch"`
	CandidateHashesMatch    bool     `json:"candidateHashesMatch"`
	DatabaseBackupVerified  bool     `json:"databaseBackupVerified"`
	NativeServingVerified   bool     `json:"nativeServingVerified"`
	RollbackVerified        bool     `json:"rollbackVerified"`
	CanActivate             bool     `json:"canActivate"`
	CanReplaceOriginals     bool     `json:"canReplaceOriginals"`
	OriginalEvictionEnabled bool     `json:"originalEvictionEnabled"`
	Blockers                []string `json:"blockers"`
}

func validateStorageRestorePlanRequest(req storageRestorePlanRequest) error {
	if req.StackID <= 0 || !storageCandidateIDPattern.MatchString(req.CandidateID) {
		return fmt.Errorf("choose a valid Visual Stack and saved candidate ID")
	}
	return nil
}

// This report can NEVER authorize pack activation or source eviction. It only
// checks that the current originals, media snapshot and saved archive agree.
func buildStorageRestorePlan(group estimateSourceGroup, id string, candidate storageCandidateProof, backup storageBackupPreflightReport) (storageRestorePlanReport, error) {
	if candidate.Group != group.ID || backup.Group != group.ID ||
		candidate.MediaBackupSnapshot != backup.Snapshot ||
		candidate.Method != "delta" && candidate.Method != "shared-tiles" ||
		candidate.Fidelity != "byte-exact" || !candidate.Verified || !candidate.BackupVerified ||
		!candidate.SourcesRetained || !candidate.NativePackingCandidate ||
		candidate.NativeServing || candidate.NativeServingVerified ||
		candidate.DatabaseBackupVerified || candidate.OriginalEviction || candidate.ArchiveActivated ||
		!backup.ReadOnly || !backup.Verified || !backup.MediaBackupVerified ||
		!backup.SourcesRetained || backup.NativePackingEnabled ||
		backup.NativeServingVerified || backup.DatabaseBackupVerified || backup.OriginalEvictionEnabled ||
		candidate.SourceBytes <= 0 || candidate.ArchiveBytes <= 0 ||
		candidate.SourceFiles != len(group.Members) || len(candidate.SourceSHA256) != len(group.Members) ||
		backup.SelectedImagesCovered != len(group.Members) || len(backup.ImageSHA256) != len(group.Members) {
		return storageRestorePlanReport{}, fmt.Errorf("saved candidate and verified backup do not meet read-only safety contract")
	}
	for _, member := range group.Members {
		fromArchive, ok := candidate.SourceSHA256[member.ID]
		fromBackup, covered := backup.ImageSHA256[member.ID]
		if !ok || !covered || !storageBackupSHA256.MatchString(fromArchive) ||
			fromArchive != fromBackup {
			return storageRestorePlanReport{}, fmt.Errorf("candidate, current source and backup SHA-256 differ")
		}
	}
	return storageRestorePlanReport{
		Experimental:            true,
		ReadOnly:                true,
		Inspected:               true,
		CandidateID:             id,
		Group:                   group.ID,
		Method:                  candidate.Method,
		Snapshot:                candidate.MediaBackupSnapshot,
		Images:                  len(group.Members),
		SourceBytes:             candidate.SourceBytes,
		ArchiveBytes:            candidate.ArchiveBytes,
		ArchiveSHA256Verified:   true,
		MediaBackupVerified:     true,
		CurrentSourcesMatch:     true,
		CandidateHashesMatch:    true,
		DatabaseBackupVerified:  false,
		NativeServingVerified:   false,
		RollbackVerified:        false,
		CanActivate:             false,
		CanReplaceOriginals:     false,
		OriginalEvictionEnabled: false,
		Blockers: []string{
			"Database and application configuration recovery has not been verified",
			"Native packed-media serving and compatibility with all media consumers has not been verified",
			"Crash-safe live restoration and rollback have not been verified",
			"Replacement and eviction of originals remain prohibited",
		},
	}, nil
}

func storageRestorePlanBusyLocked() bool {
	if storageRestorePlanBusy && storageRestorePlanJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(storageRestorePlanJobID)
		if current == nil || current.Status == job.StatusFinished ||
			current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			storageRestorePlanBusy = false
		}
	}
	return storageRestorePlanBusy
}

func (rs imageRoutes) StorageRestorePlanStatus(w http.ResponseWriter, r *http.Request) {
	storageRestorePlanMutex.Lock()
	busy := storageRestorePlanBusyLocked()
	id := storageRestorePlanJobID
	result := append(json.RawMessage(nil), storageRestorePlanResult...)
	storageRestorePlanMutex.Unlock()
	writeVisualSimilarityJSON(w, storageRestorePlanStatus{
		Available: storageCandidateScriptsAvailable() && storageBackupPreflightScriptsAvailable(),
		Busy:      busy,
		JobID:     id,
		Result:    result,
	})
}

func (rs imageRoutes) StorageRestorePlanStart(w http.ResponseWriter, r *http.Request) {
	var req storageRestorePlanRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid restore readiness request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected restore readiness request content", http.StatusBadRequest)
		return
	}
	if err := validateStorageRestorePlanRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !storageCandidateScriptsAvailable() || !storageBackupPreflightScriptsAvailable() {
		http.Error(w, "original media verification scripts unavailable", http.StatusServiceUnavailable)
		return
	}
	storageRestorePlanMutex.Lock()
	defer storageRestorePlanMutex.Unlock()
	if storageRestorePlanBusyLocked() {
		http.Error(w, "restore readiness inspection already queued or running", http.StatusConflict)
		return
	}
	storageRestorePlanResult = nil
	storageRestorePlanBusy = true
	mgr := manager.GetInstance()
	id := mgr.JobManager.Add(r.Context(), "Inspect saved image candidate restore readiness (read-only)", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageRestorePlanMutex.Lock()
				storageRestorePlanBusy = false
				storageRestorePlanMutex.Unlock()
			}()
			progress.SetTotal(4)
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
			roots := configuredBackupRoots()
			store, err := validateBackupDestination(cfg.Destination, roots)
			if err != nil {
				return err
			}
			directory, err := storageCandidateDirectory(mgr.Config.GetConfigPath(), store, roots)
			if err != nil {
				return err
			}
			candidateJSON, saved, err := storageCandidateReadReceipt(directory, req.CandidateID, true)
			if err != nil {
				return err
			}
			if saved.Group != group.ID {
				return fmt.Errorf("saved candidate does not belong to selected Visual Stack")
			}
			var candidate storageCandidateProof
			if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
				return err
			}
			archive := filepath.Join(directory, req.CandidateID+".pack")
			if _, err := validateStorageCandidateProof(candidateJSON, group, saved.Snapshot, saved.Method, archive); err != nil {
				return err
			}
			progress.SetProcessed(2)
			if job.IsCancelled(ctx) {
				return nil
			}
			backupJSON, err := runStorageBackupPreflight(ctx, group, store, saved.Snapshot)
			if err != nil {
				return err
			}
			var backup storageBackupPreflightReport
			if err := json.Unmarshal(backupJSON, &backup); err != nil {
				return err
			}
			progress.SetProcessed(3)
			if job.IsCancelled(ctx) {
				return nil
			}
			report, err := buildStorageRestorePlan(group, req.CandidateID, candidate, backup)
			if err != nil {
				return err
			}
			// Point-in-time inspection: neither report nor job authorizes live
			// restoration, and archive/source hashes must be rechecked later.
			data, err := json.Marshal(report)
			if err != nil {
				return err
			}
			storageRestorePlanMutex.Lock()
			storageRestorePlanResult = data
			storageRestorePlanMutex.Unlock()
			progress.SetProcessed(4)
			return nil
		},
	))
	storageRestorePlanJobID = id
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: id})
}
