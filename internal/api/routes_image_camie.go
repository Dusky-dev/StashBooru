package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type camieStatusResponse struct {
	Installed      bool   `json:"installed"`
	Loaded         bool   `json:"loaded"`
	WorkerOK       bool   `json:"workerOK"`
	WorkerError    string `json:"workerError,omitempty"`
	Backend        string `json:"backend"`
	RemoteURL      string `json:"remoteURL,omitempty"`
	ModelExists    bool   `json:"modelExists"`
	MetadataExists bool   `json:"metadataExists"`
	ModelPath      string `json:"modelPath,omitempty"`
	MetadataPath   string `json:"metadataPath,omitempty"`
	Model          string `json:"model"`
	TagCount       int    `json:"tagCount"`
}

type camieTagsResponse struct {
	Backend   string             `json:"backend"`
	Model     string             `json:"model"`
	Threshold float64            `json:"threshold"`
	Limit     int                `json:"limit"`
	Tags      []camietagger.Tag `json:"tags"`
}

func newCamieTagger() (camietagger.Tagger, string, string, error) {
	config, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		return nil, "local", "", err
	}
	if strings.TrimSpace(config.URL) == "" {
		return camietagger.New(""), "local", "", nil
	}

	client, err := camietagger.NewRemote(config.URL, config.Token)
	if err != nil {
		return nil, "remote", config.URL, err
	}
	return client, "remote", config.URL, nil
}

func (rs imageRoutes) CamieStatus(w http.ResponseWriter, r *http.Request) {
	response := camieStatusResponse{
		Backend: "local",
		Model:   camietagger.Model,
	}

	client, backend, remoteURL, err := newCamieTagger()
	response.Backend = backend
	response.RemoteURL = remoteURL
	if err != nil {
		response.WorkerError = err.Error()
		writeVisualSimilarityJSON(w, response)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		response.WorkerError = err.Error()
		writeVisualSimilarityJSON(w, response)
		return
	}

	response.WorkerOK = true
	response.Installed = status.Installed
	response.Loaded = status.Loaded
	response.ModelExists = status.ModelExists
	response.MetadataExists = status.MetadataExists
	response.ModelPath = status.ModelPath
	response.MetadataPath = status.MetadataPath
	response.Model = status.Model
	response.TagCount = status.TagCount
	response.WorkerError = status.Error
	writeVisualSimilarityJSON(w, response)
}

func (rs imageRoutes) ImageKnowledgeTags(w http.ResponseWriter, r *http.Request) {
	threshold := camietagger.DefaultThreshold
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed >= 1 {
			http.Error(w, "threshold must be a number greater than 0 and less than 1", http.StatusBadRequest)
			return
		}
		threshold = parsed
	}

	limit := camietagger.DefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > camietagger.MaxLimit {
			http.Error(w, fmt.Sprintf("limit must be between 1 and %d", camietagger.MaxLimit), http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	client, backend, _, err := newCamieTagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("checking %s Camie worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		message := fmt.Sprintf(
			"Camie Tagger v2 is optional and is not installed. Place camie-tagger-v2.onnx at %s and camie-tagger-v2-metadata.json at %s",
			status.ModelPath,
			status.MetadataPath,
		)
		if status.Error != "" {
			message += ": " + status.Error
		}
		http.Error(w, message, http.StatusConflict)
		return
	}

	tags, err := client.Tag(r.Context(), primary.Base().Path, threshold, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("tagging image %d with %s Camie worker: %v", image.ID, backend, err), http.StatusBadGateway)
		return
	}

	writeVisualSimilarityJSON(w, camieTagsResponse{
		Backend:   backend,
		Model:     camietagger.Model,
		Threshold: threshold,
		Limit:     limit,
		Tags:      tags,
	})
}
