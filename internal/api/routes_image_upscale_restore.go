package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
)

var upscalingRestoreJobs = struct {
	sync.Mutex
	jobs map[int]*conversionJob
}{jobs: make(map[int]*conversionJob)}

func upscalingRestoreStore() mediaconvert.Store {
	return manager.GetInstance().MediaUpscalingStore()
}

func handleImageUpscaleRestoreGet(w http.ResponseWriter, r *http.Request) {
	s := upscalingRestoreStore()
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
	latestRecords := []*mediaconvert.Record{}
	if len(history) > 0 {
		latestBatch := history[len(history)-1].Batch
		for _, record := range history {
			if record.Batch == latestBatch {
				latestRecords = append(latestRecords, record)
			}
		}
	}
	latestStats := mediaconvert.Summarize(latestRecords)

	var activeJob *conversionJob
	id, _ := strconv.Atoi(r.URL.Query().Get("jobID"))
	jobStatus := manager.GetInstance().JobManager.GetJob(id)
	upscalingRestoreJobs.Lock()
	if state := upscalingRestoreJobs.jobs[id]; state != nil {
		if jobStatus != nil && jobStatus.Status == job.StatusCancelled && state.Status == "queued" {
			state.Status = "cancelled"
		}
		jobCopy := *state
		jobCopy.Items = append([]conversionItem{}, state.Items...)
		activeJob = &jobCopy
	}
	upscalingRestoreJobs.Unlock()

	batchRecords := []*mediaconvert.Record{}
	if activeJob != nil {
		for _, record := range history {
			if record.Batch == activeJob.Batch {
				batchRecords = append(batchRecords, record)
			}
		}
	}
	batchStats := mediaconvert.Summarize(batchRecords)

	for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
		history[left], history[right] = history[right], history[left]
	}
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

	// JSON numbers cannot safely represent every 64-bit perceptual hash.
	for _, record := range history {
		for _, snap := range []mediaconvert.Snapshot{record.Before, record.After} {
			if f := snap.File(); f != nil {
				for i, fp := range f.Base().Fingerprints {
					f.Base().Fingerprints[i].Fingerprint = fp.Value()
				}
			}
		}
	}

	writeVisualSimilarityJSON(w, map[string]interface{}{
		"config":       config,
		"history":      history,
		"historyTotal": historyTotal,
		"stats":        stats,
		"job":          activeJob,
		"batchStats":   batchStats,
		"latestStats":  latestStats,
	})
}

func handleImageUpscaleRestorePost(w http.ResponseWriter, r *http.Request) {
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
		upscalingRestoreJobs.Lock()
		if state := upscalingRestoreJobs.jobs[request.JobID]; state != nil && state.Status == "queued" {
			state.Status = "cancelled"
		}
		upscalingRestoreJobs.Unlock()
		writeVisualSimilarityJSON(w, map[string]bool{"cancelled": true})
		return
	}
	if request.Action != "replace" && request.Action != "restore" && request.Action != "configure" && request.Action != "recover" {
		http.Error(w, "unknown upscaling restore action", http.StatusBadRequest)
		return
	}
	if request.Action == "replace" && (len(request.Targets) == 0 || len(request.Targets) > 10000) {
		http.Error(w, "select between 1 and 10000 images", http.StatusBadRequest)
		return
	}
	for _, target := range request.Targets {
		if target.ID <= 0 || target.Kind != "image" {
			http.Error(w, "upscaling replacement only accepts image targets", http.StatusBadRequest)
			return
		}
	}
	if request.CacheLimitBytes < 0 {
		http.Error(w, "cache size cannot be negative", http.StatusBadRequest)
		return
	}

	uniqueTargets := make([]conversionTarget, 0, len(request.Targets))
	seenTargets := map[conversionTarget]bool{}
	for _, target := range request.Targets {
		if !seenTargets[target] {
			uniqueTargets = append(uniqueTargets, target)
			seenTargets[target] = true
		}
	}
	request.Targets = uniqueTargets

	state := &conversionJob{Batch: mediaconvert.NewID(), Status: "queued", Total: len(request.Targets), Items: []conversionItem{}}
	setState := func(status string, err error) {
		upscalingRestoreJobs.Lock()
		defer upscalingRestoreJobs.Unlock()
		state.Status = status
		if err != nil {
			state.Error = err.Error()
		}
	}
	jobID := mgr.JobManager.Add(r.Context(), "Image upscaling restore: "+request.Action, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) (retErr error) {
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

		s := upscalingRestoreStore()
		if err := s.Recover(ctx); err != nil {
			return err
		}
		switch request.Action {
		case "configure":
			config, err := s.Config()
			if err != nil {
				return err
			}
			config.CacheLimitBytes = request.CacheLimitBytes
			if err := s.Configure(config); err != nil {
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
			if err := s.Recover(ctx); err != nil {
				return err
			}
			return s.Trim()
		}

		conversionConfig, err := conversionStore().Config()
		if err != nil {
			return err
		}
		backend := request.Backend
		if backend == "" {
			backend = conversionConfig.Backend
		}
		client, capabilities, notice, err := conversionClient(ctx, backend)
		if err != nil {
			return err
		}
		if err := validateDerivativeUpscaler(capabilities, request.Options); err != nil {
			return err
		}
		upscalingRestoreJobs.Lock()
		state.Backend, state.Notice = conversionBackend(client), notice
		upscalingRestoreJobs.Unlock()

		progress.SetTotal(len(request.Targets))
		seenFiles := map[models.FileID]bool{}
		converted := []models.FileID{}
		defer func() {
			if len(converted) > 0 {
				_ = generateConvertedMedia(context.WithoutCancel(ctx), converted)
			}
		}()
		for _, target := range request.Targets {
			if err := ctx.Err(); err != nil {
				return err
			}
			item := conversionItem{Target: target}
			fileID, err := conversionTargetFile(ctx, target)
			if err == nil && seenFiles[fileID] {
				progress.Increment()
				continue
			}
			if err == nil {
				seenFiles[fileID] = true
				options := request.Options
				var input string
				if options.Format == "" || options.Format == "auto" {
					options.Format, input, err = conversionDefaultForFile(ctx, conversionStore(), conversionConfig, fileID)
				} else if request.UseEncodingDefaults {
					_, input, err = conversionDefaultForFile(ctx, conversionStore(), conversionConfig, fileID)
				}
				if request.UseEncodingDefaults {
					defaults := conversionConfig.DefaultEncoding(input)
					options.Quality, options.Effort = defaults.Quality, defaults.Effort
				}
				if options.Hardware == "" {
					options.Hardware = "auto"
				}
				if options.Format == "jxl" || options.Format == "ajxl" {
					options.Distance = mediaconvert.JXLDistanceFromQuality(options.Quality)
				}
				var format mediaconvert.Format
				if err == nil {
					format, err = conversionOutputFormat(capabilities, options.Format, "image")
				}
				if err == nil {
					record, convertErr := s.Convert(ctx, fileID, state.Batch, client, options, format)
					err = convertErr
					if record != nil {
						item.RecordID = record.ID
						if record.Status == "complete" {
							converted = append(converted, fileID)
						}
					}
				}
			}
			if err != nil {
				item.Error = err.Error()
			}
			upscalingRestoreJobs.Lock()
			state.Items = append(state.Items, item)
			upscalingRestoreJobs.Unlock()
			progress.Increment()
		}
		return nil
	}))
	upscalingRestoreJobs.Lock()
	state.JobID = jobID
	upscalingRestoreJobs.jobs[jobID] = state
	upscalingRestoreJobs.Unlock()
	time.AfterFunc(24*time.Hour, func() {
		upscalingRestoreJobs.Lock()
		defer upscalingRestoreJobs.Unlock()
		if state.Status != "queued" && state.Status != "running" {
			delete(upscalingRestoreJobs.jobs, jobID)
		}
	})
	writeVisualSimilarityJSON(w, map[string]int{"jobID": jobID})
}
