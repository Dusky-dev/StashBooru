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
	"time"

	"github.com/stashapp/stash/internal/manager"
)

type missingOriginalScratchAuditRequest struct {
	CandidateID string `json:"candidateID"`
}

type missingOriginalScratchAuditReport struct {
	CandidateID                   string `json:"candidateID"`
	OutputPath                    string `json:"outputPath"`
	State                         string `json:"state"`
	Complete                      bool   `json:"complete"`
	ScratchFilesVerified          int    `json:"scratchFilesVerified"`
	ScratchBytesVerified          int64  `json:"scratchBytesVerified"`
	ReadOnlyInspection            bool   `json:"readOnlyInspection"`
	CandidateArchiveReverified    bool   `json:"candidateArchiveReverified"`
	MediaBackupReverified         bool   `json:"mediaBackupReverified"`
	HistoricalPathMappingVerified bool   `json:"historicalPathMappingVerified"`
	DatabaseRecoveryVerified      bool   `json:"databaseRecoveryVerified"`
	NativeServingVerified         bool   `json:"nativeServingVerified"`
	CrashSafeLiveRollbackVerified bool   `json:"crashSafeLiveRollbackVerified"`
	CanActivate                   bool   `json:"canActivate"`
	OriginalEvictionEnabled       bool   `json:"originalEvictionEnabled"`
}

func missingOriginalScratchAuditScriptPath() string {
	if value := strings.TrimSpace(os.Getenv("STASH_IMAGE_SCRATCH_AUDIT_SCRIPT")); value != "" {
		return value
	}
	return filepath.FromSlash("scripts/image_storage_scratch_audit.py")
}

func validateMissingOriginalScratchAuditRequest(req missingOriginalScratchAuditRequest) error {
	if !storageCandidateIDPattern.MatchString(req.CandidateID) {
		return fmt.Errorf("choose a saved P12/P13 candidate, not a filesystem path")
	}
	return nil
}

func validateMissingOriginalScratchAuditReport(data []byte, candidateID, output string, sourceCount int) (missingOriginalScratchAuditReport, error) {
	if len(data) == 0 || len(data) > 16*1024 {
		return missingOriginalScratchAuditReport{}, fmt.Errorf("invalid scratch audit report length")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return missingOriginalScratchAuditReport{}, fmt.Errorf("invalid scratch audit JSON")
	}
	if string(raw["readOnlyInspection"]) != "true" {
		return missingOriginalScratchAuditReport{}, fmt.Errorf("scratch audit did not prove read-only inspection")
	}
	for _, field := range []string{
		"candidateArchiveReverified", "mediaBackupReverified",
		"historicalPathMappingVerified", "databaseRecoveryVerified",
		"nativeServingVerified", "crashSafeLiveRollbackVerified",
		"canActivate", "originalEvictionEnabled",
	} {
		if string(raw[field]) != "false" {
			return missingOriginalScratchAuditReport{}, fmt.Errorf("unsafe audit capability claim: %s", field)
		}
	}
	var report missingOriginalScratchAuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		return report, err
	}
	if report.CandidateID != candidateID || report.OutputPath != output || report.ScratchBytesVerified < 0 {
		return report, fmt.Errorf("audit result does not match the selected private recovery")
	}
	switch report.State {
	case "no_output", "incomplete":
		if report.Complete || report.ScratchFilesVerified != 0 || report.ScratchBytesVerified != 0 {
			return report, fmt.Errorf("incomplete audit falsely claims verified scratch files")
		}
	case "complete_verified":
		if !report.Complete || sourceCount < 2 || sourceCount > 16 ||
			report.ScratchFilesVerified != sourceCount || report.ScratchBytesVerified <= 0 {
			return report, fmt.Errorf("scratch audit has incomplete member coverage")
		}
	default:
		return report, fmt.Errorf("unknown private scratch audit state")
	}
	return report, nil
}

// This POST supplies only a saved candidate ID. Output and receipt paths always
// originate from the native private journal, never from a browser field.
func (rs imageRoutes) MissingOriginalScratchAudit(w http.ResponseWriter, r *http.Request) {
	var request missingOriginalScratchAuditRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid private scratch audit request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected private audit request body", http.StatusBadRequest)
		return
	}
	if err := validateMissingOriginalScratchAuditRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	missingOriginalMutex.Lock()
	busy := missingOriginalBusyLocked()
	missingOriginalMutex.Unlock()
	if busy {
		http.Error(w, "scratch recovery is still running; audit later", http.StatusConflict)
		return
	}
	script := missingOriginalScratchAuditScriptPath()
	if info, err := os.Lstat(script); err != nil || !info.Mode().IsRegular() {
		http.Error(w, "read-only scratch audit script unavailable", http.StatusServiceUnavailable)
		return
	}
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, "media backup settings unavailable", http.StatusServiceUnavailable)
		return
	}
	roots := configuredBackupRoots()
	store, err := validateBackupDestination(cfg.Destination, roots)
	if err != nil {
		http.Error(w, "protected media backup destination unavailable", http.StatusServiceUnavailable)
		return
	}
	folder, err := storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, roots)
	if err != nil {
		http.Error(w, "private candidate directory unavailable", http.StatusServiceUnavailable)
		return
	}
	journal, err := missingOriginalReadJournal(folder)
	if err != nil || journal == nil || journal.CandidateID != request.CandidateID {
		http.Error(w, "no trustworthy matching private recovery journal", http.StatusConflict)
		return
	}
	rawReceipt, _, err := storageCandidateReadReceipt(folder, request.CandidateID, true)
	if err != nil {
		http.Error(w, "saved candidate receipt/archive verification failed", http.StatusConflict)
		return
	}
	var proof storageCandidateProof
	if err := json.Unmarshal(rawReceipt, &proof); err != nil ||
		len(proof.SourceSHA256) < 2 || len(proof.SourceSHA256) > 16 || proof.MediaBackupSnapshot != journal.Snapshot {
		http.Error(w, "saved candidate identity mismatch", http.StatusConflict)
		return
	}
	receipt, err := storageCandidateReceiptPath(folder, request.CandidateID)
	if err != nil {
		http.Error(w, "invalid private candidate receipt", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mediaBackupPython(), script,
		"--receipt", receipt, "--output", journal.OutputPath)
	bytes, err := cmd.Output()
	if err != nil {
		http.Error(w, "private scratch audit refused or detected unsafe output", http.StatusConflict)
		return
	}
	report, err := validateMissingOriginalScratchAuditReport(bytes, request.CandidateID,
		journal.OutputPath, len(proof.SourceSHA256))
	if err != nil {
		http.Error(w, "private scratch audit proof validation failed", http.StatusConflict)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeVisualSimilarityJSON(w, report)
}
