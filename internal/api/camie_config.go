package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
)

const (
	camieConfigFilename        = "camie-metadata.json"
	defaultCamieFilenameLayout = "[%artist%](%copyright%).%character%_%md5%.%ext%"
)

var camieConfigMu sync.Mutex

type camieConfig struct {
	Threshold       float64 `json:"threshold"`
	Limit           int     `json:"limit"`
	FilenameEnabled bool    `json:"filenameEnabled"`
	FilenameLayout  string  `json:"filenameLayout"`
}

func defaultCamieConfig() camieConfig {
	return camieConfig{
		Threshold:       camietagger.DefaultThreshold,
		Limit:           camietagger.DefaultLimit,
		FilenameEnabled: true,
		FilenameLayout:  defaultCamieFilenameLayout,
	}
}

func camieConfigPath() string {
	return filepath.Join(manager.GetInstance().Config.GetConfigPath(), camieConfigFilename)
}

func loadCamieConfig() (camieConfig, error) {
	camieConfigMu.Lock()
	defer camieConfigMu.Unlock()
	return loadCamieConfigLocked()
}

func loadCamieConfigLocked() (camieConfig, error) {
	config := defaultCamieConfig()
	data, err := os.ReadFile(camieConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, fmt.Errorf("reading Camie metadata configuration: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("decoding Camie metadata configuration: %w", err)
	}
	if config.Threshold == 0 {
		config.Threshold = camietagger.DefaultThreshold
	}
	if config.Limit == 0 {
		config.Limit = camietagger.DefaultLimit
	}
	if strings.TrimSpace(config.FilenameLayout) == "" {
		config.FilenameLayout = defaultCamieFilenameLayout
	}
	if err := validateCamieConfig(config); err != nil {
		return defaultCamieConfig(), fmt.Errorf("invalid Camie metadata configuration: %w", err)
	}
	return config, nil
}

func validateCamieConfig(config camieConfig) error {
	if config.Threshold <= 0 || config.Threshold >= 1 {
		return fmt.Errorf("threshold must be greater than 0 and less than 1")
	}
	if config.Limit < 1 || config.Limit > camietagger.MaxLimit {
		return fmt.Errorf("per-category limit must be between 1 and %d", camietagger.MaxLimit)
	}
	if config.FilenameEnabled {
		if _, err := compileCamieFilenameLayout(config.FilenameLayout); err != nil {
			return err
		}
	}
	return nil
}

func saveCamieConfig(config camieConfig) error {
	if err := validateCamieConfig(config); err != nil {
		return err
	}

	camieConfigMu.Lock()
	defer camieConfigMu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding Camie metadata configuration: %w", err)
	}
	data = append(data, '\n')

	path := camieConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating Camie configuration directory: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".camie-metadata-*.tmp")
	if err != nil {
		return fmt.Errorf("creating Camie configuration temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("setting Camie configuration permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("writing Camie configuration: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing Camie configuration: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("installing Camie configuration: %w", err)
	}
	return nil
}

func updateCamieInferenceDefaults(threshold float64, limit int) error {
	config, err := loadCamieConfig()
	if err != nil {
		return err
	}
	config.Threshold = threshold
	config.Limit = limit
	return saveCamieConfig(config)
}

func (rs imageRoutes) CamieConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, config)
}

func (rs imageRoutes) CamieConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var config camieConfig
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		http.Error(w, fmt.Sprintf("decoding Camie metadata configuration: %v", err), http.StatusBadRequest)
		return
	}
	config.FilenameLayout = strings.TrimSpace(config.FilenameLayout)
	if err := saveCamieConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeVisualSimilarityJSON(w, config)
}
