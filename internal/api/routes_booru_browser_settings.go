package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/stashapp/stash/pkg/booru"
)

var booruSettingsMu sync.Mutex

// Explicit fields ensure API keys never leave the server.
type booruSourceSetting struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Engine           string   `json:"engine"`
	URL              string   `json:"url"`
	MediaHosts       []string `json:"mediaHosts"`
	Login            string   `json:"login"`
	UserID           string   `json:"userID"`
	Disabled         bool     `json:"disabled"`
	Preset           bool     `json:"preset"`
	APIKeyConfigured bool     `json:"apiKeyConfigured"`
}

type booruSettingsResponse struct {
	Revision string               `json:"revision"`
	Sources  []booruSourceSetting `json:"sources"`
}

type booruSettingsUpdate struct {
	Revision    string       `json:"revision"`
	Action      string       `json:"action"`
	Source      booru.Source `json:"source"`
	ClearAPIKey bool         `json:"clearAPIKey"`
}

func (rs booruBrowserRoutes) readSettings() ([]byte, []booru.Source, error) {
	file, err := os.Open(filepath.Join(rs.configPath, "booru-browser.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, nil, errors.New("cannot read booru configuration")
	}
	if _, err := booru.Configure(data); err != nil {
		return nil, nil, err
	}
	var configured []booru.Source
	if err := json.Unmarshal(data, &configured); err != nil {
		return nil, nil, err
	}
	return data, configured, nil
}

func booruSettingsView(data []byte, configured []booru.Source) booruSettingsResponse {
	digest := sha256.Sum256(data)
	response := booruSettingsResponse{Revision: hex.EncodeToString(digest[:]), Sources: []booruSourceSetting{}}
	merged := booru.Defaults()
	for _, source := range configured {
		index := slices.IndexFunc(merged, func(s booru.Source) bool { return s.ID == source.ID })
		if index >= 0 {
			if source.Disabled && source.Engine == "" {
				merged[index].Disabled = true
			} else {
				merged[index] = source
			}
		} else {
			merged = append(merged, source)
		}
	}
	for _, source := range merged {
		preset := slices.ContainsFunc(booru.Defaults(), func(s booru.Source) bool { return s.ID == source.ID })
		hosts := source.MediaHosts
		if hosts == nil {
			hosts = []string{}
		}
		response.Sources = append(response.Sources, booruSourceSetting{ID: source.ID, Name: source.Name, Engine: source.Engine, URL: source.URL, MediaHosts: hosts, Login: source.Login, UserID: source.UserID, Disabled: source.Disabled, Preset: preset, APIKeyConfigured: source.APIKey != ""})
	}
	return response
}

func (rs booruBrowserRoutes) Settings(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	booruSettingsMu.Lock()
	defer booruSettingsMu.Unlock()
	data, configured, err := rs.readSettings()
	if err != nil {
		writeBooruError(w, 500, "configuration", "Cannot load booru-browser.json. Repair the server configuration before editing sources.")
		return
	}
	writeVisualSimilarityJSON(w, booruSettingsView(data, configured))
}

func (rs booruBrowserRoutes) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/json" {
		writeBooruError(w, 415, "content-type", "Settings require application/json.")
		return
	}
	// Reject cross-site browser writes even on servers without a password.
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || !strings.EqualFold(parsed.Host, r.Host) {
			writeBooruError(w, 403, "origin", "Settings must be changed from this server's interface.")
			return
		}
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeBooruError(w, 403, "origin", "Settings must be changed from this server's interface.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var update booruSettingsUpdate
	if decoder.Decode(&update) != nil || decoder.Decode(new(any)) != io.EOF {
		writeBooruError(w, 400, "settings", "Invalid source settings payload.")
		return
	}
	booruSettingsMu.Lock()
	defer booruSettingsMu.Unlock()
	data, configured, err := rs.readSettings()
	if err != nil {
		writeBooruError(w, 500, "configuration", "Cannot load booru-browser.json. Repair the server configuration before editing sources.")
		return
	}
	view := booruSettingsView(data, configured)
	if update.Revision != view.Revision {
		writeBooruError(w, 409, "conflict", "Source settings changed. Reload saved sources before saving again.")
		return
	}
	source := update.Source
	existing := slices.IndexFunc(view.Sources, func(s booruSourceSetting) bool { return s.ID == source.ID })
	switch update.Action {
	case "save":
		if source.APIKey == "" && !update.ClearAPIKey {
			for _, old := range configured {
				if old.ID == source.ID && old.APIKey != "" {
					if old.Engine != source.Engine || strings.TrimRight(old.URL, "/") != strings.TrimRight(source.URL, "/") {
						writeBooruError(w, 400, "credentials", "Re-enter or clear the API key when changing the URL or engine.")
						return
					}
					source.APIKey = old.APIKey
				}
			}
		}
		if update.ClearAPIKey {
			source.APIKey = ""
			source.Login = ""
			source.UserID = ""
		}
		// Validate disabled definitions too, so enabling one cannot hide invalid settings.
		if err := source.Validate(); err != nil {
			writeBooruError(w, 400, "settings", err.Error())
			return
		}
		configured = slices.DeleteFunc(configured, func(s booru.Source) bool { return s.ID == source.ID })
		configured = append(configured, source)
	case "remove", "reset":
		if existing < 0 || (update.Action == "remove" && view.Sources[existing].Preset) || (update.Action == "reset" && !view.Sources[existing].Preset) {
			writeBooruError(w, 400, "settings", "Remove a custom source or reset a preset source.")
			return
		}
		configured = slices.DeleteFunc(configured, func(s booru.Source) bool { return s.ID == source.ID })
	default:
		writeBooruError(w, 400, "settings", "Choose save, remove or reset.")
		return
	}
	if configured == nil {
		configured = []booru.Source{}
	}
	data, err = json.MarshalIndent(configured, "", "  ")
	if err != nil {
		writeBooruError(w, 500, "configuration", "Cannot encode source settings.")
		return
	}
	data = append(data, '\n')
	if len(data) > 1<<20 {
		writeBooruError(w, 400, "settings", "Source configuration exceeds 1 MiB.")
		return
	}
	if _, err := booru.Configure(data); err != nil {
		writeBooruError(w, 400, "settings", "Source configuration is invalid or exceeds 64 entries.")
		return
	}
	if err := rs.writeSettings(data); err != nil {
		writeBooruError(w, 500, "configuration", "Cannot save booru-browser.json. Check server directory permissions.")
		return
	}
	writeVisualSimilarityJSON(w, booruSettingsView(data, configured))
}

func (rs booruBrowserRoutes) writeSettings(data []byte) error {
	temp, err := os.CreateTemp(rs.configPath, ".booru-browser-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := temp.Chmod(0600); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), filepath.Join(rs.configPath, "booru-browser.json"))
}
