package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
)

type thumbnailCompressionRequest struct {
	Category     string `json:"category"`
	Enabled      bool   `json:"enabled"`
	Format       string `json:"format"`
	Quality      int    `json:"quality"`
	Acknowledged bool   `json:"acknowledged"`
}

var thumbnailRebuildMu sync.Mutex
var thumbnailRebuildBusy bool
var thumbnailRebuildID int
var thumbnailRebuildCategory string

func validateThumbnailCompressionRequest(req thumbnailCompressionRequest) error {
	if req.Category != "image" && req.Category != "clip" && req.Category != "animated" {
		return fmt.Errorf("choose still thumbnails, clip previews, or animated thumbnails")
	}
	if req.Quality < 10 || req.Quality > 100 {
		return fmt.Errorf("quality must be between 10 and 100")
	}
	switch req.Category {
	case "image":
		if req.Format != "jpg" && req.Format != "webp" && req.Format != "jxl" {
			return fmt.Errorf("unsupported still thumbnail format")
		}
	case "clip":
		if req.Format != "webm" && req.Format != "mp4" {
			return fmt.Errorf("unsupported clip preview format")
		}
	case "animated":
		if req.Format != "webp" {
			return fmt.Errorf("animated thumbnails currently support WebP only")
		}
	}
	if req.Enabled && !req.Acknowledged {
		return fmt.Errorf("cache deletion and rebuild must be acknowledged")
	}
	return nil
}

// Verify the requested codec before deleting any existing generated cache.
// An unavailable JXL/WebP/VP9/H264 encoder must fail the confirmation request
// without changing settings or deleting thumbnails.
func preflightThumbnailCodec(ctx context.Context, req thumbnailCompressionRequest, mgr *manager.Manager) error {
	if !req.Enabled {
		return nil
	}
	vipsOperation := ""
	ffmpegEncoder := ""
	switch req.Category {
	case "image":
		switch req.Format {
		case "jxl":
			vipsOperation = "jxlsave"
		case "webp":
			vipsOperation = "webpsave"
			ffmpegEncoder = "libwebp"
		default:
			return nil
		}
	case "animated":
		ffmpegEncoder = "libwebp_anim"
	case "clip":
		if req.Format == "mp4" {
			ffmpegEncoder = "libx264"
		} else {
			ffmpegEncoder = "libvpx-vp9"
		}
	}
	if vipsOperation != "" {
		if vips := image.GetVipsPath(); vips != "" {
			data, err := exec.CommandContext(ctx, vips, "-l").Output()
			if err == nil && strings.Contains(strings.ToLower(string(data)), vipsOperation) {
				return nil
			}
		}
	}
	if ffmpegEncoder != "" && mgr.FFMpeg != nil {
		out, err := mgr.FFMpeg.Command(ctx, ffmpeg.Args{"-hide_banner", "-encoders"}).Output()
		if err == nil && strings.Contains(string(out), " "+ffmpegEncoder+" ") {
			return nil
		}
	}
	return fmt.Errorf("%s encoder unavailable; no thumbnail settings were changed or caches deleted", req.Format)
}

func (rs imageRoutes) ThumbnailCompressionStatus(w http.ResponseWriter, r *http.Request) {
	m := manager.GetInstance()
	thumbnailRebuildMu.Lock()
	busy, id, category := thumbnailRebuildBusy, thumbnailRebuildID, thumbnailRebuildCategory
	thumbnailRebuildMu.Unlock()
	status := map[string]any{
		"busy":            busy,
		"jobID":           id,
		"category":        category,
		"imageEnabled":    m.Config.IsExperimentalImageThumbnailEnabled(),
		"clipEnabled":     m.Config.IsExperimentalClipPreviewEnabled(),
		"animatedEnabled": m.Config.IsExperimentalAnimatedThumbnailEnabled(),
	}
	if id != 0 {
		if current := m.JobManager.GetJob(id); current != nil {
			status["jobStatus"] = current.Status
			status["progress"] = current.Progress
			if current.Error != nil {
				status["jobError"] = *current.Error
			}
			if current.Status == job.StatusReady || current.Status == job.StatusRunning || current.Status == job.StatusStopping {
				status["busy"] = true
			}
		}
	}
	writeVisualSimilarityJSON(w, status)
}

func (rs imageRoutes) ThumbnailCompressionApply(w http.ResponseWriter, r *http.Request) {
	var req thumbnailCompressionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid thumbnail settings request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "unexpected additional request data", http.StatusBadRequest)
		return
	}
	if err := validateThumbnailCompressionRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	thumbnailRebuildMu.Lock()
	defer thumbnailRebuildMu.Unlock()
	if thumbnailRebuildBusy {
		http.Error(w, "a thumbnail cache rebuild is already queued or running", http.StatusConflict)
		return
	}

	m := manager.GetInstance()
	if err := preflightThumbnailCodec(r.Context(), req, m); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	settings := m.Config.GetUIConfiguration()
	next := make(map[string]interface{}, len(settings)+4)
	for key, value := range settings {
		next[key] = value
	}
	switch req.Category {
	case "image":
		next["thumbnailImageEnabled"] = req.Enabled
		next["thumbnailImageFormat"] = req.Format
		next["thumbnailImageQuality"] = req.Quality
	case "clip":
		next["thumbnailVideoEnabled"] = req.Enabled
		next["thumbnailVideoFormat"] = req.Format
		next["thumbnailVideoQuality"] = req.Quality
	case "animated":
		next["thumbnailAnimatedEnabled"] = req.Enabled
		next["thumbnailAnimatedQuality"] = req.Quality
	}
	m.Config.SetUIConfiguration(next)
	if err := m.Config.Write(); err != nil {
		m.Config.SetUIConfiguration(settings)
		http.Error(w, "unable to persist thumbnail settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !req.Enabled {
		// Revert to native format without deleting any other cache category.
		// Native default thumbnails regenerate on demand if absent.
		writeVisualSimilarityJSON(w, map[string]any{"enabled": false, "busy": false})
		return
	}

	thumbnailRebuildBusy = true
	thumbnailRebuildCategory = req.Category
	id := m.JobManager.Add(r.Context(), "Rebuild experimental "+req.Category+" thumbnail cache", job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			defer func() {
				thumbnailRebuildMu.Lock()
				thumbnailRebuildBusy = false
				thumbnailRebuildMu.Unlock()
			}()
			return m.RebuildExperimentalThumbnailCache(ctx, progress, req.Category)
		},
	))
	thumbnailRebuildID = id
	writeVisualSimilarityJSON(w, map[string]any{"enabled": true, "busy": true, "jobID": id, "category": req.Category})
}
