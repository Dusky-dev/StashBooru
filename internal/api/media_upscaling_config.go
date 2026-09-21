package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
)

const mediaUpscalingConfigFilename = "media-upscaling.json"

var mediaUpscalingConfigMu sync.Mutex
var mediaUpscalingEnvironment = newMediaUpscalingEnvironment()

type mediaUpscalingConfig struct {
	Waifu2xExecutable   string `json:"waifu2xExecutable"`
	Waifu2xModels       string `json:"waifu2xModels"`
	SeedVR2CLI          string `json:"seedVR2CLI"`
	SeedVR2Models       string `json:"seedVR2Models"`
	SeedVR2Model        string `json:"seedVR2Model"`
	SeedVR2Python       string `json:"seedVR2Python"`
	SeedVR2BlocksToSwap int    `json:"seedVR2BlocksToSwap"`
}

type environmentBaseline struct {
	value string
	set   bool
}

type mediaUpscalingEnvironmentState struct {
	baseline map[string]environmentBaseline
}

func newMediaUpscalingEnvironment() mediaUpscalingEnvironmentState {
	keys := []string{
		"STASH_WAIFU2X",
		"STASH_WAIFU2X_MODELS",
		"STASH_SEEDVR2_CLI",
		"STASH_SEEDVR2_MODELS",
		"STASH_SEEDVR2_MODEL",
		"STASH_SEEDVR2_PYTHON",
		"STASH_SEEDVR2_BLOCKS_TO_SWAP",
	}
	baseline := make(map[string]environmentBaseline, len(keys))
	for _, key := range keys {
		value, set := os.LookupEnv(key)
		baseline[key] = environmentBaseline{value: value, set: set}
	}
	return mediaUpscalingEnvironmentState{baseline: baseline}
}

func mediaUpscalingConfigPath() string {
	return filepath.Join(manager.GetInstance().Config.GetConfigPath(), mediaUpscalingConfigFilename)
}

func loadMediaUpscalingConfig() (mediaUpscalingConfig, error) {
	mediaUpscalingConfigMu.Lock()
	defer mediaUpscalingConfigMu.Unlock()

	var config mediaUpscalingConfig
	data, err := os.ReadFile(mediaUpscalingConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, fmt.Errorf("reading media upscaling configuration: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("decoding media upscaling configuration: %w", err)
	}
	config = normalizeMediaUpscalingConfig(config)
	if err := validateMediaUpscalingConfig(config); err != nil {
		return mediaUpscalingConfig{}, fmt.Errorf("validating media upscaling configuration: %w", err)
	}
	return config, nil
}

func normalizeMediaUpscalingConfig(config mediaUpscalingConfig) mediaUpscalingConfig {
	config.Waifu2xExecutable = strings.TrimSpace(config.Waifu2xExecutable)
	config.Waifu2xModels = strings.TrimSpace(config.Waifu2xModels)
	config.SeedVR2CLI = strings.TrimSpace(config.SeedVR2CLI)
	config.SeedVR2Models = strings.TrimSpace(config.SeedVR2Models)
	config.SeedVR2Model = strings.TrimSpace(config.SeedVR2Model)
	config.SeedVR2Python = strings.TrimSpace(config.SeedVR2Python)
	return config
}

func validateMediaUpscalingConfig(config mediaUpscalingConfig) error {
	if config.SeedVR2BlocksToSwap < 0 || config.SeedVR2BlocksToSwap > 128 {
		return fmt.Errorf("SeedVR2 blocks to swap must be between 0 and 128")
	}
	for name, value := range map[string]string{
		"waifu2x executable":       config.Waifu2xExecutable,
		"waifu2x model directory":  config.Waifu2xModels,
		"SeedVR2 CLI":              config.SeedVR2CLI,
		"SeedVR2 model directory":  config.SeedVR2Models,
		"SeedVR2 model":            config.SeedVR2Model,
		"SeedVR2 Python executable": config.SeedVR2Python,
	} {
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%s contains an invalid NUL byte", name)
		}
	}
	return nil
}

func saveMediaUpscalingConfig(config mediaUpscalingConfig) error {
	config = normalizeMediaUpscalingConfig(config)
	if err := validateMediaUpscalingConfig(config); err != nil {
		return err
	}

	mediaUpscalingConfigMu.Lock()
	defer mediaUpscalingConfigMu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding media upscaling configuration: %w", err)
	}
	data = append(data, '\n')
	path := mediaUpscalingConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating media upscaling configuration directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".media-upscaling-*.tmp")
	if err != nil {
		return fmt.Errorf("creating media upscaling temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("setting media upscaling configuration permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("writing media upscaling configuration: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing media upscaling configuration: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("installing media upscaling configuration: %w", err)
	}
	return nil
}

func restoreMediaUpscalingEnvironment(key string) {
	baseline := mediaUpscalingEnvironment.baseline[key]
	if baseline.set {
		_ = os.Setenv(key, baseline.value)
	} else {
		_ = os.Unsetenv(key)
	}
}

func setMediaUpscalingEnvironment(key, value string) {
	if value == "" {
		restoreMediaUpscalingEnvironment(key)
		return
	}
	_ = os.Setenv(key, value)
}

func applyMediaUpscalingConfig(config mediaUpscalingConfig) {
	config = normalizeMediaUpscalingConfig(config)
	setMediaUpscalingEnvironment("STASH_WAIFU2X", config.Waifu2xExecutable)
	setMediaUpscalingEnvironment("STASH_WAIFU2X_MODELS", config.Waifu2xModels)
	setMediaUpscalingEnvironment("STASH_SEEDVR2_CLI", config.SeedVR2CLI)
	setMediaUpscalingEnvironment("STASH_SEEDVR2_MODELS", config.SeedVR2Models)
	setMediaUpscalingEnvironment("STASH_SEEDVR2_MODEL", config.SeedVR2Model)
	setMediaUpscalingEnvironment("STASH_SEEDVR2_PYTHON", config.SeedVR2Python)
	if config.SeedVR2BlocksToSwap > 0 {
		_ = os.Setenv("STASH_SEEDVR2_BLOCKS_TO_SWAP", strconv.Itoa(config.SeedVR2BlocksToSwap))
	} else {
		restoreMediaUpscalingEnvironment("STASH_SEEDVR2_BLOCKS_TO_SWAP")
	}
}

func (rs imageRoutes) MediaUpscalingConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := loadMediaUpscalingConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, config)
}

func (rs imageRoutes) MediaUpscalingConfigUpdate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var config mediaUpscalingConfig
	if err := decoder.Decode(&config); err != nil {
		http.Error(w, fmt.Sprintf("invalid media upscaling configuration: %v", err), http.StatusBadRequest)
		return
	}
	config = normalizeMediaUpscalingConfig(config)
	if err := validateMediaUpscalingConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := saveMediaUpscalingConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	applyMediaUpscalingConfig(config)
	writeVisualSimilarityJSON(w, config)
}
