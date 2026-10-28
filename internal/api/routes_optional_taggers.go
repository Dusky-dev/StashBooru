package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/pixaitagger"
)

func newPixaiTagger() (pixaitagger.Tagger, string, string, error) {
	config, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		return nil, "local", "", err
	}
	if strings.TrimSpace(config.URL) == "" {
		return pixaitagger.New(""), "local", "", nil
	}
	client, err := pixaitagger.NewRemote(config.URL, config.Token)
	return client, "remote", config.URL, err
}

type pixaiStatusResponse struct {
	Installed     bool   `json:"installed"`
	Loaded        bool   `json:"loaded"`
	WorkerOK      bool   `json:"workerOK"`
	WorkerError   string `json:"workerError,omitempty"`
	Backend       string `json:"backend"`
	RemoteURL     string `json:"remoteURL,omitempty"`
	Model         string `json:"model"`
	ModelPath     string `json:"modelPath,omitempty"`
	RuntimeExists bool   `json:"runtimeExists"`
	ModelExists   bool   `json:"modelExists"`
}

func (rs imageRoutes) PixaiStatus(w http.ResponseWriter, r *http.Request) {
	response := pixaiStatusResponse{Model: pixaitagger.Model}
	client, backend, remoteURL, err := newPixaiTagger()
	response.Backend, response.RemoteURL = backend, remoteURL
	if err == nil {
		defer client.Close()
		var status pixaitagger.Status
		status, err = client.Status(r.Context())
		if err == nil {
			response.WorkerOK = true
			response.Model = status.Model
			response.Installed = status.Installed
			response.Loaded = status.Loaded
			response.ModelPath = status.ModelPath
			response.RuntimeExists = status.RuntimeExists
			response.ModelExists = status.ModelExists
		}
	}
	if err != nil {
		response.WorkerError = err.Error()
	}
	writeVisualSimilarityJSON(w, response)
}

func (rs imageRoutes) PixaiPredictions(w http.ResponseWriter, r *http.Request) {
	config, err := resolveCamieRequestOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := imageTaggingPrimaryPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	client, backend, _, err := newPixaiTagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()
	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		http.Error(w, "PixAI is not installed on the selected inference worker", http.StatusConflict)
		return
	}
	predicted, err := client.Tag(r.Context(), path, config.Threshold, config.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	tags := make([]camietagger.Tag, 0, len(predicted))
	for _, item := range predicted {
		tags = append(tags, camietagger.Tag{Name: item.Name, Category: item.Category, Score: item.Score, Source: "pixai"})
	}
	tags, err = enrichNativeCamiePredictionTargets(r.Context(), tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, camieTagsResponse{Backend: backend, Model: pixaitagger.Model, Threshold: config.Threshold, Limit: config.Limit, Tags: tags})
}

func (rs imageRoutes) ModelInstall(w http.ResponseWriter, r *http.Request) { rs.modelAction(w, r, false) }
func (rs imageRoutes) ModelDelete(w http.ResponseWriter, r *http.Request) { rs.modelAction(w, r, true) }

func (rs imageRoutes) modelAction(w http.ResponseWriter, r *http.Request, remove bool) {
	model := "Camie"
	if strings.Contains(r.URL.Path, "/pixai/") {
		model = "PixAI"
	}
	action := "Installing"
	if remove {
		action = "Deleting"
	}
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), fmt.Sprintf("%s %s model...", action, model), job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			progress.SetTotal(1)
			if model == "PixAI" {
				client, _, _, err := newPixaiTagger()
				if err != nil {
					return err
				}
				defer client.Close()
				managed, ok := client.(pixaitagger.ModelManager)
				if !ok {
					return fmt.Errorf("pixai worker does not support model installation")
				}
				if remove {
					_, err = managed.Delete(ctx)
				} else {
					_, err = managed.Download(ctx)
				}
				if err != nil {
					return err
				}
			} else {
				client, _, _, err := newCamieTagger()
				if err != nil {
					return err
				}
				defer client.Close()
				managed, ok := client.(camietagger.ModelManager)
				if !ok {
					return fmt.Errorf("camie worker does not support model installation")
				}
				if remove {
					_, err = managed.Delete(ctx)
				} else {
					_, err = managed.Download(ctx)
				}
				if err != nil {
					return err
				}
			}
			progress.SetProcessed(1)
			return nil
		},
	))
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
