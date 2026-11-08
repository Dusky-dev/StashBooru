package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func conversionClient(ctx context.Context, backend string) (mediaconvert.Client, mediaconvert.Capabilities, string, error) {
	mgr := manager.GetInstance()
	if backend == "" {
		config, err := conversionStore().Config()
		if err != nil {
			return mediaconvert.Client{}, mediaconvert.Capabilities{}, "", err
		}
		backend = config.Backend
	}
	c := mediaconvert.Client{}
	if mgr.FFMpeg != nil {
		c.FFMpeg = mgr.FFMpeg.Path()
	}
	if mgr.FFProbe != nil {
		c.FFProbe = mgr.FFProbe.Path()
	}
	remote := mediaconvert.Client{}
	if backend != "local" {
		config, err := loadVisualSimilarityRemoteConfig()
		if err != nil {
			return c, mediaconvert.Capabilities{}, "", err
		}
		remote.URL, remote.Token = config.URL, config.Token
	}
	return mediaconvert.SelectWorker(ctx, backend, c, remote)
}

type conversionTarget struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}
type conversionRequest struct {
	Action                   string                                   `json:"action"`
	Backend                  string                                   `json:"backend"`
	Targets                  []conversionTarget                       `json:"targets"`
	Options                  mediaconvert.Options                     `json:"options"`
	RecordID                 string                                   `json:"recordID"`
	JobID                    int                                      `json:"jobID"`
	JobBatch                 string                                   `json:"jobBatch"`
	CacheLimitBytes          int64                                    `json:"cacheLimitBytes"`
	FormatDefaults           map[string]string                        `json:"formatDefaults"`
	FormatRulesEnabled       map[string]bool                          `json:"formatRulesEnabled"`
	UseQuality               bool                                     `json:"useQuality"`
	UseEncodingDefaults      bool                                     `json:"useEncodingDefaults"`
	UseDecodingSpeedDefaults *bool                                    `json:"useDecodingSpeedDefaults,omitempty"`
	EncodingDefaults         map[string]mediaconvert.EncodingDefaults `json:"encodingDefaults"`
	Savings                  *mediaconvert.SavingsThresholds          `json:"savings,omitempty"`
	TrialIDs                 []string                                 `json:"trialIDs,omitempty"`
	TrialCacheLimitBytes     *int64                                   `json:"trialCacheLimitBytes,omitempty"`
	TrialTTLHours            *int                                     `json:"trialTTLHours,omitempty"`
}
type conversionItem struct {
	Target   conversionTarget `json:"target"`
	RecordID string           `json:"recordID,omitempty"`
	Error    string           `json:"error,omitempty"`
	TrialID  string           `json:"trialID,omitempty"`
	Status   string           `json:"status,omitempty"`
}
type conversionJob struct {
	JobID       int                           `json:"jobID"`
	Batch       string                        `json:"batch"`
	Status      string                        `json:"status"`
	Total       int                           `json:"total"`
	Items       []conversionItem              `json:"items"`
	Error       string                        `json:"error,omitempty"`
	Backend     string                        `json:"backend,omitempty"`
	Notice      string                        `json:"notice,omitempty"`
	Action      string                        `json:"action,omitempty"`
	Estimate    *mediaconvert.Estimate        `json:"estimate,omitempty"`
	Reservation *mediaconvert.DiskReservation `json:"reservation,omitempty"`
}

var conversionJobs = processingJobRegistry{jobs: make(map[int]*conversionJob)}
var conversionMutations sync.Mutex
var conversionSettings sync.Mutex

func conversionTargetFile(ctx context.Context, target conversionTarget) (models.FileID, error) {
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

// latestRestorableConversion selects the active converted version for a
// particular file. Older conversion records must not be used to roll back a
// different active file (for instance after a second conversion or a rescan).
func latestRestorableConversion(history []*mediaconvert.Record, fileID models.FileID, currentPath string) (string, error) {
	for i := len(history) - 1; i >= 0; i-- {
		record := history[i]
		if record.Status != "complete" {
			continue
		}
		after := record.After.File()
		if after == nil || after.Base().ID != fileID || after.Base().Path != currentPath {
			continue
		}
		if !record.Cached {
			return "", fmt.Errorf("the original for this conversion is no longer in the restore cache")
		}
		return record.ID, nil
	}
	return "", fmt.Errorf("no converted version with a restorable original was found for this file")
}

func handleMediaConversionGet(w http.ResponseWriter, r *http.Request) {
	if id := r.URL.Query().Get("trialID"); id != "" {
		role := r.URL.Query().Get("file")
		if role != "source" && role != "output" {
			http.Error(w, "choose source or output", http.StatusBadRequest)
			return
		}
		path, err := conversionStore().TrialFile(r.Context(), id, role == "source")
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		http.ServeFile(w, r, path)
		return
	}
	if r.URL.Query().Get("config") == "1" {
		config, err := conversionStore().Config()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeVisualSimilarityJSON(w, map[string]interface{}{"config": config, "inputFormats": mediaconvert.InputFormats, "outputFormats": mediaconvert.OutputFormats})
		return
	}
	if r.URL.Query().Get("capabilities") == "1" {
		client, capabilities, notice, err := conversionClient(r.Context(), r.URL.Query().Get("backend"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeVisualSimilarityJSON(w, map[string]interface{}{"formats": capabilities.Formats, "upscalers": capabilities.Upscalers, "backend": conversionBackend(client), "notice": notice})
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
	id, _ := strconv.Atoi(r.URL.Query().Get("jobID"))
	jobStatus := manager.GetInstance().JobManager.GetJob(id)
	activeJob := conversionJobs.snapshot(id, r.URL.Query().Get("batch"), jobStatus)
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
	trials, err := s.Trials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	trialStats := mediaconvert.SummarizeTrials(trials)
	trialTotal := len(trials)
	for left, right := 0, len(trials)-1; left < right; left, right = left+1, right-1 {
		trials[left], trials[right] = trials[right], trials[left]
	}
	trialOffset, _ := strconv.Atoi(r.URL.Query().Get("trialOffset"))
	trialOffset = max(0, min(trialOffset, trialTotal))
	trials = trials[trialOffset:min(trialOffset+50, trialTotal)]
	writeVisualSimilarityJSON(w, map[string]interface{}{"config": config, "history": history, "historyTotal": historyTotal, "stats": stats, "job": activeJob, "jobUnavailable": id > 0 && activeJob == nil, "batchStats": batchStats, "latestStats": latestStats, "trials": trials, "trialStats": trialStats, "trialTotal": trialTotal})
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
	if request.Savings != nil {
		if err := request.Savings.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if request.TrialCacheLimitBytes != nil && *request.TrialCacheLimitBytes < 0 {
		http.Error(w, "trial cache cannot be negative", http.StatusBadRequest)
		return
	}
	if request.TrialTTLHours != nil && (*request.TrialTTLHours < 1 || *request.TrialTTLHours > 168) {
		http.Error(w, "trial expiry must be 1–168 hours", http.StatusBadRequest)
		return
	}
	if request.Action == "cancel" {
		if !conversionJobs.cancel(request.JobID, request.JobBatch, mgr.JobManager.CancelJob) {
			http.Error(w, "This live job is no longer available. Refresh job status; saved records are unchanged.", http.StatusConflict)
			return
		}
		writeVisualSimilarityJSON(w, map[string]bool{"cancelled": true})
		return
	}
	if request.Action == "inspect-animations" {
		id, err := mgr.Scan(r.Context(), manager.ScanMetadataInput{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeVisualSimilarityJSON(w, map[string]int{"jobID": id})
		return
	}
	if request.Action == "save-defaults" {
		if request.FormatDefaults == nil {
			http.Error(w, "format defaults are required", http.StatusBadRequest)
			return
		}
		if err := mediaconvert.ValidateFormatDefaults(request.FormatDefaults); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mediaconvert.ValidateFormatRulesEnabled(request.FormatRulesEnabled, request.FormatDefaults); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mediaconvert.ValidateEncodingDefaults(request.EncodingDefaults, request.Backend); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		conversionSettings.Lock()
		defer conversionSettings.Unlock()
		config, err := conversionStore().Config()
		if err == nil {
			config.FormatDefaults = request.FormatDefaults
			if request.FormatRulesEnabled != nil {
				config.FormatRulesEnabled = request.FormatRulesEnabled
			} else {
				// Older clients still replace the rule list; drop flags for removed rules.
				for input := range config.FormatRulesEnabled {
					if _, ok := request.FormatDefaults[input]; !ok {
						delete(config.FormatRulesEnabled, input)
					}
				}
			}
			if request.EncodingDefaults != nil {
				config.EncodingDefaults = request.EncodingDefaults
			}
			if request.Backend != "" {
				config.Backend = request.Backend
			}
			if request.Savings != nil {
				config.Savings = *request.Savings
			}
			if request.TrialCacheLimitBytes != nil {
				config.TrialCacheLimitBytes = *request.TrialCacheLimitBytes
			}
			if request.TrialTTLHours != nil {
				config.TrialTTLHours = *request.TrialTTLHours
			}
			err = conversionStore().Configure(config)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeVisualSimilarityJSON(w, config)
		return
	}
	if isConversionReviewAction(request.Action) {
		startConversionReview(w, r, request)
		return
	}
	if request.Action != "start" && request.Action != "restore" && request.Action != "restore-targets" && request.Action != "restore-status" && request.Action != "configure" && request.Action != "recover" && request.Action != "preview" {
		http.Error(w, "unknown conversion action", http.StatusBadRequest)
		return
	}
	if (request.Action == "start" || request.Action == "preview" || request.Action == "restore-targets" || request.Action == "restore-status") && (len(request.Targets) == 0 || len(request.Targets) > 10000) {
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
	if request.Action == "restore-status" {
		store := conversionStore()
		history, err := store.History()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type targetRestoreStatus struct {
			Target     conversionTarget `json:"target"`
			Restorable bool             `json:"restorable"`
		}
		statuses := make([]targetRestoreStatus, 0, len(request.Targets))
		for _, target := range request.Targets {
			status := targetRestoreStatus{Target: target}
			fileID, err := conversionTargetFile(r.Context(), target)
			if err == nil {
				current, lookupErr := store.Repository.Get(r.Context(), fileID)
				if lookupErr == nil && current != nil {
					_, err = latestRestorableConversion(history, fileID, current.Base().Path)
					status.Restorable = err == nil
				}
			}
			statuses = append(statuses, status)
		}
		writeVisualSimilarityJSON(w, map[string]interface{}{"targets": statuses})
		return
	}
	if request.Action == "preview" {
		previewConversionDefaults(w, r, request.Targets)
		return
	}
	if request.UseQuality && (request.Options.Quality < 0 || request.Options.Quality > 100) {
		http.Error(w, "quality must be between 0 and 100", http.StatusBadRequest)
		return
	}
	if request.CacheLimitBytes < 0 {
		http.Error(w, "cache size cannot be negative", http.StatusBadRequest)
		return
	}
	state := &conversionJob{Batch: mediaconvert.NewID(), Status: "queued", Total: len(request.Targets), Action: request.Action, Items: []conversionItem{}}
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
			conversionSettings.Lock()
			config, err := s.Config()
			if err == nil {
				config.CacheLimitBytes = request.CacheLimitBytes
				err = s.Configure(config)
			}
			conversionSettings.Unlock()
			if err != nil {
				return err
			}
			return s.Trim()
		case "restore-targets":
			history, err := s.History()
			if err != nil {
				return err
			}
			progress.SetTotal(len(request.Targets))
			var restored []models.FileID
			defer func() {
				if len(restored) > 0 {
					_ = generateConvertedMedia(context.WithoutCancel(ctx), restored)
				}
			}()
			for _, target := range request.Targets {
				if err := ctx.Err(); err != nil {
					return err
				}
				item := conversionItem{Target: target, Status: "failed"}
				fileID, itemErr := conversionTargetFile(ctx, target)
				if itemErr == nil {
					var current models.File
					current, itemErr = s.Repository.Get(ctx, fileID)
					if itemErr == nil && current == nil {
						itemErr = fmt.Errorf("selected media file no longer exists")
					}
					if itemErr == nil {
						var recordID string
						recordID, itemErr = latestRestorableConversion(history, fileID, current.Base().Path)
						if itemErr == nil {
							item.RecordID = recordID
							itemErr = s.Restore(ctx, recordID)
							if itemErr == nil {
								item.Status = "restored"
								restored = append(restored, fileID)
							}
						}
					}
				}
				if itemErr != nil {
					item.Error = itemErr.Error()
				}
				conversionJobs.Lock()
				state.Items = append(state.Items, item)
				conversionJobs.Unlock()
				progress.Increment()
			}
			return nil
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
		config, err := s.Config()
		if err != nil {
			return err
		}
		var sources []models.File
		for _, target := range request.Targets {
			if err := ctx.Err(); err != nil {
				return err
			}
			id, err := conversionTargetFile(ctx, target)
			if err != nil {
				continue // The item loop reports lookup failures individually.
			}
			source, err := s.Repository.Get(ctx, id)
			if err == nil && source != nil {
				sources = append(sources, source)
			}
		}
		reservation, err := s.Preflight(sources, true)
		if err != nil {
			return err
		}
		conversionJobs.Lock()
		state.Reservation = &reservation
		conversionJobs.Unlock()
		backend := request.Backend
		if backend == "" {
			backend = config.Backend
		}
		client, capabilities, notice, err := conversionClient(ctx, backend)
		if err != nil {
			return err
		}
		conversionJobs.Lock()
		state.Backend, state.Notice = conversionBackend(client), notice
		conversionJobs.Unlock()
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
			id, err := conversionTargetFile(ctx, target)
			if err == nil && seen[id] {
				progress.Increment()
				continue
			}
			if err == nil {
				seen[id] = true
				options, format, resolveErr := resolveConversionOptions(ctx, s, config, id, target, request, capabilities)
				err = resolveErr
				if err == nil {
					policy := config.Savings
					if request.Savings != nil {
						policy = *request.Savings
					}
					record, convertErr := s.ConvertWithSavings(ctx, id, state.Batch, client, options, format, policy)
					err = convertErr
					if record != nil {
						item.RecordID = record.ID
						item.Status = record.Status
						if record.Status == "complete" {
							converted = append(converted, id)
						}
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
	writeVisualSimilarityJSON(w, map[string]interface{}{"jobID": jobID, "batch": state.Batch})
}

func resolveConversionOptions(ctx context.Context, s mediaconvert.Store, config mediaconvert.Config, id models.FileID, target conversionTarget, request conversionRequest, capabilities mediaconvert.Capabilities) (mediaconvert.Options, mediaconvert.Format, error) {
	options := request.Options
	var err error
	var input string
	if options.Format == "" || options.Format == "auto" {
		options.Format, input, err = conversionDefaultForFile(ctx, s, config, id)
		if err == nil && options.Format == "" {
			err = fmt.Errorf("automatic conversion is disabled for %s; enable its rule or choose an output format", input)
		}
	} else if request.UseEncodingDefaults || conversionUsesDecodingSpeedDefaults(request) {
		_, input, err = conversionDefaultForFile(ctx, s, config, id)
	}
	defaults := config.DefaultEncoding(input)
	if request.UseEncodingDefaults {
		options.Quality, options.Effort = defaults.Quality, defaults.Effort
		options.AllowLarger = defaults.AllowLarger
		options.Lossless = defaults.Lossless
		options.DropAudio = defaults.DropAudio
		options.AllowAlphaLoss = defaults.AllowAlphaLoss
	}
	if conversionUsesDecodingSpeedDefaults(request) {
		options.DecodingSpeed = defaults.DecodingSpeed
		options.FasterDecoding = nil
	}
	if options.Hardware == "" {
		options.Hardware = "auto"
	}
	if (request.UseQuality || request.UseEncodingDefaults) && (options.Format == "jxl" || options.Format == "ajxl") {
		options.Distance = mediaconvert.JXLDistanceFromQuality(options.Quality)
	}
	var format mediaconvert.Format
	if err == nil {
		format, err = conversionOutputFormat(capabilities, options.Format, target.Kind)
	}
	if err == nil {
		if !formatSupportsControl(format, "lossless") {
			options.Lossless = false
		}
		if options.DecodingSpeed == nil {
			options.DecodingSpeed = options.FasterDecoding
		}
		if options.DecodingSpeed == nil {
			options.DecodingSpeed = defaultConversionDecodingSpeed(format)
		}
		switch {
		case formatSupportsControl(format, "decodingSpeed"):
			options.FasterDecoding = nil
		case formatSupportsControl(format, "fasterDecoding"):
			// Keep the old wire name for remote workers that predate
			// the generic decodingSpeed option.
			options.FasterDecoding = options.DecodingSpeed
			options.DecodingSpeed = nil
		default:
			// Older workers may not have a compatible encoder control.
			// Omit it so the existing worker protocol remains compatible.
			options.DecodingSpeed = nil
			options.FasterDecoding = nil
		}
		if options.DecodingSpeed != nil && formatDecodingSpeedLevels(format) == 1 && *options.DecodingSpeed > 0 {
			// The UI's generic range maps to AV1's codec-specific on/off
			// control when a mixed batch uses one shared override.
			value := 1
			options.DecodingSpeed = &value
		}
	}
	return options, format, err
}

func conversionBackend(client mediaconvert.Client) string {
	if client.URL != "" {
		return "remote"
	}
	return "local"
}

func conversionDefaultForFile(ctx context.Context, s mediaconvert.Store, config mediaconvert.Config, id models.FileID) (output, input string, err error) {
	f, err := s.Repository.Get(ctx, id)
	if err != nil {
		return "", "", err
	}
	if f == nil {
		return "", "", fmt.Errorf("media file not found")
	}
	input = mediaconvert.SourceFormat(f)
	_, video := f.(*models.VideoFile)
	return config.DefaultOutput(input, video), input, nil
}

func conversionOutputFormat(capabilities mediaconvert.Capabilities, id, kind string) (mediaconvert.Format, error) {
	for _, format := range capabilities.Formats {
		if format.ID != id || !format.Available {
			continue
		}
		for _, expected := range mediaconvert.OutputFormats {
			if expected.ID == id && expected.Extension == format.Extension && expected.Family == format.Family {
				if kind == "scene" && format.Family != "video" {
					return format, fmt.Errorf("video entries require a video output format")
				}
				return format, nil
			}
		}
		return format, fmt.Errorf("invalid output format returned by worker")
	}
	return mediaconvert.Format{}, fmt.Errorf("output format %s is unavailable on the selected worker", id)
}

func formatSupportsControl(format mediaconvert.Format, control string) bool {
	for _, available := range format.Controls {
		if available == control {
			return true
		}
	}
	return false
}

func formatDecodingSpeedLevels(format mediaconvert.Format) int {
	if format.DecodingSpeedLevels > 0 {
		return format.DecodingSpeedLevels
	}
	if (format.ID == "jxl" || format.ID == "ajxl") &&
		(formatSupportsControl(format, "decodingSpeed") || formatSupportsControl(format, "fasterDecoding")) {
		return 4
	}
	return 0
}

func conversionUsesDecodingSpeedDefaults(request conversionRequest) bool {
	if request.UseDecodingSpeedDefaults != nil {
		return *request.UseDecodingSpeedDefaults
	}
	// Preserve the old combined setting for clients that have not yet sent the
	// separate decode-speed preference.
	return request.UseEncodingDefaults
}

func defaultConversionDecodingSpeed(format mediaconvert.Format) *int {
	value := 0
	if formatDecodingSpeedLevels(format) > 1 {
		value = 2
	}
	return &value
}

func previewConversionDefaults(w http.ResponseWriter, r *http.Request, targets []conversionTarget) {
	s := conversionStore()
	config, err := s.Config()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type plan struct {
		Input          string  `json:"input"`
		Output         string  `json:"output"`
		Count          int     `json:"count"`
		Error          string  `json:"error,omitempty"`
		Quality        float64 `json:"quality"`
		Effort         int     `json:"effort"`
		DecodingSpeed  int     `json:"decodingSpeed"`
		AllowLarger    bool    `json:"allowLarger,omitempty"`
		Lossless       bool    `json:"lossless,omitempty"`
		DropAudio      bool    `json:"dropAudio,omitempty"`
		AllowAlphaLoss bool    `json:"allowAlphaLoss,omitempty"`
	}
	plans := []plan{}
	indices := map[plan]int{}
	for _, target := range targets {
		if r.Context().Err() != nil {
			return
		}
		var next plan
		id, err := conversionTargetFile(r.Context(), target)
		if err == nil {
			next.Output, next.Input, err = conversionDefaultForFile(r.Context(), s, config, id)
			defaults := config.DefaultEncoding(next.Input)
			next.Quality, next.Effort, next.DecodingSpeed = defaults.Quality, defaults.Effort, defaults.DecodingSpeedValue()
			next.AllowLarger, next.Lossless = defaults.AllowLarger, defaults.Lossless
			next.DropAudio, next.AllowAlphaLoss = defaults.DropAudio, defaults.AllowAlphaLoss
		}
		if err != nil {
			next.Error = err.Error()
		}
		index, exists := indices[next]
		if !exists {
			index = len(plans)
			indices[next] = index
			plans = append(plans, next)
		}
		plans[index].Count++
	}
	writeVisualSimilarityJSON(w, map[string]interface{}{"plans": plans})
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
