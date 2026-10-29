package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

const storageCandidateDirName = "image-storage-candidates"

var storageCandidateMutex sync.Mutex
var storageCandidateBusy bool
var storageCandidateJobID int
var storageCandidateResult json.RawMessage

type storageCandidateRequest struct {
	StackID      int    `json:"stackID"`
	Snapshot     string `json:"snapshot"`
	Method       string `json:"method"`
	Acknowledged bool   `json:"acknowledged"`
}

type storageCandidateStatus struct {
	Available  bool                      `json:"available"`
	Busy       bool                      `json:"busy"`
	JobID      int                       `json:"jobID,omitempty"`
	Result     json.RawMessage           `json:"result,omitempty"`
	Candidates []storageCandidateSummary `json:"candidates,omitempty"`
}

type storageCandidateProof struct {
	Group                  string            `json:"group"`
	MediaBackupSnapshot    string            `json:"mediaBackupSnapshot"`
	Method                 string            `json:"method"`
	Fidelity               string            `json:"fidelity"`
	SourceFiles            int               `json:"sourceFiles"`
	SourceBytes            int64             `json:"sourceBytes"`
	ArchiveBytes           int64             `json:"archiveBytes"`
	SourceSHA256           map[string]string `json:"sourceSHA256"`
	Verified               bool              `json:"verified"`
	BackupVerified         bool              `json:"backupVerified"`
	SourcesRetained        bool              `json:"sourcesRetained"`
	NativePackingCandidate bool              `json:"nativePackingCandidateOnly"`
	NativeServing          bool              `json:"nativeServing"`
	NativeServingVerified  bool              `json:"nativeServingVerified"`
	DatabaseBackupVerified bool              `json:"databaseBackupVerified"`
	OriginalEviction       bool              `json:"originalEvictionEnabled"`
	ArchiveActivated       bool              `json:"archiveActivated"`
}

func validateStorageCandidateRequest(req storageCandidateRequest) error {
	if req.StackID < 1 || !mediaBackupSnapshotID.MatchString(req.Snapshot) {
		return fmt.Errorf("select a valid Visual Stack ID and media backup snapshot")
	}
	if req.Method != "delta" && req.Method != "shared-tiles" {
		return fmt.Errorf("only byte-exact delta or shared-tiles methods are supported")
	}
	if !req.Acknowledged {
		return fmt.Errorf("explicit acknowledgement is required for experimental disk usage")
	}
	return nil
}

func storageCandidateScriptPath() string {
	if path := strings.TrimSpace(os.Getenv("STASH_IMAGE_NATIVE_CANDIDATE_SCRIPT")); path != "" {
		return path
	}
	return filepath.FromSlash("scripts/image_storage_native_candidate.py")
}

func storageCandidateScriptsAvailable() bool {
	if !storageBackupPreflightScriptsAvailable() {
		return false
	}
	for _, path := range []string{storageCandidateScriptPath(), filepath.FromSlash("scripts/image_storage_pack_proof.py")} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// Outputs are server-managed, never supplied by browser requests. Reject any
// overlap between candidate, original media, and immutable media backup roots.
func storageCandidateDirectory(configPath, backupStore string, mediaRoots []string) (string, error) {
	if !filepath.IsAbs(configPath) || !filepath.IsAbs(backupStore) || len(mediaRoots) == 0 {
		return "", fmt.Errorf("candidate storage needs absolute config, backup and media roots")
	}
	config, err := filepath.EvalSymlinks(filepath.Clean(configPath))
	if err != nil {
		return "", fmt.Errorf("cannot resolve application configuration: %w", err)
	}
	info, err := os.Stat(config)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("configuration root must exist")
	}
	candidate := filepath.Join(config, storageCandidateDirName)
	if existing, err := os.Lstat(candidate); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 || !existing.IsDir() {
			return "", fmt.Errorf("candidate output is a symlink or non-directory")
		}
		resolvedCandidate, err := filepath.EvalSymlinks(candidate)
		if err != nil || resolvedCandidate != candidate {
			return "", fmt.Errorf("candidate output uses a symlink alias")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot inspect candidate output: %w", err)
	}
	protected := append(append([]string{}, mediaRoots...), backupStore)
	for _, location := range protected {
		resolved, err := filepath.EvalSymlinks(location)
		if err != nil {
			return "", fmt.Errorf("cannot resolve protected media or backup root: %w", err)
		}
		if mediaBackupContains(resolved, candidate) || mediaBackupContains(candidate, resolved) {
			return "", fmt.Errorf("candidate output must not overlap media roots or backup storage")
		}
	}
	return candidate, nil
}

func prepareStorageCandidateDirectory(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("candidate directory is not a regular directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("candidate directory is not private (mode 0700 required)")
	}
	return nil
}

func nextStorageCandidateOutput(folder string, stackID int, method string) (string, error) {
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	output := filepath.Join(folder, fmt.Sprintf("stack-%d-%s-%s.pack", stackID, method, hex.EncodeToString(nonce[:])))
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return "", fmt.Errorf("candidate filename unavailable")
	}
	return output, nil
}

func validateStorageCandidateProof(data []byte, group estimateSourceGroup, snapshot, method, output string) (json.RawMessage, error) {
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, fmt.Errorf("unexpected candidate report length")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid native candidate report")
	}
	for _, key := range []string{"verified", "backupVerified", "sourcesRetained", "nativePackingCandidateOnly"} {
		if string(fields[key]) != "true" {
			return nil, fmt.Errorf("candidate report did not prove %s", key)
		}
	}
	for _, key := range []string{"nativeServing", "nativeServingVerified", "databaseBackupVerified", "originalEvictionEnabled", "archiveActivated"} {
		if string(fields[key]) != "false" {
			return nil, fmt.Errorf("candidate report unexpectedly enabled %s", key)
		}
	}
	var receipt storageCandidateProof
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("invalid candidate evidence: %w", err)
	}
	if receipt.Group != group.ID || receipt.MediaBackupSnapshot != snapshot ||
		receipt.Method != method || receipt.Fidelity != "byte-exact" ||
		receipt.SourceFiles != len(group.Members) ||
		receipt.SourceBytes <= 0 || receipt.ArchiveBytes <= 0 ||
		len(receipt.SourceSHA256) != len(group.Members) {
		return nil, fmt.Errorf("candidate proof selection, method or byte-exact evidence mismatch")
	}
	for _, member := range group.Members {
		digest, ok := receipt.SourceSHA256[member.ID]
		if !ok || !storageBackupSHA256.MatchString(digest) {
			return nil, fmt.Errorf("candidate is missing an original SHA-256 proof")
		}
	}
	info, err := os.Lstat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() != receipt.ArchiveBytes {
		return nil, fmt.Errorf("candidate archive is missing or has an unexpected size")
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	result["archivePath"] = output
	return json.Marshal(result)
}

func runNativeStorageCandidate(ctx context.Context, group estimateSourceGroup, store, snapshot, method, output string) (json.RawMessage, error) {
	input, err := os.CreateTemp("", "stash-candidate-*.json")
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
	cmd := exec.CommandContext(ctx, mediaBackupPython(), storageCandidateScriptPath(),
		"--stacks", input.Name(), "--group", group.ID,
		"--store", store, "--snapshot", snapshot, "--method", method, "--output", output)
	data, err := cmd.CombinedOutput()
	if err != nil {
		if len(data) > 2048 {
			data = data[:2048]
		}
		return nil, fmt.Errorf("native candidate packing failed: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return validateStorageCandidateProof(data, group, snapshot, method, output)
}

func storageCandidateBusyLocked() bool {
	if storageCandidateBusy && storageCandidateJobID != 0 {
		j := manager.GetInstance().JobManager.GetJob(storageCandidateJobID)
		if j == nil || j.Status == job.StatusFinished || j.Status == job.StatusFailed || j.Status == job.StatusCancelled {
			storageCandidateBusy = false
		}
	}
	return storageCandidateBusy
}

func (rs imageRoutes) StorageCandidateStatus(w http.ResponseWriter, r *http.Request) {
	storageCandidateMutex.Lock()
	busy := storageCandidateBusyLocked()
	id := storageCandidateJobID
	report := append(json.RawMessage(nil), storageCandidateResult...)
	storageCandidateMutex.Unlock()
	var saved []storageCandidateSummary
	if cfg, err := readMediaBackupConfig(); err == nil {
		if store, err := validateBackupDestination(cfg.Destination, configuredBackupRoots()); err == nil {
			if dir, err := storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, configuredBackupRoots()); err == nil {
				saved = storageCandidateSavedList(dir)
			}
		}
	}
	writeVisualSimilarityJSON(w, storageCandidateStatus{
		Available:  storageCandidateScriptsAvailable(),
		Busy:       busy,
		JobID:      id,
		Result:     report,
		Candidates: saved,
	})
}

func (rs imageRoutes) StorageCandidateStart(w http.ResponseWriter, r *http.Request) {
	var req storageCandidateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid candidate packing request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected candidate packing data", http.StatusBadRequest)
		return
	}
	if err := validateStorageCandidateRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !storageCandidateScriptsAvailable() {
		http.Error(w, "native candidate packing scripts unavailable", http.StatusServiceUnavailable)
		return
	}
	storageCandidateMutex.Lock()
	defer storageCandidateMutex.Unlock()
	if storageCandidateBusyLocked() {
		http.Error(w, "a native image candidate job is already queued or running", http.StatusConflict)
		return
	}
	storageCandidateResult = nil
	storageCandidateBusy = true
	mgr := manager.GetInstance()
	id := mgr.JobManager.Add(r.Context(), "Create verified experimental image candidate", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				storageCandidateMutex.Lock()
				storageCandidateBusy = false
				storageCandidateMutex.Unlock()
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
			dir, err := storageCandidateDirectory(mgr.Config.GetConfigPath(), store, roots)
			if err != nil {
				return err
			}
			if err := prepareStorageCandidateDirectory(dir); err != nil {
				return err
			}
			checked, err := storageCandidateDirectory(mgr.Config.GetConfigPath(), store, configuredBackupRoots())
			if err != nil || checked != dir {
				return fmt.Errorf("candidate output location changed during preparation")
			}
			output, err := nextStorageCandidateOutput(dir, req.StackID, req.Method)
			if err != nil {
				return err
			}
			progress.SetProcessed(2)
			if job.IsCancelled(ctx) {
				return nil
			}
			report, err := runNativeStorageCandidate(ctx, group, store, req.Snapshot, req.Method, output)
			if err != nil {
				return err
			}
			progress.SetProcessed(3)
			if job.IsCancelled(ctx) {
				return nil
			}
			if _, err := storageCandidateSaveReceipt(dir, report); err != nil {
				return fmt.Errorf("candidate verified but receipt publication failed: %w", err)
			}
			storageCandidateMutex.Lock()
			storageCandidateResult = report
			storageCandidateMutex.Unlock()
			progress.SetProcessed(4)
			return nil
		},
	))
	storageCandidateJobID = id
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: id})
}
