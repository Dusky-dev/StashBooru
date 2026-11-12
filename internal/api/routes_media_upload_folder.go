package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type mediaUploadFolderInput struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
}

func validMediaUploadFolderName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 180 ||
		strings.TrimSpace(name) != name || strings.HasPrefix(name, ".") ||
		filepath.Base(name) != name || strings.ContainsAny(name, "/\\:") {
		return false
	}
	for _, char := range name {
		if char < 32 || char == 127 {
			return false
		}
	}
	return true
}

func (rs booruBrowserRoutes) CreateUploadFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !mediaUploadSameOrigin(r) {
		writeBooruError(w, http.StatusForbidden, "origin", "Folder changes must come from StashBooru.")
		return
	}
	if !rs.requireMediaUpload(w, "folder") {
		return
	}
	var input mediaUploadFolderInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input); err != nil ||
		!validMediaUploadFolderName(input.Name) {
		writeBooruError(w, http.StatusBadRequest, "folder", "Enter a valid single folder name.")
		return
	}
	parent, err := rs.destination(input.Parent)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "directory", err.Error())
		return
	}
	target := filepath.Join(parent, input.Name)
	if err := os.Mkdir(target, 0755); err != nil {
		writeBooruError(w, http.StatusConflict, "folder", "Cannot create folder (it may already exist or the library may be read-only).")
		return
	}
	// Resolve again after mkdir to ensure a new symlink cannot escape roots.
	result, err := rs.destination(target)
	if err != nil {
		_ = os.Remove(target)
		writeBooruError(w, http.StatusBadRequest, "directory", "Created folder is outside this library.")
		return
	}
	writeVisualSimilarityJSON(w, map[string]string{"path": result})
}
