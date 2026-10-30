package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// An explicit, transient verification of the currently prepared shadow intent.
// This is NOT activation, nor a proof that database/config restore or rollback
// works. All member reads compare byte-exactly with the retained originals.
type shadowPreflightRequest struct {
	Acknowledged bool
}

type shadowPreflightReport struct {
	ReadOnly                      bool
	CandidateID                   string
	StackID                       int
	Revision                      uint64
	MediaBackupVerified           bool
	ArchiveSHA256Verified         bool
	AllOriginalBytesVerified      bool
	MembersVerified               int
	OriginalsAuthoritative        bool
	NativeRoutingEnabled          bool
	CanActivate                   bool
	OriginalEvictionEnabled       bool
	TransactionalRollbackVerified bool
	Blockers                      []string
}

func shadowPreflightStable(before, after *shadowSelectionRecord) bool {
	return before != nil && after != nil && before.State == "prepared" &&
		after.State == "prepared" && *before == *after
}

func makeShadowPreflightReport(record shadowSelectionRecord, members int) (shadowPreflightReport, error) {
	if err := shadowSelectionValidate(record); err != nil || record.State != "prepared" ||
		members < 2 || members > 16 {
		return shadowPreflightReport{}, fmt.Errorf("shadow preflight requires a valid prepared selection with 2–16 members")
	}
	return shadowPreflightReport{
		ReadOnly:                 true,
		CandidateID:              record.CandidateID,
		StackID:                  record.StackID,
		Revision:                 record.Revision,
		MediaBackupVerified:      true,
		ArchiveSHA256Verified:    true,
		AllOriginalBytesVerified: true,
		MembersVerified:          members,
		OriginalsAuthoritative:   true,
		Blockers: []string{
			"Verification is point-in-time and is not a durable activation authorization",
			"Coherent database/configuration recovery and crash-safe rollback are not proven",
			"Automatic native packed serving and original-file eviction remain disabled",
		},
	}, nil
}

// POST /image/storage-shadow-preflight is intentionally explicit and expensive.
// No caller can supply an archive path, image ID or alternate selection.
// A prepared intent remains non-activating even after this succeeds.
func (rs imageRoutes) StorageShadowPreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request shadowPreflightRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&request); err != nil || !request.Acknowledged {
		http.Error(w, "explicit read-only preflight acknowledgement required", http.StatusBadRequest)
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || len(r.URL.Query()) != 0 {
		http.Error(w, "unexpected preflight parameters", http.StatusBadRequest)
		return
	}
	select {
	case storageShadowSlot <- struct{}{}:
		defer func() { <-storageShadowSlot }()
	default:
		http.Error(w, "another packed-image check is running", http.StatusTooManyRequests)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	directory, err := shadowSelectionDirectory()
	if err != nil {
		http.Error(w, "saved candidate directory unavailable", http.StatusConflict)
		return
	}
	shadowSelectionMutex.Lock()
	initial, err := shadowSelectionRead(directory)
	shadowSelectionMutex.Unlock()
	if err != nil || initial == nil || initial.State != "prepared" {
		http.Error(w, "no safe prepared selection", http.StatusConflict)
		return
	}
	if err := shadowSelectionCheckEvidence(r, directory, *initial); err != nil {
		http.Error(w, "prepared selection evidence changed", http.StatusConflict)
		return
	}

	group, err := estimateStackSources(ctx, initial.StackID)
	if err != nil {
		http.Error(w, "Visual Stack is unavailable", http.StatusConflict)
		return
	}
	raw, summary, err := storageCandidateReadReceipt(directory, initial.CandidateID, true)
	if err != nil || summary.Group != group.ID || summary.Method != initial.Method ||
		summary.Snapshot != initial.Snapshot {
		http.Error(w, "saved candidate does not match intent", http.StatusConflict)
		return
	}
	archive := filepath.Join(directory, initial.CandidateID+".pack")
	if _, err := validateStorageCandidateProof(raw, group, summary.Snapshot, summary.Method, archive); err != nil {
		http.Error(w, "candidate proof is stale", http.StatusConflict)
		return
	}
	var candidate storageCandidateProof
	if err := json.Unmarshal(raw, &candidate); err != nil {
		http.Error(w, "invalid saved candidate", http.StatusConflict)
		return
	}
	members, err := storageShadowMemberOptions(group, candidate)
	if err != nil {
		http.Error(w, "candidate members are invalid", http.StatusConflict)
		return
	}
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, "original-media backup not configured", http.StatusConflict)
		return
	}
	store, err := validateBackupDestination(cfg.Destination, configuredBackupRoots())
	if err != nil {
		http.Error(w, "original-media backup unavailable", http.StatusConflict)
		return
	}
	backupJSON, err := runStorageBackupPreflight(ctx, group, store, summary.Snapshot)
	if err != nil {
		http.Error(w, "original-media backup verification failed", http.StatusConflict)
		return
	}
	var backup storageBackupPreflightReport
	if err := json.Unmarshal(backupJSON, &backup); err != nil {
		http.Error(w, "invalid backup verification result", http.StatusConflict)
		return
	}
	if _, err := buildStorageRestorePlan(group, initial.CandidateID, candidate, backup); err != nil {
		http.Error(w, "archive and backup source hashes disagree", http.StatusConflict)
		return
	}
	if info, err := os.Lstat(storageShadowScriptPath()); err != nil || !info.Mode().IsRegular() {
		http.Error(w, "byte-exact reconstruction script unavailable", http.StatusServiceUnavailable)
		return
	}
	for _, option := range members {
		identity := fmt.Sprintf("image:%d", option.ImageID)
		member, err := storageShadowMember(group, identity)
		if err != nil {
			http.Error(w, "image source changed during preflight", http.StatusConflict)
			return
		}
		digest, found := candidate.SourceSHA256[identity]
		if !found {
			http.Error(w, "image missing trusted source hash", http.StatusConflict)
			return
		}
		if _, err := storageShadowRun(ctx, candidate.Method, archive, identity, digest, member.Path); err != nil {
			http.Error(w, "archived bytes do not match retained original", http.StatusConflict)
			return
		}
	}
	// Disarm/prepare can occur while a long preflight is running. Re-read and
	// revalidate all catalogue and archive evidence before reporting success.
	latestDirectory, err := shadowSelectionDirectory()
	if err != nil || latestDirectory != directory {
		http.Error(w, "candidate directory changed during preflight", http.StatusConflict)
		return
	}
	shadowSelectionMutex.Lock()
	latest, readErr := shadowSelectionRead(directory)
	shadowSelectionMutex.Unlock()
	if readErr != nil || !shadowPreflightStable(initial, latest) ||
		shadowSelectionCheckEvidence(r, directory, *initial) != nil {
		http.Error(w, "prepared selection changed during preflight", http.StatusConflict)
		return
	}
	report, err := makeShadowPreflightReport(*initial, len(members))
	if err != nil {
		http.Error(w, "preflight verification could not be summarized", http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, report)
}
