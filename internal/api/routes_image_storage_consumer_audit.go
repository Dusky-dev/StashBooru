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
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

var storageConsumerAuditMutex sync.Mutex
var storageConsumerAuditBusy bool
var storageConsumerAuditJobID int
var storageConsumerAuditResult json.RawMessage

type storageConsumerAuditRequest struct {
	StackID     int    `json:"stackID"`
	CandidateID string `json:"candidateID"`
}

type storageConsumerAuditStatus struct {
	Available bool            `json:"available"`
	Busy      bool            `json:"busy"`
	JobID     int             `json:"jobID,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type storageConsumerAuditMember struct {
	ID              string `json:"id"`
	Bytes           int    `json:"bytes"`
	SHA256          string `json:"sha256"`
	ByteExact       bool   `json:"byteExact"`
	OriginalPresent bool   `json:"originalPresent"`
	ThumbnailCache  string `json:"thumbnailCache"`
	PreviewCache    string `json:"previewCache"`
}

type storageConsumerAuditReport struct {
	Verified                bool                         `json:"verified"`
	ReadOnly                bool                         `json:"readOnly"`
	Group                   string                       `json:"group"`
	CandidateID             string                       `json:"candidateID"`
	Method                  string                       `json:"method"`
	Snapshot                string                       `json:"mediaBackupSnapshot"`
	MediaBackupVerified     bool                         `json:"mediaBackupVerified"`
	ArchiveSHA256Verified   bool                         `json:"archiveSHA256Verified"`
	Members                 []storageConsumerAuditMember `json:"members"`
	NativeConsumersVerified bool                         `json:"nativeConsumersVerified"`
	NativeServingActivated  bool                         `json:"nativeServingActivated"`
	CanActivate             bool                         `json:"canActivate"`
	OriginalEvictionEnabled bool                         `json:"originalEvictionEnabled"`
	Blockers                []string                     `json:"blockers"`
}

func storageConsumerAuditScript() string {
	if override := strings.TrimSpace(os.Getenv("STASH_IMAGE_SHADOW_AUDIT_SCRIPT")); override != "" {
		return override
	}
	return filepath.FromSlash("scripts/image_storage_shadow_audit.py")
}

func storageConsumerAuditAvailable() bool {
	if !storageCandidateScriptsAvailable() || !storageBackupPreflightScriptsAvailable() {
		return false
	}
	info, err := os.Lstat(storageConsumerAuditScript())
	return err == nil && info.Mode().IsRegular()
}

func validateStorageConsumerAuditRequest(req storageConsumerAuditRequest) error {
	if req.StackID < 1 || !storageCandidateIDPattern.MatchString(req.CandidateID) {
		return fmt.Errorf("choose a valid Visual Stack and saved candidate")
	}
	return nil
}

func storageConsumerCacheState(path string) string {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil || !info.Mode().IsRegular() {
		return "unsafe_or_unreadable"
	}
	return "cached_regular_unverified"
}

// Read only: report whether generated cache files currently exist. A cache
// file existing is NOT evidence that decoding or serving actually works.
func storageConsumerCacheStates(ctx context.Context, group estimateSourceGroup) (map[string][2]string, error) {
	mgr := manager.GetInstance()
	cache := make(map[string][2]string, len(group.Members))
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		for _, member := range group.Members {
			n, err := strconv.Atoi(strings.TrimPrefix(member.ID, "image:"))
			if err != nil || n < 1 || member.ID != fmt.Sprintf("image:%d", n) {
				return fmt.Errorf("invalid native member identity")
			}
			image, err := mgr.Repository.Image.Find(ctx, n)
			if err != nil || image == nil {
				return fmt.Errorf("image missing during cache read-only inspection")
			}
			cache[member.ID] = [2]string{
				storageConsumerCacheState(mgr.Paths.Generated.GetThumbnailPath(image.Checksum, models.DefaultGthumbWidth)),
				storageConsumerCacheState(mgr.Paths.Generated.GetClipPreviewPath(image.Checksum, models.DefaultGthumbWidth)),
			}
		}
		return nil
	})
	return cache, err
}

func validateStorageConsumerAuditReport(data []byte, group estimateSourceGroup, candidate storageCandidateProof, id string, caches map[string][2]string) (json.RawMessage, error) {
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, fmt.Errorf("shadow audit report has invalid length")
	}
	var flags map[string]json.RawMessage
	if err := json.Unmarshal(data, &flags); err != nil || flags == nil {
		return nil, fmt.Errorf("invalid shadow audit JSON")
	}
	for _, key := range []string{"verified", "readOnly"} {
		if string(flags[key]) != "true" {
			return nil, fmt.Errorf("shadow audit does not prove %s", key)
		}
	}
	for _, key := range []string{"archiveActivated", "databaseModified", "nativeServingActivated", "originalEvictionEnabled"} {
		if string(flags[key]) != "false" {
			return nil, fmt.Errorf("shadow audit unexpectedly enabled %s", key)
		}
	}
	var decoded struct {
		Group    string `json:"group"`
		Method   string `json:"method"`
		Snapshot string `json:"mediaBackupSnapshot"`
		Members  []struct {
			ID              string `json:"id"`
			Bytes           int    `json:"bytes"`
			SHA256          string `json:"sha256"`
			ByteExact       bool   `json:"byteExact"`
			OriginalPresent bool   `json:"originalPresent"`
		} `json:"members"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	if decoded.Group != group.ID || decoded.Method != candidate.Method ||
		decoded.Snapshot != candidate.MediaBackupSnapshot ||
		len(decoded.Members) != len(group.Members) || len(caches) != len(group.Members) {
		return nil, fmt.Errorf("audit selection does not match current Visual Stack")
	}
	seen := make(map[string]struct{}, len(group.Members))
	rows := make([]storageConsumerAuditMember, 0, len(group.Members))
	for _, item := range decoded.Members {
		digest, ok := candidate.SourceSHA256[item.ID]
		if !ok || !item.ByteExact || !item.OriginalPresent || !storageBackupSHA256.MatchString(item.SHA256) ||
			digest != item.SHA256 || item.Bytes <= 0 || item.Bytes > storageShadowMaxImageBytes {
			return nil, fmt.Errorf("invalid byte-exact shadow member evidence")
		}
		if _, exists := seen[item.ID]; exists {
			return nil, fmt.Errorf("duplicate audited image")
		}
		seen[item.ID] = struct{}{}
		cache, ok := caches[item.ID]
		if !ok {
			return nil, fmt.Errorf("missing native cache check for member")
		}
		rows = append(rows, storageConsumerAuditMember{
			ID: item.ID, Bytes: item.Bytes, SHA256: item.SHA256,
			ByteExact: true, OriginalPresent: true,
			ThumbnailCache: cache[0], PreviewCache: cache[1],
		})
	}
	for _, member := range group.Members {
		if _, ok := seen[member.ID]; !ok {
			return nil, fmt.Errorf("audit skipped an original image")
		}
	}
	report := storageConsumerAuditReport{
		Verified: true, ReadOnly: true, Group: group.ID, CandidateID: id,
		Method: candidate.Method, Snapshot: candidate.MediaBackupSnapshot,
		MediaBackupVerified: true, ArchiveSHA256Verified: true,
		Members: rows,
		NativeConsumersVerified: false, NativeServingActivated: false,
		CanActivate: false, OriginalEvictionEnabled: false,
		Blockers: []string{
			"Native image, thumbnail, preview and download response compatibility has not been tested",
			"Cache presence is a filesystem observation, not a successful render or serving test",
			"Recovery after missing originals and database/configuration rollback are not proven",
			"Automatic packed serving and original-file eviction are disabled",
		},
	}
	return json.Marshal(report)
}

func runStorageConsumerAudit(ctx context.Context, group estimateSourceGroup, archive string, candidate storageCandidateProof, id string, caches map[string][2]string) (json.RawMessage, error) {
	spec, err := json.Marshal(map[string]any{
		"group": group.ID, "members": group.Members,
		"archivePath": archive, "method": candidate.Method,
		"mediaBackupSnapshot": candidate.MediaBackupSnapshot,
		"sourceSHA256": candidate.SourceSHA256,
	})
	if err != nil {
		return nil, err
	}
	input, err := os.CreateTemp("", "stash-shadow-audit-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(input.Name())
	if err := input.Chmod(0600); err != nil {
		input.Close()
		return nil, err
	}
	if _, err := input.Write(spec); err != nil {
		input.Close()
		return nil, err
	}
	if err := input.Close(); err != nil {
		return nil, err
	}
	data, err := exec.CommandContext(ctx, mediaBackupPython(), storageConsumerAuditScript(), "--spec", input.Name()).CombinedOutput()
	if err != nil {
		if len(data) > 1024 {
			data = data[:1024]
		}
		return nil, fmt.Errorf("shadow audit refused: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return validateStorageConsumerAuditReport(data, group, candidate, id, caches)
}

func storageConsumerAuditBusyLocked() bool {
	if storageConsumerAuditBusy && storageConsumerAuditJobID != 0 {
		current := manager.GetInstance().JobManager.GetJob(storageConsumerAuditJobID)
		if current == nil || current.Status == job.StatusFinished ||
			current.Status == job.StatusFailed || current.Status == job.StatusCancelled {
			storageConsumerAuditBusy = false
		}
	}
	return storageConsumerAuditBusy
}

func (rs imageRoutes) StorageConsumerAuditStatus(w http.ResponseWriter, r *http.Request) {
	storageConsumerAuditMutex.Lock()
	busy := storageConsumerAuditBusyLocked()
	id := storageConsumerAuditJobID
	result := append(json.RawMessage(nil), storageConsumerAuditResult...)
	storageConsumerAuditMutex.Unlock()
	writeVisualSimilarityJSON(w, storageConsumerAuditStatus{
		Available: storageConsumerAuditAvailable(), Busy: busy, JobID: id, Result: result,
	})
}

func (rs imageRoutes) StorageConsumerAuditStart(w http.ResponseWriter, r *http.Request) {
	var request storageConsumerAuditRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid audit request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected audit request fields", http.StatusBadRequest)
		return
	}
	if err := validateStorageConsumerAuditRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !storageConsumerAuditAvailable() {
		http.Error(w, "shadow audit scripts unavailable", http.StatusServiceUnavailable)
		return
	}
	storageConsumerAuditMutex.Lock()
	defer storageConsumerAuditMutex.Unlock()
	if storageConsumerAuditBusyLocked() {
		http.Error(w, "packed image audit already queued or running", http.StatusConflict)
		return
	}
	storageConsumerAuditBusy = true
	storageConsumerAuditResult = nil
	mgr := manager.GetInstance()
	id := mgr.JobManager.Add(r.Context(), "Audit P12/P13 image member and cache compatibility (read-only)", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageConsumerAuditMutex.Lock()
				storageConsumerAuditBusy = false
				storageConsumerAuditMutex.Unlock()
			}()
			progress.SetTotal(4)
			if job.IsCancelled(ctx) {
				return nil
			}
			group, err := estimateStackSources(ctx, request.StackID)
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
			savedJSON, summary, err := storageCandidateReadReceipt(directory, request.CandidateID, true)
			if err != nil || summary.Group != group.ID {
				return fmt.Errorf("candidate receipt missing, altered or selected stack mismatch")
			}
			var candidate storageCandidateProof
			if err := json.Unmarshal(savedJSON, &candidate); err != nil {
				return err
			}
			archive := filepath.Join(directory, request.CandidateID+".pack")
			if _, err := validateStorageCandidateProof(savedJSON, group, summary.Snapshot, summary.Method, archive); err != nil {
				return err
			}
			progress.SetProcessed(2)
			backupJSON, err := runStorageBackupPreflight(ctx, group, store, summary.Snapshot)
			if err != nil {
				return err
			}
			var backup storageBackupPreflightReport
			if err := json.Unmarshal(backupJSON, &backup); err != nil {
				return err
			}
			if _, err := buildStorageRestorePlan(group, request.CandidateID, candidate, backup); err != nil {
				return err
			}
			progress.SetProcessed(3)
			if job.IsCancelled(ctx) {
				return nil
			}
			caches, err := storageConsumerCacheStates(ctx, group)
			if err != nil {
				return err
			}
			report, err := runStorageConsumerAudit(ctx, group, archive, candidate, request.CandidateID, caches)
			if err != nil {
				return err
			}
			storageConsumerAuditMutex.Lock()
			storageConsumerAuditResult = report
			storageConsumerAuditMutex.Unlock()
			progress.SetProcessed(4)
			return nil
		},
	))
	storageConsumerAuditJobID = id
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: id})
}
