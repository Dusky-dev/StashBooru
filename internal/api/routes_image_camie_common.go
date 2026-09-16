package api

import (
	"net/http"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
)

const maxCamieApplyTags = 500

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
	Backend   string            `json:"backend"`
	Model     string            `json:"model"`
	Threshold float64           `json:"threshold"`
	Limit     int               `json:"limit"`
	Tags      []camietagger.Tag `json:"tags"`
}

type camieApplyRequest struct {
	Tags          []camietagger.Tag `json:"tags"`
	ReplaceArtist bool              `json:"replaceArtist"`
}

type camieAppliedEntity struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
	Created  bool    `json:"created"`
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
