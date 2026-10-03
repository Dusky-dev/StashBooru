package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
)

func isConversionReviewAction(action string) bool {
	return action == "estimate" || action == "trial" || action == "apply-trials" || action == "discard-trials"
}

func conversionTargetKey(target conversionTarget) string {
	return fmt.Sprintf("%s:%d", target.Kind, target.ID)
}

func startConversionReview(w http.ResponseWriter, r *http.Request, request conversionRequest) {
	if request.Options.Upscaler != "" {
		http.Error(w, "use the separate image upscaler", http.StatusBadRequest)
		return
	}
	if request.Action == "estimate" || request.Action == "trial" {
		if len(request.Targets) < 1 || len(request.Targets) > 10000 {
			http.Error(w, "select between 1 and 10000 media entries", http.StatusBadRequest)
			return
		}
		seen := map[conversionTarget]bool{}
		unique := []conversionTarget{}
		for _, target := range request.Targets {
			if target.ID <= 0 || (target.Kind != "image" && target.Kind != "scene") {
				http.Error(w, "invalid media target", http.StatusBadRequest)
				return
			}
			if !seen[target] {
				unique = append(unique, target)
				seen[target] = true
			}
		}
		request.Targets = unique
	} else {
		if len(request.TrialIDs) < 1 || len(request.TrialIDs) > 10000 {
			http.Error(w, "select between 1 and 10000 trials", http.StatusBadRequest)
			return
		}
		seen := map[string]bool{}
		unique := []string{}
		for _, id := range request.TrialIDs {
			// Loading verifies IDs and canonical manifest paths before queuing.
			if _, err := conversionStore().TrialRecord(id); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if !seen[id] {
				unique = append(unique, id)
				seen[id] = true
			}
		}
		request.TrialIDs = unique
	}
	state := &conversionJob{Batch: mediaconvert.NewID(), Status: "queued", Total: len(request.Targets), Action: request.Action, Items: []conversionItem{}}
	if len(request.TrialIDs) > 0 {
		state.Total = len(request.TrialIDs)
	}
	setState := func(status string, err error) {
		conversionJobs.Lock()
		defer conversionJobs.Unlock()
		state.Status = status
		if err != nil {
			state.Error = err.Error()
		}
	}
	addItem := func(item conversionItem) {
		conversionJobs.Lock()
		defer conversionJobs.Unlock()
		state.Items = append(state.Items, item)
	}
	mgr := manager.GetInstance()
	id := mgr.JobManager.Add(r.Context(), "Media converter: "+request.Action, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) (retErr error) {
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
		if request.Action == "discard-trials" {
			progress.SetTotal(len(request.TrialIDs))
			for _, id := range request.TrialIDs {
				if err := ctx.Err(); err != nil {
					return err
				}
				item := conversionItem{TrialID: id, Status: "discarded"}
				if err := s.DiscardTrial(id); err != nil {
					item.Error, item.Status = err.Error(), "failed"
				}
				addItem(item)
				progress.Increment()
			}
			return nil
		}
		config, err := s.Config()
		if err != nil {
			return err
		}
		policy := config.Savings
		if request.Savings != nil {
			policy = *request.Savings
		}
		client, caps, notice, err := conversionClient(ctx, request.Backend)
		if err != nil {
			return err
		}
		if (request.Action == "trial" || request.Action == "apply-trials") && caps.Signature == "" {
			return fmt.Errorf("saved trials require a versioned converter: update scripts/media_conversion_worker.py on the selected worker and restart it")
		}
		conversionJobs.Lock()
		state.Backend, state.Notice = conversionBackend(client), notice
		conversionJobs.Unlock()
		if request.Action == "apply-trials" {
			progress.SetTotal(len(request.TrialIDs))
			var activated []models.FileID
			defer func() {
				if len(activated) > 0 {
					_ = generateConvertedMedia(context.WithoutCancel(ctx), activated)
				}
			}()
			var sources []models.File
			for _, id := range request.TrialIDs {
				t, err := s.TrialRecord(id)
				if err != nil {
					return err
				}
				sources = append(sources, t.Before.File())
			}
			reservation, err := s.Preflight(sources, true)
			if err != nil {
				return err
			}
			conversionJobs.Lock()
			state.Reservation = &reservation
			conversionJobs.Unlock()
			for _, id := range request.TrialIDs {
				if err := ctx.Err(); err != nil {
					return err
				}
				item := conversionItem{TrialID: id, Status: "failed"}
				t, err := s.TrialRecord(id)
				if err == nil {
					target := conversionTarget{Kind: t.Target.Kind, ID: t.Target.ID}
					item.Target = target
					fileID, lookupErr := conversionTargetFile(ctx, target)
					err = lookupErr
					if err == nil && fileID != t.Before.File().Base().ID {
						err = fmt.Errorf("primary file changed; run a new trial")
					}
					if err == nil {
						options, _, resolveErr := resolveConversionOptions(ctx, s, config, fileID, target, request, caps)
						err = resolveErr
						if err == nil {
							record, applyErr := s.ApplyTrial(ctx, id, state.Batch, client, options, policy, caps.Signature)
							err = applyErr
							if record != nil {
								item.RecordID, item.Status = record.ID, record.Status
								if record.Status == "complete" {
									activated = append(activated, fileID)
								}
							}
						}
					}
				}
				if err != nil {
					item.Error = err.Error()
				}
				addItem(item)
				progress.Increment()
			}
			return nil
		}
		type sourceEntry struct {
			target conversionTarget
			file   models.File
		}
		entries := map[string]sourceEntry{}
		candidates := []mediaconvert.SampleCandidate{}
		sources := []models.File{}
		for _, target := range request.Targets {
			if err := ctx.Err(); err != nil {
				return err
			}
			fileID, err := conversionTargetFile(ctx, target)
			var file models.File
			if err == nil {
				file, err = s.Repository.Get(ctx, fileID)
				if file == nil && err == nil {
					err = fmt.Errorf("media file not found")
				}
			}
			if err != nil {
				addItem(conversionItem{Target: target, Status: "failed", Error: err.Error()})
				continue
			}
			key := conversionTargetKey(target)
			entries[key] = sourceEntry{target: target, file: file}
			sources = append(sources, file)
			candidates = append(candidates, mediaconvert.SampleCandidate{Key: key, Input: mediaconvert.SourceFormat(file), Animated: file.Base().FrameCount > 1, AnimationUnknown: file.Base().FrameCount == 0, Size: file.Base().Size})
		}
		selected := candidates
		if request.Action == "estimate" {
			selected = mediaconvert.SelectEstimateSample(candidates, 24)
			sources = nil
			for _, c := range selected {
				sources = append(sources, entries[c.Key].file)
			}
		}
		reservation, err := s.Preflight(sources, false)
		if err != nil {
			return err
		}
		conversionJobs.Lock()
		state.Reservation = &reservation
		state.Total = len(selected) + len(state.Items)
		conversionJobs.Unlock()
		progress.SetTotal(len(selected))
		measurements := []mediaconvert.EstimateMeasurement{}
		for _, candidate := range selected {
			if err := ctx.Err(); err != nil {
				return err
			}
			entry := entries[candidate.Key]
			item := conversionItem{Target: entry.target, Status: "failed"}
			options, format, err := resolveConversionOptions(ctx, s, config, entry.file.Base().ID, entry.target, request, caps)
			measurement := mediaconvert.EstimateMeasurement{Candidate: candidate}
			if err == nil {
				trial, trialErr := s.CreateTrial(ctx, entry.file.Base().ID, state.Batch, mediaconvert.TrialTarget{Kind: entry.target.Kind, ID: entry.target.ID}, client, options, format, policy, caps.Signature, request.Action == "trial")
				err = trialErr
				if trial != nil {
					item.Status = trial.Status
					if request.Action == "trial" {
						item.TrialID = trial.ID
					}
					measurement.OutputBytes, measurement.Eligible = trial.Result.Size, trial.Status == "verified"
				}
			}
			if err != nil {
				item.Error, measurement.Error = err.Error(), err.Error()
			}
			measurements = append(measurements, measurement)
			addItem(item)
			progress.Increment()
		}
		if request.Action == "estimate" {
			estimate := mediaconvert.SummarizeEstimate(candidates, measurements)
			if len(candidates) != len(request.Targets) {
				estimate.Covered = false
				estimate.EstimatedSavedBytes, estimate.ObservedLowBytes, estimate.ObservedHighBytes = 0, 0, 0
			}
			conversionJobs.Lock()
			state.Estimate = &estimate
			conversionJobs.Unlock()
		}
		return nil
	}))
	conversionJobs.Lock()
	state.JobID = id
	conversionJobs.jobs[id] = state
	conversionJobs.Unlock()
	time.AfterFunc(24*time.Hour, func() {
		conversionJobs.Lock()
		defer conversionJobs.Unlock()
		if state.Status != "queued" && state.Status != "running" {
			delete(conversionJobs.jobs, id)
		}
	})
	writeVisualSimilarityJSON(w, map[string]int{"jobID": id})
}
