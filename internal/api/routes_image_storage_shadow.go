package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/manager"
)

const storageShadowMaxImageBytes = 64 * 1024 * 1024

// One expensive comparison at a time; this never intercepts native /image.
var storageShadowSlot = make(chan struct{}, 1)

type storageShadowRequest struct {
	StackID     int
	ImageID     int
	CandidateID string
}

func storageShadowParseRequest(r *http.Request) (storageShadowRequest, error) {
	query := r.URL.Query()
	if len(query) != 3 || len(query["stackID"]) != 1 ||
		len(query["imageID"]) != 1 || len(query["candidateID"]) != 1 {
		return storageShadowRequest{}, fmt.Errorf("expected exactly stackID, imageID and candidateID")
	}
	stackID, err := strconv.Atoi(query.Get("stackID"))
	if err != nil || stackID < 1 {
		return storageShadowRequest{}, fmt.Errorf("invalid Visual Stack ID")
	}
	imageID, err := strconv.Atoi(query.Get("imageID"))
	if err != nil || imageID < 1 {
		return storageShadowRequest{}, fmt.Errorf("invalid image ID")
	}
	candidateID := query.Get("candidateID")
	if !storageCandidateIDPattern.MatchString(candidateID) {
		return storageShadowRequest{}, fmt.Errorf("invalid saved candidate ID")
	}
	return storageShadowRequest{StackID: stackID, ImageID: imageID, CandidateID: candidateID}, nil
}

func storageShadowMember(group estimateSourceGroup, identity string) (estimateSourceMember, error) {
	for _, member := range group.Members {
		if member.ID == identity {
			if err := estimateRegularSource(member.Path); err != nil {
				return estimateSourceMember{}, err
			}
			return member, nil
		}
	}
	return estimateSourceMember{}, fmt.Errorf("selected image is not in this Visual Stack")
}

func storageShadowScriptPath() string {
	if value := strings.TrimSpace(os.Getenv("STASH_IMAGE_SHADOW_MEMBER_SCRIPT")); value != "" {
		return value
	}
	return filepath.FromSlash("scripts/image_storage_shadow_member.py")
}

func storageShadowRun(ctx context.Context, method, archive, identity, hash, original string) ([]byte, error) {
	childCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, mediaBackupPython(), storageShadowScriptPath(),
		"--archive", archive, "--method", method, "--member", identity,
		"--sha256", hash, "--original", original)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, storageShadowMaxImageBytes+1))
	if readErr != nil || len(data) > storageShadowMaxImageBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("shadow output unavailable or larger than 64 MiB")
	}
	if err := cmd.Wait(); err != nil {
		msg := stderr.String()
		if len(msg) > 512 {
			msg = msg[:512]
		}
		return nil, fmt.Errorf("shadow reconstruction refused: %w: %s", err, msg)
	}
	if len(data) == 0 || childCtx.Err() != nil {
		return nil, fmt.Errorf("shadow result is empty or timed out")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("shadow bytes differ from original SHA-256")
	}
	return data, nil
}

func storageShadowContentType(data []byte) (string, bool) {
	detected := http.DetectContentType(data)
	switch detected {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return detected, true
	default:
		return "application/octet-stream", false
	}
}

// GET delivers a verified member from a saved candidate, not native media.
// It never activates archives, mutates catalogue data or deletes originals.
func (rs imageRoutes) StorageShadowImage(w http.ResponseWriter, r *http.Request) {
	req, err := storageShadowParseRequest(r)
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
	ctx := r.Context()
	group, err := estimateStackSources(ctx, req.StackID)
	if err != nil {
		http.Error(w, "Visual Stack source validation failed", http.StatusConflict)
		return
	}
	identity := fmt.Sprintf("image:%d", req.ImageID)
	member, err := storageShadowMember(group, identity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, "media backup not configured", http.StatusConflict)
		return
	}
	roots := configuredBackupRoots()
	store, err := validateBackupDestination(cfg.Destination, roots)
	if err != nil {
		http.Error(w, "backup path cannot be validated", http.StatusConflict)
		return
	}
	directory, err := storageCandidateDirectory(manager.GetInstance().Config.GetConfigPath(), store, roots)
	if err != nil {
		http.Error(w, "private candidate directory is unsafe", http.StatusConflict)
		return
	}
	receiptJSON, summary, err := storageCandidateReadReceipt(directory, req.CandidateID, true)
	if err != nil || summary.Group != group.ID {
		http.Error(w, "candidate missing, changed or mismatched", http.StatusConflict)
		return
	}
	var candidate storageCandidateProof
	if err := json.Unmarshal(receiptJSON, &candidate); err != nil {
		http.Error(w, "invalid candidate proof", http.StatusConflict)
		return
	}
	archive := filepath.Join(directory, req.CandidateID+".pack")
	if _, err := validateStorageCandidateProof(receiptJSON, group, summary.Snapshot, summary.Method, archive); err != nil {
		http.Error(w, "candidate proof no longer matches original catalogue", http.StatusConflict)
		return
	}
	digest, exists := candidate.SourceSHA256[identity]
	if !exists || !storageBackupSHA256.MatchString(digest) {
		http.Error(w, "selected image not in candidate proof", http.StatusConflict)
		return
	}
	if info, err := os.Lstat(storageShadowScriptPath()); err != nil || !info.Mode().IsRegular() {
		http.Error(w, "shadow proof script is unavailable", http.StatusServiceUnavailable)
		return
	}
	data, err := storageShadowRun(ctx, candidate.Method, archive, identity, digest, member.Path)
	if err != nil {
		http.Error(w, "shadow image could not be reconstructed byte-exactly", http.StatusConflict)
		return
	}
	serveVerifiedStorageShadowBytes(w, r, data)
}

// Shared only by the isolated shadow endpoint and response-contract tests.
// Never call this with unverified bytes from an archive.
func serveVerifiedStorageShadowBytes(w http.ResponseWriter, r *http.Request, data []byte) {
	kind, inline := storageShadowContentType(data)
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if !inline {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}
