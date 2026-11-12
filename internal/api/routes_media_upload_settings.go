package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Uploads are opt-in while experimental. These settings are server-side,
// separate from local browser preferences, and enforced on all write endpoints.
type mediaUploadSettings struct {
	Enabled              bool   `json:"enabled"`
	AllowURL             bool   `json:"allowURL"`
	AllowFolderCreation  bool   `json:"allowFolderCreation"`
	DefaultDirectory     string `json:"defaultDirectory"`
	QuickUpload          bool   `json:"quickUpload"`
	QuickTagDefaults     bool   `json:"quickTagDefaults"`
	QuickConvertDefaults bool   `json:"quickConvertDefaults"`
}

var mediaUploadSettingsMu sync.Mutex

func defaultMediaUploadSettings() mediaUploadSettings {
	return mediaUploadSettings{
		Enabled:             false,
		AllowURL:            true,
		AllowFolderCreation: true,
	}
}

func (rs booruBrowserRoutes) readMediaUploadSettings() (mediaUploadSettings, error) {
	defaults := defaultMediaUploadSettings()
	if rs.configPath == "" {
		return defaults, nil
	}
	file, err := os.Open(filepath.Join(rs.configPath, "media-upload.json"))
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, fmt.Errorf("cannot read experimental upload settings: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(data) > 8192 {
		return defaults, fmt.Errorf("experimental upload settings are unreadable or too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var saved mediaUploadSettings
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF {
		return defaults, fmt.Errorf("experimental upload settings are invalid")
	}
	return saved, nil
}

func (rs booruBrowserRoutes) saveMediaUploadSettings(settings mediaUploadSettings) error {
	if rs.configPath == "" {
		return fmt.Errorf("server config directory not configured")
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(rs.configPath, ".media-upload-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	_, err = temp.Write(data)
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp.Name(), filepath.Join(rs.configPath, "media-upload.json"))
}

func (rs booruBrowserRoutes) GetMediaUploadSettings(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	settings, err := rs.readMediaUploadSettings()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "settings", err.Error())
		return
	}
	writeVisualSimilarityJSON(w, settings)
}

func (rs booruBrowserRoutes) UpdateMediaUploadSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !mediaUploadSameOrigin(r) {
		writeBooruError(w, http.StatusForbidden, "origin", "Experimental settings must be changed from StashBooru.")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	var input mediaUploadSettings
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		writeBooruError(w, http.StatusBadRequest, "settings", "Invalid experimental media upload settings.")
		return
	}
	if input.DefaultDirectory != "" {
		resolved, err := rs.destination(input.DefaultDirectory)
		if err != nil {
			writeBooruError(w, http.StatusBadRequest, "directory", err.Error())
			return
		}
		input.DefaultDirectory = resolved
	}
	mediaUploadSettingsMu.Lock()
	err := rs.saveMediaUploadSettings(input)
	mediaUploadSettingsMu.Unlock()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "settings", "Cannot save experimental upload settings.")
		return
	}
	writeVisualSimilarityJSON(w, input)
}

func (rs booruBrowserRoutes) requireMediaUpload(w http.ResponseWriter, feature string) bool {
	settings, err := rs.readMediaUploadSettings()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "settings", "Cannot load experimental upload settings.")
		return false
	}
	if !settings.Enabled {
		writeBooruError(w, http.StatusForbidden, "experimental", "Enable media uploads in Settings > Experimental first.")
		return false
	}
	if (feature == "url" && !settings.AllowURL) ||
		(feature == "folder" && !settings.AllowFolderCreation) {
		writeBooruError(w, http.StatusForbidden, "experimental", "This upload option is disabled in Experimental settings.")
		return false
	}
	return true
}
