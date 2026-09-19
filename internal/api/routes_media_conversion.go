package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

func conversionStore() mediaconvert.Store {
	return manager.GetInstance().MediaConversionStore()
}

// Booru identity survives re-encoding even after its original bytes are evicted.
func lookupConvertedMediaBooruMetadata(ctx context.Context, path string) (booruProvider, *booruPost, string, string, error) {
	mgr := manager.GetInstance()
	var sourceMD5 string
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		f, err := mgr.Repository.File.FindByPath(ctx, path, true)
		if f != nil {
			sourceMD5 = f.Base().Fingerprints.GetString("source_md5")
		}
		return err
	})
	if err != nil {
		return booruProvider{}, nil, "", "", err
	}
	if sourceMD5 != "" {
		provider, post, err := lookupBooruPost(ctx, sourceMD5)
		return provider, post, sourceMD5, "original", err
	}
	return lookupImageBooruMetadata(ctx, path, lookupBooruPost)
}

func conversionClient(backend string) (mediaconvert.Client, error) {
	mgr := manager.GetInstance()
	c := mediaconvert.Client{}
	if mgr.FFMpeg != nil {
		c.FFMpeg = mgr.FFMpeg.Path()
	}
	if mgr.FFProbe != nil {
		c.FFProbe = mgr.FFProbe.Path()
	}
	switch backend {
	case "local", "":
		return c, nil
	case "remote":
		config, err := loadVisualSimilarityRemoteConfig()
		if err != nil {
			return c, err
		}
		if config.URL == "" {
			return c, fmt.Errorf("configure the remote tagging worker URL in Visual Similarity settings first")
		}
		c.URL, c.Token = config.URL, config.Token
		return c, nil
	default:
		return c, fmt.Errorf("unknown conversion backend")
	}
}

type conversionTarget struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}
type conversionRequest struct {
	Action          string               `json:"action"`
	Backend         string               `json:"backend"`
	Targets         []conversionTarget   `json:"targets"`
	Options         mediaconvert.Options `json:"options"`
	RecordID        string               `json:"recordID"`
	JobID           int                  `json:"jobID"`
	CacheLimitBytes int64                `json:"cacheLimitBytes"`
}
type conversionItem struct {
	Target   conversionTarget `json:"target"`
	RecordID string           `json:"recordID,omitempty"`
	Error    string           `json:"error,omitempty"`
}
type conversionJob struct {
	JobID  int              `json:"jobID"`
	Batch  string           `json:"batch"`
	Status string           `json:"status"`
	Total  int              `json:"total"`
	Items  []conversionItem `json:"items"`
	Error  string           `json:"error,omitempty"`
}

var conversionJobs = struct {
	sync.Mutex
	jobs map[int]*conversionJob
}{jobs: make(map[int]*conversionJob)}
var conversionMutations sync.Mutex

func conversionTargetFile(ctx context.Context, target conversionTarget, family string) (models.FileID, error) {
	mgr := manager.GetInstance()
	var id models.FileID
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		switch target.Kind {
		case "image":
			im, err := mgr.Repository.Image.Find(ctx, target.ID)
			if err != nil {
				return err
			}
			if im == nil || im.PrimaryFileID == nil {
				return fmt.Errorf("image has no primary file")
			}
			id = *im.PrimaryFileID
		case "scene":
			if family != "video" {
				return fmt.Errorf("video entries require a video output format")
			}
			v, err := mgr.Repository.Scene.Find(ctx, target.ID)
			if err != nil {
				return err
			}
			if v == nil || v.PrimaryFileID == nil {
				return fmt.Errorf("video has no primary file")
			}
			id = *v.PrimaryFileID
		default:
			return fmt.Errorf("invalid media kind")
		}
		images, err := mgr.Repository.Image.FindByFileID(ctx, id)
		if err != nil {
			return err
		}
		scenes, err := mgr.Repository.Scene.FindByFileID(ctx, id)
		if err != nil {
			return err
		}
		if len(images)+len(scenes) != 1 {
			return fmt.Errorf("file is shared by multiple media entries; separate them before converting")
		}
		return nil
	})
	return id, err
}

func handleMediaConversionGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("capabilities") == "1" {
		client, err := conversionClient(r.URL.Query().Get("backend"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		capabilities, err := client.Capabilities(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeVisualSimilarityJSON(w, capabilities)
		return
	}
	s := conversionStore()
	config, err := s.Config()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	history, err := s.History()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats := mediaconvert.Summarize(history)
	var activeJob *conversionJob
	id, _ := strconv.Atoi(r.URL.Query().Get("jobID"))
	jobStatus := manager.GetInstance().JobManager.GetJob(id)
	conversionJobs.Lock()
	if state := conversionJobs.jobs[id]; state != nil {
		if jobStatus != nil && jobStatus.Status == job.StatusCancelled && state.Status == "queued" {
			state.Status = "cancelled"
		}
		jobCopy := *state
		jobCopy.Items = append([]conversionItem{}, state.Items...)
		activeJob = &jobCopy
	}
	conversionJobs.Unlock()
	batchRecords := []*mediaconvert.Record{}
	if activeJob != nil {
		for _, record := range history {
			if record.Batch == activeJob.Batch {
				batchRecords = append(batchRecords, record)
			}
		}
	}
	// Return newest first, while lifetime stats include all retained manifests.
	for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
		history[left], history[right] = history[right], history[left]
	}
	// Pagination keeps older restore points reachable.
	historyTotal := len(history)
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	if offset > historyTotal {
		offset = historyTotal
	}
	end := min(offset+200, historyTotal)
	history = history[offset:end]
	batchStats := mediaconvert.Summarize(batchRecords)
	// JSON numbers cannot represent all 64-bit pHashes in the browser.
	for _, record := range history {
		for _, snap := range []mediaconvert.Snapshot{record.Before, record.After} {
			if f := snap.File(); f != nil {
				for i, fp := range f.Base().Fingerprints {
					f.Base().Fingerprints[i].Fingerprint = fp.Value()
				}
			}
		}
	}
	writeVisualSimilarityJSON(w, map[string]interface{}{"config": config, "history": history, "historyTotal": historyTotal, "stats": stats, "job": activeJob, "batchStats": batchStats})
}

func handleMediaConversionPost(w http.ResponseWriter, r *http.Request) {
	var request conversionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mgr := manager.GetInstance()
	if request.Action == "cancel" {
		mgr.JobManager.CancelJob(request.JobID)
		conversionJobs.Lock()
		if j := conversionJobs.jobs[request.JobID]; j != nil && j.Status == "queued" {
			j.Status = "cancelled"
		}
		conversionJobs.Unlock()
		writeVisualSimilarityJSON(w, map[string]bool{"cancelled": true})
		return
	}
	if request.Action != "start" && request.Action != "restore" && request.Action != "configure" && request.Action != "recover" {
		http.Error(w, "unknown conversion action", http.StatusBadRequest)
		return
	}
	if request.Action == "start" && (len(request.Targets) == 0 || len(request.Targets) > 10000) {
		http.Error(w, "select between 1 and 10000 media entries", http.StatusBadRequest)
		return
	}
	for _, target := range request.Targets {
		if target.ID <= 0 || (target.Kind != "image" && target.Kind != "scene") {
			http.Error(w, "invalid media target", http.StatusBadRequest)
			return
		}
	}
	uniqueTargets := make([]conversionTarget, 0, len(request.Targets))
	targetSeen := map[conversionTarget]bool{}
	for _, target := range request.Targets {
		if !targetSeen[target] {
			uniqueTargets = append(uniqueTargets, target)
			targetSeen[target] = true
		}
	}
	request.Targets = uniqueTargets
	if request.CacheLimitBytes < 0 {
		http.Error(w, "cache size cannot be negative", http.StatusBadRequest)
		return
	}
	state := &conversionJob{Batch: mediaconvert.NewID(), Status: "queued", Total: len(request.Targets), Items: []conversionItem{}}
	setState := func(status string, err error) {
		conversionJobs.Lock()
		defer conversionJobs.Unlock()
		state.Status = status
		if err != nil {
			state.Error = err.Error()
		}
	}
	jobID := mgr.JobManager.Add(r.Context(), "Media converter: "+request.Action, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) (retErr error) {
		conversionMutations.Lock()
		defer conversionMutations.Unlock()
		setState("running", nil)
		defer func() {
			switch {
			case ctx.Err() != nil:
				setState("cancelled", ctx.Err())
			case retErr != nil:
				setState("failed", retErr)
			default:
				setState("complete", nil)
			}
		}()
		s := conversionStore()
		if err := s.Recover(ctx); err != nil {
			return err
		}
		switch request.Action {
		case "configure":
			if err := s.Configure(mediaconvert.Config{CacheLimitBytes: request.CacheLimitBytes}); err != nil {
				return err
			}
			return s.Trim()
		case "restore":
			if err := s.Restore(ctx, request.RecordID); err != nil {
				return err
			}
			record, err := s.Record(request.RecordID)
			if err != nil {
				return err
			}
			return generateConvertedMedia(ctx, []models.FileID{record.Before.File().Base().ID})
		case "recover":
			return s.Trim()
		}
		client, err := conversionClient(request.Backend)
		if err != nil {
			return err
		}
		capabilities, err := client.Capabilities(ctx)
		if err != nil {
			return err
		}
		var format mediaconvert.Format
		for _, f := range capabilities.Formats {
			if f.ID == request.Options.Format {
				format = f
				break
			}
		}
		if !format.Available {
			return fmt.Errorf("output format is unavailable on the selected worker")
		}
		// Extension is returned by a remote worker; never use it as a filesystem path.
		allowedExtensions := map[string]bool{"jxl": true, "mp4": true, "mkv": true, "webm": true, "mov": true, "jpg": true, "png": true, "webp": true, "avif": true, "gif": true, "tiff": true, "bmp": true}
		if !allowedExtensions[format.Extension] || filepath.Base(format.Extension) != format.Extension {
			return fmt.Errorf("invalid output extension")
		}
		seen := map[models.FileID]bool{}
		var converted []models.FileID
		defer func() {
			if len(converted) > 0 {
				_ = generateConvertedMedia(context.WithoutCancel(ctx), converted)
			}
		}()
		progress.SetTotal(len(request.Targets))
		for _, target := range request.Targets {
			if err := ctx.Err(); err != nil {
				return err
			}
			item := conversionItem{Target: target}
			id, err := conversionTargetFile(ctx, target, format.Family)
			if err == nil && seen[id] {
				progress.Increment()
				continue
			}
			if err == nil {
				seen[id] = true
				record, convertErr := s.Convert(ctx, id, state.Batch, client, request.Options, format)
				err = convertErr
				if record != nil {
					item.RecordID = record.ID
					if record.Status == "complete" {
						converted = append(converted, id)
					}
				}
			}
			if err != nil {
				item.Error = err.Error()
			}
			conversionJobs.Lock()
			state.Items = append(state.Items, item)
			conversionJobs.Unlock()
			progress.Increment()
		}
		return nil
	}))
	conversionJobs.Lock()
	state.JobID = jobID
	conversionJobs.jobs[jobID] = state
	conversionJobs.Unlock()
	time.AfterFunc(24*time.Hour, func() {
		conversionJobs.Lock()
		defer conversionJobs.Unlock()
		if state.Status != "queued" && state.Status != "running" {
			delete(conversionJobs.jobs, jobID)
		}
	})
	writeVisualSimilarityJSON(w, map[string]int{"jobID": jobID})
}

func generateConvertedMedia(ctx context.Context, ids []models.FileID) error {
	mgr := manager.GetInstance()
	input := manager.GenerateMetadataInput{Covers: true, Sprites: true, Previews: true, ImagePreviews: true, Phashes: true, ImagePhashes: true, ImageThumbnails: true, ClipPreviews: true}
	if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		for _, id := range ids {
			images, err := mgr.Repository.Image.FindByFileID(ctx, id)
			if err != nil {
				return err
			}
			for _, im := range images {
				input.ImageIDs = append(input.ImageIDs, strconv.Itoa(im.ID))
			}
			scenes, err := mgr.Repository.Scene.FindByFileID(ctx, id)
			if err != nil {
				return err
			}
			for _, scene := range scenes {
				input.SceneIDs = append(input.SceneIDs, strconv.Itoa(scene.ID))
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(input.ImageIDs)+len(input.SceneIDs) == 0 {
		return nil
	}
	_, err := mgr.Generate(ctx, input)
	return err
}
