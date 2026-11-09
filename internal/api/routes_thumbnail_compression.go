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
	Action       string `json:"action,omitempty"`
	Category     string `json:"category"`
	Enabled      bool   `json:"enabled"`
	Format       string `json:"format"`
	Quality      int    `json:"quality"`
	KeepOld      bool   `json:"keepOld"`
	Acknowledged bool   `json:"acknowledged"`
}

var thumbnailRebuildMu sync.Mutex
var thumbnailRebuildBusy bool
var thumbnailRebuildID int
var thumbnailRebuildCategory string
var thumbnailRebuildOperation string

func thumbnailKeepOldKey(category string) string {
	switch category {
	case "image":
		return "thumbnailImageKeepOld"
	case "clip":
		return "thumbnailVideoKeepOld"
	default:
		return ""
	}
}

func thumbnailKeepOld(mgr *manager.Manager, category string) bool {
	key := thumbnailKeepOldKey(category)
	if key == "" {
		return false // native animated images are originals, not a generated cache
	}
	value, _ := mgr.Config.GetUIConfiguration()[key].(bool)
	return value
}

func thumbnailEnabled(mgr *manager.Manager, category string) bool {
	switch category {
	case "image":
		return mgr.Config.IsExperimentalImageThumbnailEnabled()
	case "clip":
		return mgr.Config.IsExperimentalClipPreviewEnabled()
	case "animated":
		return mgr.Config.IsExperimentalAnimatedThumbnailEnabled()
	default:
		return false
	}
}

func setThumbnailKeepOld(mgr *manager.Manager, category string, keep bool) error {
	key := thumbnailKeepOldKey(category)
	if key == "" {
		return fmt.Errorf("unsupported native thumbnail category")
	}
	previous := mgr.Config.GetUIConfiguration()
	next := make(map[string]interface{}, len(previous)+1)
	for k, v := range previous {
		next[k] = v
	}
	next[key] = keep
	mgr.Config.SetUIConfiguration(next)
	if err := mgr.Config.Write(); err != nil {
		mgr.Config.SetUIConfiguration(previous)
		return err
	}
	return nil
}

func thumbnailRebuildBusyLocked(mgr *manager.Manager) bool {
	if thumbnailRebuildBusy && thumbnailRebuildID != 0 {
		active := mgr.JobManager.GetJob(thumbnailRebuildID)
		if active == nil || active.Status == job.StatusFinished || active.Status == job.StatusCancelled || active.Status == job.StatusFailed {
			thumbnailRebuildBusy = false
		}
	}
	return thumbnailRebuildBusy
}

func queueThumbnailCacheOperation(w http.ResponseWriter, r *http.Request, mgr *manager.Manager, category, operation string, fn job.JobExecFn) {
	thumbnailRebuildBusy = true
	thumbnailRebuildCategory = category
	thumbnailRebuildOperation = operation
	description := "Rebuild thumbnail cache"
	if operation == "deleteOld" {
		description = "Delete retained old thumbnails"
	} else if operation == "restore" {
		description = "Regenerate native thumbnails"
	}
	id := mgr.JobManager.Add(r.Context(), description+" ("+category+")", job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		defer func() {
			thumbnailRebuildMu.Lock()
			thumbnailRebuildBusy = false
			thumbnailRebuildMu.Unlock()
		}()
		return fn(ctx, progress)
	}))
	thumbnailRebuildID = id
	writeVisualSimilarityJSON(w, map[string]any{
		"busy": true, "jobID": id, "operation": operation,
		"category": category, "enabled": thumbnailEnabled(mgr, category),
	})
}

func validateThumbnailCompressionRequest(req thumbnailCompressionRequest) error {
	if req.Category != "image" && req.Category != "clip" && req.Category != "animated" {
		return fmt.Errorf("choose still thumbnails, clip previews, or animated thumbnails")
	}
	if req.Action != "" && req.Action != "apply" && req.Action != "deleteOld" {
		return fmt.Errorf("unknown thumbnail cache operation")
	}
	if req.Action == "deleteOld" {
		if req.Category == "animated" {
			return fmt.Errorf("animated originals cannot be deleted as an old thumbnail cache")
		}
		if !req.Acknowledged {
			return fmt.Errorf("deleting retained native thumbnails requires explicit confirmation")
		}
		return nil
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
	busy := thumbnailRebuildBusyLocked(m)
	id, category, operation := thumbnailRebuildID, thumbnailRebuildCategory, thumbnailRebuildOperation
	thumbnailRebuildMu.Unlock()
	status := map[string]any{
		"busy":            busy,
		"jobID":           id,
		"category":        category,
		"operation":       operation,
		"imageKeepOld":    thumbnailKeepOld(m, "image"),
		"clipKeepOld":     thumbnailKeepOld(m, "clip"),
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
	if thumbnailRebuildBusyLocked(manager.GetInstance()) {
		http.Error(w, "a thumbnail cache operation is already queued or running", http.StatusConflict)
		return
	}

	m := manager.GetInstance()
	if req.Action == "deleteOld" {
		if !thumbnailEnabled(m, req.Category) {
			http.Error(w, "enable the selected experimental codec before deleting old thumbnails", http.StatusConflict)
			return
		}
		if !thumbnailKeepOld(m, req.Category) {
			http.Error(w, "this category has no retained native cache configured for deletion", http.StatusConflict)
			return
		}
		queueThumbnailCacheOperation(w, r, m, req.Category, "deleteOld", func(ctx context.Context, progress *job.Progress) error {
			progress.SetTotal(1)
			removed, err := manager.RemoveOldThumbnailCategory(ctx, m.Paths.Generated.Thumbnails, req.Category)
			if err != nil {
				return err
			}
			if err := setThumbnailKeepOld(m, req.Category, false); err != nil {
				return fmt.Errorf("removed %d native cached files but could not save retention setting: %w", removed, err)
			}
			progress.SetProcessed(1)
			return nil
		})
		return
	}
	if err := preflightThumbnailCodec(r.Context(), req, m); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	priorEnabled := thumbnailEnabled(m, req.Category)
	priorKept := thumbnailKeepOld(m, req.Category)
	settings := m.Config.GetUIConfiguration()
	next := make(map[string]interface{}, len(settings)+5)
	for key, value := range settings {
		next[key] = value
	}
	switch req.Category {
	case "image":
		next["thumbnailImageEnabled"] = req.Enabled
		next["thumbnailImageFormat"] = req.Format
		next["thumbnailImageQuality"] = req.Quality
		if req.Enabled {
			next["thumbnailImageKeepOld"] = req.KeepOld
		}
	case "clip":
		next["thumbnailVideoEnabled"] = req.Enabled
		next["thumbnailVideoFormat"] = req.Format
		next["thumbnailVideoQuality"] = req.Quality
		if req.Enabled {
			next["thumbnailVideoKeepOld"] = req.KeepOld
		}
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
		// With retained native files, switching back is immediate: the
		// serving paths already point to the untouched legacy JPEG/WebM.
		// If the user deleted them, regenerate only that category.
		// The playable clip endpoint cannot create missing native WebM files
		// on demand. Regenerate any missing clips while preserving old ones.
		// For still thumbnails, generate only when the old cache was deleted.
		if priorEnabled && (req.Category == "clip" || (req.Category == "image" && !priorKept)) {
			queueThumbnailCacheOperation(w, r, m, req.Category, "restore", func(ctx context.Context, progress *job.Progress) error {
				return m.RebuildExperimentalThumbnailCache(ctx, progress, req.Category, priorKept)
			})
			return
		}
		writeVisualSimilarityJSON(w, map[string]any{"enabled": false, "busy": false})
		return
	}
	queueThumbnailCacheOperation(w, r, m, req.Category, "rebuild", func(ctx context.Context, progress *job.Progress) error {
		return m.RebuildExperimentalThumbnailCache(ctx, progress, req.Category, req.KeepOld)
	})
}
