package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/stashapp/stash/pkg/models"
)

// A prepared, explicit, read-only canary of the NORMAL source file's bytes.
// It is a separate route, never a switch for /image/{id}/image. The original
// remains authoritative and is retained even when packed bytes are returned.
func shadowCanaryAllowed(record *shadowSelectionRecord, imageID int) bool {
	return imageID > 0 && record != nil && record.State == "prepared" &&
		shadowSelectionValidate(*record) == nil
}

// Reconstruct only the image ID from the native URL, from the single durable
// prepared selection. Never trust caller-provided paths or candidate IDs.
func shadowCanaryBytes(r *http.Request, directory string, selected shadowSelectionRecord, imageID int) ([]byte, error) {
	if err := shadowSelectionCheckEvidence(r, directory, selected); err != nil {
		return nil, err
	}
	group, err := estimateStackSources(r.Context(), selected.StackID)
	if err != nil {
		return nil, err
	}
	identity := fmt.Sprintf("image:%d", imageID)
	member, err := storageShadowMember(group, identity)
	if err != nil {
		return nil, err
	}
	raw, summary, err := storageCandidateReadReceipt(directory, selected.CandidateID, true)
	if err != nil {
		return nil, err
	}
	if summary.Group != group.ID || summary.Snapshot != selected.Snapshot ||
		summary.Method != selected.Method {
		return nil, fmt.Errorf("saved shadow receipt no longer matches prepared state")
	}
	archive := filepath.Join(directory, selected.CandidateID+".pack")
	if _, err := validateStorageCandidateProof(raw, group, summary.Snapshot, summary.Method, archive); err != nil {
		return nil, err
	}
	var candidate storageCandidateProof
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return nil, err
	}
	if _, err := storageShadowMemberOptions(group, candidate); err != nil {
		return nil, err
	}
	digest, ok := candidate.SourceSHA256[identity]
	if !ok || !storageBackupSHA256.MatchString(digest) {
		return nil, fmt.Errorf("image is absent from trusted shadow candidate")
	}
	if info, err := os.Lstat(storageShadowScriptPath()); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("shadow reader unavailable")
	}
	return storageShadowRun(r.Context(), candidate.Method, archive, identity, digest, member.Path)
}

// GET /image/{imageId}/storage-canary must always be explicitly requested.
// A missing/disarmed/unsafe selection fails closed. When a prepared candidate
// cannot reconstruct byte-exact media, ONLY the retained original is served.
// The fallback does not try a different archive or silently activate anything.
func (rs imageRoutes) StorageShadowCanary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	idString := chi.URLParam(r, "imageId")
	imageID, err := strconv.Atoi(idString)
	if err != nil || imageID <= 0 || strconv.Itoa(imageID) != idString || r.URL.RawQuery != "" {
		http.Error(w, "canonical image ID and no query parameters required", http.StatusBadRequest)
		return
	}
	image := r.Context().Value(imageKey).(*models.Image)
	dir, err := shadowSelectionDirectory()
	if err != nil {
		http.Error(w, "saved canary selection unavailable", http.StatusConflict)
		return
	}
	shadowSelectionMutex.Lock()
	selected, err := shadowSelectionRead(dir)
	shadowSelectionMutex.Unlock()
	if err != nil || !shadowCanaryAllowed(selected, imageID) {
		http.Error(w, "no safe prepared canary selection", http.StatusConflict)
		return
	}
	// This endpoint is for members of the prepared Visual Stack only.
	// A different image must not be mistaken for a failed archive fallback.
	group, err := estimateStackSources(r.Context(), selected.StackID)
	if err != nil {
		http.Error(w, "prepared Visual Stack unavailable", http.StatusConflict)
		return
	}
	if _, err := storageShadowMember(group, fmt.Sprintf("image:%d", imageID)); err != nil {
		http.Error(w, "image is not an available member of the prepared candidate", http.StatusConflict)
		return
	}
	fallback := func() {
		w.Header().Set("X-Stash-Shadow-Source", "original-fallback")
		rs.serveImage(w, r, image, false)
	}
	select {
	case storageShadowSlot <- struct{}{}:
		defer func() { <-storageShadowSlot }()
	default:
		fallback()
		return
	}
	data, err := shadowCanaryBytes(r, dir, *selected, imageID)
	if err != nil {
		fallback()
		return
	}
	// Refuse a prepared trial changed by disarm, refresh or external tamper
	// while byte reconstruction was in progress. No durable preflight grant.
	shadowSelectionMutex.Lock()
	latest, readErr := shadowSelectionRead(dir)
	shadowSelectionMutex.Unlock()
	if readErr != nil || !shadowPreflightStable(selected, latest) ||
		shadowSelectionCheckEvidence(r, dir, *selected) != nil {
		fallback()
		return
	}
	w.Header().Set("X-Stash-Shadow-Source", "packed-verified")
	serveVerifiedStorageShadowBytes(w, r, data)
}
