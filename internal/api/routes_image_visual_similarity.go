package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stashapp/stash/pkg/visualembedding"
)

type visualSimilarityStatusResponse struct {
	Installed     bool   `json:"installed"`
	Loaded        bool   `json:"loaded"`
	WorkerOK      bool   `json:"workerOK"`
	WorkerError   string `json:"workerError,omitempty"`
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
		Model:      sqlite.VisualEmbeddingModel,
		Dimensions: sqlite.VisualEmbeddingDimensions,
	}

	client := visualembedding.New("")
	status, err := client.Status(r.Context())
	_ = client.Close()
	if err != nil {
		response.WorkerError = err.Error()
	} else {
		response.WorkerOK = true
		response.Installed = status.Installed
		response.Loaded = status.Loaded
		response.ModelPath = status.ModelPath
		response.Model = status.Model
		response.Revision = status.Revision
		response.Dimensions = status.Dimensions
	}

	indexedImages, err := sqlite.VisualEmbeddings.CountImages(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("counting image visual embeddings: %v", err), http.StatusInternalServerError)
		return
	}
	response.IndexedImages = indexedImages

	if err := rs.withReadTxn(r, func(ctx context.Context) error {
		var countErr error
		response.TotalImages, countErr = manager.GetInstance().Repository.Image.Count(ctx)
		return countErr
	}); err != nil {
		http.Error(w, fmt.Sprintf("counting images: %v", err), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, response)
}

func (rs imageRoutes) VisualSimilarityDownload(w http.ResponseWriter, r *http.Request) {
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Downloading visual similarity model...", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			progress.Indefinite()
			client := visualembedding.New("")
			defer client.Close()

			_, err := client.Download(ctx)
			return err
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

			client := visualembedding.New("")
			defer client.Close()
			status, err := client.Status(ctx)
			if err != nil {
				return fmt.Errorf("checking visual similarity model: %w", err)
			}
			if !status.Installed {
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

				current, err := sqlite.VisualEmbeddings.HasCurrentImage(ctx, source.ID, sourceKey)
				if err != nil {
					return fmt.Errorf("checking visual embedding for image %d: %w", source.ID, err)
				}
				if current {
					progress.Increment()
					continue
				}

				embedding, err := client.Embed(ctx, source.Path)
				if err != nil {
					failures++
					logger.Warnf("visual similarity: embedding image %d (%s): %v", source.ID, source.Path, err)
					progress.Increment()
					continue
				}
				if err := sqlite.VisualEmbeddings.UpsertImage(ctx, source.ID, embedding, sourceKey); err != nil {
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
