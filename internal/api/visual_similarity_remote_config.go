package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/visualembedding"
)

const visualSimilarityRemoteConfigFilename = "visual-similarity-remote.json"

var visualSimilarityRemoteConfigMu sync.Mutex

type visualSimilarityRemoteConfig struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

type visualSimilarityRemoteConfigResponse struct {
	URL             string `json:"url"`
	TokenConfigured bool   `json:"tokenConfigured"`
}

type visualSimilarityRemoteConfigUpdate struct {
	URL        string  `json:"url"`
	Token      *string `json:"token,omitempty"`
	ClearToken bool    `json:"clearToken,omitempty"`
}

func visualSimilarityRemoteConfigPath() string {
	return filepath.Join(manager.GetInstance().Config.GetConfigPath(), visualSimilarityRemoteConfigFilename)
}

func loadVisualSimilarityRemoteConfig() (visualSimilarityRemoteConfig, error) {
	visualSimilarityRemoteConfigMu.Lock()
	defer visualSimilarityRemoteConfigMu.Unlock()
	return loadVisualSimilarityRemoteConfigLocked()
}

func loadVisualSimilarityRemoteConfigLocked() (visualSimilarityRemoteConfig, error) {
	var config visualSimilarityRemoteConfig
	data, err := os.ReadFile(visualSimilarityRemoteConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, fmt.Errorf("reading visual similarity remote configuration: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("decoding visual similarity remote configuration: %w", err)
	}
	return config, nil
}

func saveVisualSimilarityRemoteConfig(config visualSimilarityRemoteConfig) error {
	visualSimilarityRemoteConfigMu.Lock()
	defer visualSimilarityRemoteConfigMu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding visual similarity remote configuration: %w", err)
	}
	data = append(data, '\n')

	path := visualSimilarityRemoteConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating visual similarity configuration directory: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".visual-similarity-remote-*.tmp")
	if err != nil {
		return fmt.Errorf("creating visual similarity configuration temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("setting visual similarity configuration permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("writing visual similarity remote configuration: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing visual similarity remote configuration: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("installing visual similarity remote configuration: %w", err)
	}
	return nil
}

func normalizeVisualSimilarityRemoteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid remote worker URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("remote worker URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("remote worker URL must include a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("remote worker URL must not contain a query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func newVisualSimilarityEmbedder() (visualembedding.Embedder, string, string, error) {
	config, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		return nil, "local", "", err
	}
	if strings.TrimSpace(config.URL) == "" {
		return visualembedding.New(""), "local", "", nil
	}

	client, err := visualembedding.NewRemote(config.URL, config.Token)
	if err != nil {
		return nil, "remote", config.URL, err
	}
	return client, "remote", config.URL, nil
}

func (rs imageRoutes) VisualSimilarityRemoteConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, visualSimilarityRemoteConfigResponse{
		URL:             config.URL,
		TokenConfigured: strings.TrimSpace(config.Token) != "",
	})
}

func (rs imageRoutes) VisualSimilarityRemoteConfigUpdate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var update visualSimilarityRemoteConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, fmt.Sprintf("invalid remote worker configuration: %v", err), http.StatusBadRequest)
		return
	}

	normalizedURL, err := normalizeVisualSimilarityRemoteURL(update.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	current, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	current.URL = normalizedURL
	if update.ClearToken {
		current.Token = ""
	} else if update.Token != nil {
		current.Token = strings.TrimSpace(*update.Token)
	}

	if err := saveVisualSimilarityRemoteConfig(current); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, visualSimilarityRemoteConfigResponse{
		URL:             current.URL,
		TokenConfigured: strings.TrimSpace(current.Token) != "",
	})
}
