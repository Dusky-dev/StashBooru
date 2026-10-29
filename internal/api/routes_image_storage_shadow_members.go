package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
)

// The guided comparison selector lists only IDs from a verified saved
// candidate and the current Visual Stack. It never exposes local paths,
// reconstructs a file or modifies native image routes.
type storageShadowMemberOption struct {
	ImageID int `json:"imageID"`
}

type storageShadowMembersReport struct {
	ReadOnly                bool                        `json:"readOnly"`
	CandidateID             string                      `json:"candidateID"`
	StackID                 int                         `json:"stackID"`
	Members                 []storageShadowMemberOption `json:"members"`
	SourcesRetained         bool                        `json:"sourcesRetained"`
	CanActivate             bool                        `json:"canActivate"`
	OriginalEvictionEnabled bool                        `json:"originalEvictionEnabled"`
}

type storageShadowMembersRequest struct {
	StackID     int
	CandidateID string
}

func storageShadowParseMembersRequest(r *http.Request) (storageShadowMembersRequest, error) {
	query := r.URL.Query()
	if len(query) != 2 || len(query["stackID"]) != 1 || len(query["candidateID"]) != 1 {
		return storageShadowMembersRequest{}, fmt.Errorf("expected exactly stackID and candidateID")
	}
	stackID, err := strconv.Atoi(query.Get("stackID"))
	if err != nil || stackID < 1 || !storageCandidateIDPattern.MatchString(query.Get("candidateID")) {
		return storageShadowMembersRequest{}, fmt.Errorf("invalid Visual Stack or saved candidate ID")
	}
	return storageShadowMembersRequest{StackID: stackID, CandidateID: query.Get("candidateID")}, nil
}

func storageShadowMemberOptions(group estimateSourceGroup, proof storageCandidateProof) ([]storageShadowMemberOption, error) {
	if proof.Group != group.ID || proof.Fidelity != "byte-exact" ||
		proof.Method != "delta" && proof.Method != "shared-tiles" ||
		!proof.Verified || !proof.BackupVerified || !proof.SourcesRetained ||
		!proof.NativePackingCandidate || proof.NativeServing || proof.NativeServingVerified ||
		proof.DatabaseBackupVerified || proof.OriginalEviction || proof.ArchiveActivated ||
		len(group.Members) < 2 || len(group.Members) > 16 ||
		proof.SourceFiles != len(group.Members) || len(proof.SourceSHA256) != len(group.Members) {
		return nil, fmt.Errorf("candidate has missing or unsafe read-only proof")
	}
	options := make([]storageShadowMemberOption, 0, len(group.Members))
	seen := make(map[int]bool, len(group.Members))
	for _, member := range group.Members {
		if len(member.ID) < len("image:")+1 || member.ID[:len("image:")] != "image:" {
			return nil, fmt.Errorf("candidate contains an invalid native image identity")
		}
		id, err := strconv.Atoi(member.ID[len("image:"):])
		digest, found := proof.SourceSHA256[member.ID]
		if err != nil || id <= 0 || strconv.Itoa(id) != member.ID[len("image:"):] ||
			seen[id] || !found || !storageBackupSHA256.MatchString(digest) {
			return nil, fmt.Errorf("candidate member cannot be selected safely")
		}
		seen[id] = true
		options = append(options, storageShadowMemberOption{ImageID: id})
	}
	return options, nil
}

func (rs imageRoutes) StorageShadowMembers(w http.ResponseWriter, r *http.Request) {
	req, err := storageShadowParseMembersRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	select {
	case storageShadowSlot <- struct{}{}:
		defer func() { <-storageShadowSlot }()
	default:
		http.Error(w, "another shadow verification is already running", http.StatusTooManyRequests)
		return
	}
	group, err := estimateStackSources(r.Context(), req.StackID)
	if err != nil {
		http.Error(w, "Visual Stack source validation failed", http.StatusConflict)
		return
	}
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, "original-media backup not configured", http.StatusConflict)
		return
	}
	roots := configuredBackupRoots()
	store, err := validateBackupDestination(cfg.Destination, roots)
	if err != nil {
		http.Error(w, "backup store cannot be validated", http.StatusConflict)
		return
	}
	directory, err := storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, roots)
	if err != nil {
		http.Error(w, "private candidate directory is unsafe", http.StatusConflict)
		return
	}
	data, summary, err := storageCandidateReadReceipt(directory, req.CandidateID, true)
	if err != nil || summary.Group != group.ID {
		http.Error(w, "candidate missing, changed or mismatched", http.StatusConflict)
		return
	}
	archive := filepath.Join(directory, req.CandidateID+".pack")
	if _, err := validateStorageCandidateProof(data, group, summary.Snapshot, summary.Method, archive); err != nil {
		http.Error(w, "candidate proof no longer matches current catalogue", http.StatusConflict)
		return
	}
	var proof storageCandidateProof
	if err := json.Unmarshal(data, &proof); err != nil {
		http.Error(w, "invalid saved candidate proof", http.StatusConflict)
		return
	}
	members, err := storageShadowMemberOptions(group, proof)
	if err != nil {
		http.Error(w, "saved candidate membership is not safe", http.StatusConflict)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeVisualSimilarityJSON(w, storageShadowMembersReport{
		ReadOnly: true, CandidateID: req.CandidateID, StackID: req.StackID,
		Members: members, SourcesRetained: true,
		CanActivate: false, OriginalEvictionEnabled: false,
	})
}
