package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager"
)

const shadowSelectionFile = "shadow-serving-selection.json"
const shadowSelectionLimit = 4096

var shadowSelectionMutex sync.Mutex

// A persisted, inspectable trial intent, NOT catalogue activation.
// The default /image/{id} routes never consult this record.
type shadowSelectionRecord struct {
	Version int
	State string
	CandidateID string
	StackID int
	Snapshot string
	Method string
	ReceiptSHA256 string
	ArchiveSHA256 string
	Revision uint64
	UpdatedUnix int64
	OriginalsAuthoritative bool
	NativeRoutingEnabled bool
	CanActivate bool
	OriginalEvictionEnabled bool
	TransactionalRollbackVerified bool
}

type shadowSelectionStatus struct {
	State string
	CandidateID string
	StackID int
	Revision uint64
	EvidenceVerifiedNow bool
	RequiresFreshByteCheck bool
	OriginalsAuthoritative bool
	NativeRoutingEnabled bool
	CanActivate bool
	OriginalEvictionEnabled bool
	TransactionalRollbackVerified bool
}

type shadowSelectionRequest struct {
	Action string
	CandidateID string
	StackID int
	Acknowledged bool
}

func shadowSelectionValidate(record shadowSelectionRecord) error {
	if record.Version != 1 || (record.State != "prepared" && record.State != "disarmed") ||
		!storageCandidateIDPattern.MatchString(record.CandidateID) ||
		record.StackID <= 0 || !mediaBackupSnapshotID.MatchString(record.Snapshot) ||
		(record.Method != "delta" && record.Method != "shared-tiles") ||
		!storageBackupSHA256.MatchString(record.ReceiptSHA256) ||
		!storageBackupSHA256.MatchString(record.ArchiveSHA256) ||
		record.Revision == 0 || record.Revision > 1000000 ||
		record.UpdatedUnix <= 0 || record.UpdatedUnix > time.Now().Add(24*time.Hour).Unix() ||
		!record.OriginalsAuthoritative || record.NativeRoutingEnabled || record.CanActivate ||
		record.OriginalEvictionEnabled || record.TransactionalRollbackVerified {
		return fmt.Errorf("invalid or unsafe shadow serving selection")
	}
	if !strings.HasPrefix(record.CandidateID, fmt.Sprintf("stack-%d-%s-", record.StackID, record.Method)) {
		return fmt.Errorf("shadow candidate does not match the selected stack and method")
	}
	return nil
}

func shadowSelectionPath(directory string) string {
	return filepath.Join(directory, shadowSelectionFile)
}

// Corrupt, symlinked, changed, or oversized state must fail closed, not
// silently reset a stored selection or present it as safe.
func shadowSelectionRead(directory string) (*shadowSelectionRecord, error) {
	path := shadowSelectionPath(directory)
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > shadowSelectionLimit {
		return nil, fmt.Errorf("unsafe or oversized shadow selection")
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("shadow selection changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(input, shadowSelectionLimit+1))
	if err != nil || int64(len(data)) != before.Size() {
		return nil, fmt.Errorf("shadow selection changed while reading")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("shadow selection changed during read")
	}
	var record shadowSelectionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	if err := shadowSelectionValidate(record); err != nil {
		return nil, err
	}
	return &record, nil
}

// Caller holds shadowSelectionMutex. The temporary file is mode 0600,
// fsynced, then atomically renamed and the directory fsynced. No other
// StashBooru process is synchronized by this process-local lock.
func shadowSelectionWrite(directory string, record shadowSelectionRecord) error {
	if err := shadowSelectionValidate(record); err != nil {
		return err
	}
	if err := prepareStorageCandidateDirectory(directory); err != nil {
		return err
	}
	old, err := shadowSelectionRead(directory)
	if err != nil {
		return fmt.Errorf("refusing to replace unsafe selection: %w", err)
	}
	if old == nil && record.Revision != 1 || old != nil && record.Revision != old.Revision+1 {
		return fmt.Errorf("invalid shadow selection revision")
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) > shadowSelectionLimit {
		return fmt.Errorf("private selection payload too large")
	}
	tmp, err := os.CreateTemp(directory, ".shadow-selection-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), shadowSelectionPath(directory)); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		dir, err := os.Open(directory)
		if err != nil {
			return err
		}
		err = dir.Sync()
		closeErr := dir.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return nil
}

func shadowSelectionStatusFor(r *shadowSelectionRecord, verified bool) shadowSelectionStatus {
	out := shadowSelectionStatus{State: "not_prepared", OriginalsAuthoritative: true}
	if r != nil {
		out.State = r.State
		out.CandidateID = r.CandidateID
		out.StackID = r.StackID
		out.Revision = r.Revision
		out.EvidenceVerifiedNow = verified && r.State == "prepared"
		out.RequiresFreshByteCheck = r.State == "prepared"
	}
	return out
}

func shadowSelectionCandidateEvidence(directory, candidateID string) (string, string, error) {
	receiptPath, err := storageCandidateReceiptPath(directory, candidateID)
	if err != nil {
		return "", "", err
	}
	info, err := os.Lstat(receiptPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > storageCandidateReceiptLimit {
		return "", "", fmt.Errorf("candidate receipt not a bounded regular file")
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil || int64(len(data)) != info.Size() {
		return "", "", fmt.Errorf("cannot inspect exact candidate receipt")
	}
	hash := sha256.Sum256(data)
	archiveHash, err := storageCandidateArchiveDigest(filepath.Join(directory, candidateID+".pack"))
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(hash[:]), archiveHash, nil
}

func shadowSelectionCheckEvidence(r *http.Request, directory string, record shadowSelectionRecord) error {
	raw, summary, err := storageCandidateReadReceipt(directory, record.CandidateID, true)
	if err != nil || summary.Group != "stack:"+strconv.Itoa(record.StackID) ||
		summary.Snapshot != record.Snapshot || summary.Method != record.Method {
		return fmt.Errorf("candidate proof missing or mismatched")
	}
	receiptHash, archiveHash, err := shadowSelectionCandidateEvidence(directory, record.CandidateID)
	if err != nil || receiptHash != record.ReceiptSHA256 || archiveHash != record.ArchiveSHA256 {
		return fmt.Errorf("candidate evidence differs from selection")
	}
	group, err := estimateStackSources(r.Context(), record.StackID)
	if err != nil {
		return fmt.Errorf("current Visual Stack no longer matches prepared intent")
	}
	if _, err := validateStorageCandidateProof(raw, group, record.Snapshot, record.Method,
		filepath.Join(directory, record.CandidateID+".pack")); err != nil {
		return err
	}
	var proof storageCandidateProof
	if err := json.Unmarshal(raw, &proof); err != nil {
		return err
	}
	_, err = storageShadowMemberOptions(group, proof)
	return err
}

func shadowSelectionDirectory() (string, error) {
	cfg, err := readMediaBackupConfig()
	if err != nil {
		return "", err
	}
	roots := configuredBackupRoots()
	store, err := validateBackupDestination(cfg.Destination, roots)
	if err != nil {
		return "", err
	}
	return storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, roots)
}

func (rs imageRoutes) StorageShadowSelectionStatus(w http.ResponseWriter, r *http.Request) {
	shadowSelectionMutex.Lock()
	defer shadowSelectionMutex.Unlock()
	dir, err := shadowSelectionDirectory()
	if err != nil {
		writeVisualSimilarityJSON(w, shadowSelectionStatusFor(nil, false))
		return
	}
	record, err := shadowSelectionRead(dir)
	if err != nil {
		http.Error(w, "unsafe private shadow selection", http.StatusConflict)
		return
	}
	verified := record != nil && record.State == "prepared" &&
		shadowSelectionCheckEvidence(r, dir, *record) == nil
	writeVisualSimilarityJSON(w, shadowSelectionStatusFor(record, verified))
}

func (rs imageRoutes) StorageShadowSelectionAction(w http.ResponseWriter, r *http.Request) {
	var req shadowSelectionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid shadow selection request", http.StatusBadRequest)
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		http.Error(w, "unexpected shadow selection request data", http.StatusBadRequest)
		return
	}
	if (req.Action != "prepare" && req.Action != "disarm") || !req.Acknowledged {
		http.Error(w, "explicit prepare or disarm acknowledgement required", http.StatusBadRequest)
		return
	}
	shadowSelectionMutex.Lock()
	defer shadowSelectionMutex.Unlock()
	dir, err := shadowSelectionDirectory()
	if err != nil {
		http.Error(w, "candidate directory unavailable", http.StatusConflict)
		return
	}
	current, err := shadowSelectionRead(dir)
	if err != nil {
		http.Error(w, "unsafe existing selection cannot be overwritten", http.StatusConflict)
		return
	}
	if req.Action == "disarm" {
		if req.CandidateID != "" || req.StackID != 0 || current == nil {
			http.Error(w, "disarm requires only an existing selection", http.StatusBadRequest)
			return
		}
		record := *current
		record.State = "disarmed"
		record.Revision++
		record.UpdatedUnix = time.Now().Unix()
		if err := shadowSelectionWrite(dir, record); err != nil {
			http.Error(w, "could not persist disarmed state", http.StatusConflict)
			return
		}
		writeVisualSimilarityJSON(w, shadowSelectionStatusFor(&record, false))
		return
	}
	if req.StackID < 1 || !storageCandidateIDPattern.MatchString(req.CandidateID) {
		http.Error(w, "valid saved candidate and stack required", http.StatusBadRequest)
		return
	}
	raw, summary, err := storageCandidateReadReceipt(dir, req.CandidateID, true)
	if err != nil || summary.Group != "stack:"+strconv.Itoa(req.StackID) {
		http.Error(w, "saved candidate does not match selected stack", http.StatusConflict)
		return
	}
	group, err := estimateStackSources(r.Context(), req.StackID)
	if err != nil {
		http.Error(w, "cannot verify current Visual Stack", http.StatusConflict)
		return
	}
	if _, err := validateStorageCandidateProof(raw, group, summary.Snapshot, summary.Method,
		filepath.Join(dir, req.CandidateID+".pack")); err != nil {
		http.Error(w, "candidate catalogue evidence mismatch", http.StatusConflict)
		return
	}
	var proof storageCandidateProof
	if err := json.Unmarshal(raw, &proof); err != nil {
		http.Error(w, "candidate proof cannot be decoded", http.StatusConflict)
		return
	}
	if _, err := storageShadowMemberOptions(group, proof); err != nil {
		http.Error(w, "candidate members cannot be trusted", http.StatusConflict)
		return
	}
	receiptHash, archiveHash, err := shadowSelectionCandidateEvidence(dir, req.CandidateID)
	if err != nil {
		http.Error(w, "candidate checksum evidence unavailable", http.StatusConflict)
		return
	}
	record := shadowSelectionRecord{
		Version: 1, State: "prepared", CandidateID: req.CandidateID,
		StackID: req.StackID, Snapshot: summary.Snapshot, Method: summary.Method,
		ReceiptSHA256: receiptHash, ArchiveSHA256: archiveHash,
		Revision: 1, UpdatedUnix: time.Now().Unix(), OriginalsAuthoritative: true,
	}
	if current != nil {
		record.Revision = current.Revision+1
	}
	if err := shadowSelectionWrite(dir, record); err != nil {
		http.Error(w, "cannot publish private shadow selection", http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, shadowSelectionStatusFor(&record, true))
}
