package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stashapp/stash/pkg/visualembedding"
)

const visualEmbeddingModelSizeBytes = 1_260_436_067

type visualSimilarityStatusResponse struct {
	Installed     bool   `json:"installed"`
	Loaded        bool   `json:"loaded"`
	WorkerOK      bool   `json:"workerOK"`
	WorkerError   string `json:"workerError,omitempty"`
	Backend       string `json:"backend"`
	RemoteURL     string `json:"remoteURL,omitempty"`
	ModelPath     string `json:"modelPath,omitempty"`
	Model         string `json:"model"`
	Revision      string `json:"revision"`
	Dimensions    int    `json:"dimensions"`
	IndexedImages int    `json:"indexedImages"`
	TotalImages   int    `json:"totalImages"`
}

type visualSimilarityJobResponse struct {
	JobID int `json:"jobID"`
}

type visualEmbeddingImageSource struct {
	ID   int
	Path string
}

func writeVisualSimilarityJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logger.Errorf("writing visual similarity response: %v", err)
	}
}

func (rs imageRoutes) VisualSimilarityStatus(w http.ResponseWriter, r *http.Request) {
	response := visualSimilarityStatusResponse{
		Backend:    "local",
		Model:      sqlite.VisualEmbeddingModel,
		Dimensions: sqlite.VisualEmbeddingDimensions,
	}

	client, backend, remoteURL, err := newVisualSimilarityEmbedder()
	response.Backend = backend
	response.RemoteURL = remoteURL
	if err != nil {
		response.WorkerError = err.Error()
	} else {
		status, statusErr := client.Status(r.Context())
		_ = client.Close()
		if statusErr != nil {
			response.WorkerError = statusErr.Error()
		} else {
			response.WorkerOK = true
			response.Installed = status.Installed
			response.Loaded = status.Loaded
			response.ModelPath = status.ModelPath
			response.Model = status.Model
			response.Revision = status.Revision
			response.Dimensions = status.Dimensions
		}
	}

	if err := rs.withReadTxn(r, func(ctx context.Context) error {
		var countErr error
		response.IndexedImages, countErr = sqlite.VisualEmbeddings.CountImages(ctx)
		if countErr != nil {
			return fmt.Errorf("counting image visual embeddings: %w", countErr)
		}

		response.TotalImages, countErr = manager.GetInstance().Repository.Image.Count(ctx)
		if countErr != nil {
			return fmt.Errorf("counting images: %w", countErr)
		}
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, response)
}

func (rs imageRoutes) VisualSimilarityDownload(w http.ResponseWriter, r *http.Request) {
	remoteConfig, err := loadVisualSimilarityRemoteConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(remoteConfig.URL) != "" {
		http.Error(w, "model downloads are managed on the remote visual embedding worker", http.StatusBadRequest)
		return
	}

	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Downloading visual similarity model...", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			client := visualembedding.New("")
			defer client.Close()

			status, err := client.Status(ctx)
			if err != nil {
				return fmt.Errorf("checking visual similarity model before download: %w", err)
			}
			if status.Installed {
				progress.SetPercent(1)
				return nil
			}

			progress.SetTotal(visualEmbeddingModelSizeBytes)
			downloadDone := make(chan error, 1)
			go func() {
				_, downloadErr := client.Download(ctx)
				downloadDone <- downloadErr
			}()

			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			partPattern := filepath.Join(
				filepath.Dir(status.ModelPath),
				filepath.Base(status.ModelPath)+".*.part",
			)

			for {
				select {
				case downloadErr := <-downloadDone:
					if downloadErr == nil {
						progress.SetProcessed(visualEmbeddingModelSizeBytes)
					}
					return downloadErr
				case <-ticker.C:
					matches, globErr := filepath.Glob(partPattern)
					if globErr != nil {
						logger.Warnf("visual similarity: checking model download progress: %v", globErr)
						continue
					}
					var downloaded int64
					for _, match := range matches {
						stat, statErr := os.Stat(match)
						if statErr == nil && stat.Size() > downloaded {
							downloaded = stat.Size()
						}
					}
					if downloaded > 0 {
						progress.SetProcessed(int(downloaded))
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		},
	))

	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

func (rs imageRoutes) VisualSimilarityIndexImages(w http.ResponseWriter, r *http.Request) {
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Generating image visual embeddings...", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			var sources []visualEmbeddingImageSource
			if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
				images, err := mgr.Repository.Image.All(ctx)
				if err != nil {
					return err
				}

				sources = make([]visualEmbeddingImageSource, 0, len(images))
				for _, image := range images {
					if err := image.LoadPrimaryFile(ctx, mgr.Repository.File); err != nil {
						logger.Warnf("visual similarity: loading primary file for image %d: %v", image.ID, err)
						continue
					}
					primary := image.Files.Primary()
					if primary == nil || primary.Base().Path == "" {
						continue
					}
					sources = append(sources, visualEmbeddingImageSource{ID: image.ID, Path: primary.Base().Path})
				}
				return nil
			}); err != nil {
				return fmt.Errorf("loading images for visual embedding indexing: %w", err)
			}

			progress.SetTotal(len(sources))
			if len(sources) == 0 {
				return nil
			}

			client, backend, _, err := newVisualSimilarityEmbedder()
			if err != nil {
				return fmt.Errorf("initializing %s visual embedding worker: %w", backend, err)
			}
			defer client.Close()

			status, err := client.Status(ctx)
			if err != nil {
				return fmt.Errorf("checking %s visual similarity worker: %w", backend, err)
			}
			if !status.Installed {
				if backend == "remote" {
					return fmt.Errorf("remote visual similarity worker does not have the embedding model installed")
				}
				return fmt.Errorf("visual similarity model is not installed; download it from Settings > System > Visual Similarity first")
			}

			failures := 0
			for _, source := range sources {
				if job.IsCancelled(ctx) {
					return nil
				}

				stat, err := os.Stat(source.Path)
				if err != nil {
					failures++
					logger.Warnf("visual similarity: stat image %d (%s): %v", source.ID, source.Path, err)
					progress.Increment()
					continue
				}
				sourceKey := fmt.Sprintf("%s:%d:%d", source.Path, stat.Size(), stat.ModTime().UnixNano())

				var current bool
				if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
					var checkErr error
					current, checkErr = sqlite.VisualEmbeddings.HasCurrentImage(ctx, source.ID, sourceKey)
					return checkErr
				}); err != nil {
					return fmt.Errorf("checking visual embedding for image %d: %w", source.ID, err)
				}
				if current {
					progress.Increment()
					continue
				}

				embedding, err := client.Embed(ctx, source.Path)
				if err != nil {
					failures++
					logger.Warnf("visual similarity: %s embedding image %d (%s): %v", backend, source.ID, source.Path, err)
					progress.Increment()
					continue
				}
				if err := txn.WithTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
					return sqlite.VisualEmbeddings.UpsertImage(ctx, source.ID, embedding, sourceKey)
				}); err != nil {
					return fmt.Errorf("storing visual embedding for image %d: %w", source.ID, err)
				}
				progress.Increment()
			}

			if failures > 0 {
				return fmt.Errorf("visual embedding indexing skipped %d image(s); see the logs for details", failures)
			}
			return nil
		},
	))

	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
