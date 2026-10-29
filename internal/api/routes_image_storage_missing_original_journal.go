package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const missingOriginalJournalFile = "missing-original-latest-journal.json"
const missingOriginalJournalMax = 16 * 1024

var missingOriginalScratchName = regexp.MustCompile(`^stack-[1-9][0-9]*-orphan-rehearsal-[0-9a-f]{24}$`)

// Only the latest scratch-only recovery attempt is journaled. Its record is
// not a substitute for fresh archive, backup, or scratch-byte verification.
type missingOriginalJournalRecord struct {
	Version                   int    `json:"version"`
	CandidateID               string `json:"candidateID"`
	Snapshot                  string `json:"snapshot"`
	OutputPath                string `json:"outputPath"`
	State                     string `json:"state"`
	StartedUnix               int64  `json:"startedUnix"`
	UpdatedUnix               int64  `json:"updatedUnix"`
	RestoredFiles             int    `json:"restoredFiles"`
	ReportSHA256              string `json:"reportSHA256,omitempty"`
	ScratchProofAtCompletion  bool   `json:"scratchProofAtCompletion"`
	CanActivate               bool   `json:"canActivate"`
	OriginalEvictionEnabled   bool   `json:"originalEvictionEnabled"`
	DatabaseRecoveryVerified  bool   `json:"databaseRecoveryVerified"`
	CrashSafeRollbackVerified bool   `json:"crashSafeRollbackVerified"`
}

type missingOriginalJournalStatus struct {
	CandidateID     string `json:"candidateID"`
	OutputPath      string `json:"outputPath"`
	State           string `json:"state"`
	UpdatedUnix     int64  `json:"updatedUnix"`
	RestoredFiles   int    `json:"restoredFiles"`
	RequiresReaudit bool   `json:"requiresReaudit"`
	CanActivate     bool   `json:"canActivate"`
	EvictionEnabled bool   `json:"evictionEnabled"`
}

func missingOriginalValidateJournal(directory string, r missingOriginalJournalRecord) error {
	if r.Version != 1 || !storageCandidateIDPattern.MatchString(r.CandidateID) ||
		len(r.Snapshot) == 0 || len(r.Snapshot) > 96 || strings.ContainsAny(r.Snapshot, "/\\\x00\r\n") ||
		!filepath.IsAbs(r.OutputPath) || filepath.Clean(r.OutputPath) != r.OutputPath ||
		filepath.Dir(r.OutputPath) != filepath.Clean(directory) ||
		!missingOriginalScratchName.MatchString(filepath.Base(r.OutputPath)) ||
		r.StartedUnix <= 0 || r.UpdatedUnix < r.StartedUnix || r.UpdatedUnix > time.Now().Add(24*time.Hour).Unix() ||
		r.CanActivate || r.OriginalEvictionEnabled || r.DatabaseRecoveryVerified || r.CrashSafeRollbackVerified {
		return fmt.Errorf("invalid or unsafe private scratch recovery journal")
	}
	prefix := strings.SplitN(r.CandidateID, "-", 3)
	if len(prefix) != 3 || !strings.HasPrefix(filepath.Base(r.OutputPath), prefix[0]+"-"+prefix[1]+"-orphan-rehearsal-") {
		return fmt.Errorf("scratch journal refers to another stack")
	}
	switch r.State {
	case "running", "failed":
		if r.ScratchProofAtCompletion || r.RestoredFiles != 0 || r.ReportSHA256 != "" {
			return fmt.Errorf("unfinished journal cannot claim a completed scratch copy")
		}
	case "scratch_complete":
		if !r.ScratchProofAtCompletion || r.RestoredFiles < 2 || r.RestoredFiles > 16 ||
			!storageBackupSHA256.MatchString(r.ReportSHA256) {
			return fmt.Errorf("invalid scratch completion journal evidence")
		}
	default:
		return fmt.Errorf("unknown private scratch recovery journal state")
	}
	return nil
}

func missingOriginalJournalPath(directory string) string {
	return filepath.Join(directory, missingOriginalJournalFile)
}

// Read and validate a bounded, regular journal file. Missing means no recorded
// attempt, never successful recovery. A malformed or symlinked file fails closed.
func missingOriginalReadJournal(directory string) (*missingOriginalJournalRecord, error) {
	path := missingOriginalJournalPath(directory)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > missingOriginalJournalMax {
		return nil, fmt.Errorf("private recovery journal is unsafe or oversized")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("private recovery journal changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, missingOriginalJournalMax+1))
	if err != nil || len(data) > missingOriginalJournalMax {
		return nil, fmt.Errorf("failed to read bounded recovery journal")
	}
	final, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, final) || final.Size() != int64(len(data)) ||
		!info.ModTime().Equal(final.ModTime()) {
		return nil, fmt.Errorf("private recovery journal changed during read")
	}
	var r missingOriginalJournalRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("invalid recovery journal JSON: %w", err)
	}
	if err := missingOriginalValidateJournal(directory, r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Atomically publish a scratch-only state transition. Fsync the file and
// directory so a crash cannot make a partially written success look complete.
func missingOriginalWriteJournal(directory string, record missingOriginalJournalRecord) error {
	if err := missingOriginalValidateJournal(directory, record); err != nil {
		return err
	}
	if err := prepareStorageCandidateDirectory(directory); err != nil {
		return err
	}
	if _, err := missingOriginalReadJournal(directory); err != nil {
		return fmt.Errorf("refusing to replace unsafe recovery journal: %w", err)
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) > missingOriginalJournalMax {
		return fmt.Errorf("recovery journal cannot be serialized safely")
	}
	temp, err := os.CreateTemp(directory, ".missing-original-journal-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, missingOriginalJournalPath(directory)); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		d, err := os.Open(directory)
		if err != nil {
			return err
		}
		err = d.Sync()
		closeErr := d.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func missingOriginalJournalForDisplay(r *missingOriginalJournalRecord, currentBusy bool) *missingOriginalJournalStatus {
	if r == nil {
		return nil
	}
	state := r.State
	if state == "running" && !currentBusy {
		// Process-local job IDs do not survive restart. No auto-resume claim.
		state = "interrupted_or_unconfirmed"
	}
	return &missingOriginalJournalStatus{
		CandidateID: r.CandidateID, OutputPath: r.OutputPath, State: state,
		UpdatedUnix: r.UpdatedUnix, RestoredFiles: r.RestoredFiles,
		RequiresReaudit: true, CanActivate: false, EvictionEnabled: false,
	}
}

func missingOriginalNewJournal(candidate, snapshot, output string) missingOriginalJournalRecord {
	now := time.Now().UTC().Unix()
	return missingOriginalJournalRecord{
		Version: 1, CandidateID: candidate, Snapshot: snapshot,
		OutputPath: output, State: "running", StartedUnix: now, UpdatedUnix: now,
	}
}

func missingOriginalJournalFinish(r *missingOriginalJournalRecord, result json.RawMessage, restored int) {
	sum := sha256.Sum256(result)
	r.State = "scratch_complete"
	r.UpdatedUnix = time.Now().UTC().Unix()
	r.RestoredFiles = restored
	r.ScratchProofAtCompletion = true
	r.ReportSHA256 = hex.EncodeToString(sum[:])
}

func missingOriginalJournalFail(r *missingOriginalJournalRecord) {
	r.State = "failed"
	r.UpdatedUnix = time.Now().UTC().Unix()
	r.RestoredFiles = 0
	r.ReportSHA256 = ""
	r.ScratchProofAtCompletion = false
}
