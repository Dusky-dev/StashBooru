package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

const (
	associationInheritanceReviewTTL   = 30 * time.Minute
	associationInheritanceReviewLimit = 8
)

type associationInheritanceInput struct {
	Characters *bool `json:"characters"`
	Artists    *bool `json:"artists"`
	Copyrights *bool `json:"copyrights"`
	Tags       *bool `json:"tags"`
}

type associationInheritanceRequest struct {
	Action   string                       `json:"action"`
	ReviewID string                       `json:"reviewID"`
	Proposed *associationInheritanceInput `json:"proposed"`
}

type associationInheritanceResponse struct {
	ReviewID  string                                `json:"reviewID"`
	JobID     int                                   `json:"jobID"`
	Status    string                                `json:"status"`
	Current   config.AssociationInheritanceSettings `json:"current"`
	Proposed  config.AssociationInheritanceSettings `json:"proposed"`
	Processed int                                   `json:"processed"`
	Total     int                                   `json:"total"`
	Report    *associationInheritanceReport         `json:"report,omitempty"`
	Error     string                                `json:"error,omitempty"`
}

type associationInheritanceState struct {
	mu       sync.Mutex
	response associationInheritanceResponse
	updated  time.Time
}

type associationInheritanceRoutes struct {
	repository models.Repository
	config     *config.Config
	jobs       *job.Manager
	mu         sync.Mutex
	states     map[string]*associationInheritanceState
}

func newAssociationInheritanceRoutes(repository models.Repository, cfg *config.Config, jobs *job.Manager) *associationInheritanceRoutes {
	return &associationInheritanceRoutes{repository: repository, config: cfg, jobs: jobs, states: make(map[string]*associationInheritanceState)}
}

func associationInheritanceActive(status string) bool {
	switch status {
	case "queued_preview", "previewing", "queued_apply", "applying", "cancelling":
		return true
	}
	return false
}

func (rs *associationInheritanceRoutes) normalizeState(state *associationInheritanceState) {
	// Caller holds the state lock. Native Jobs UI can cancel a queued review
	// before its executor starts, so synchronize that terminal state here.
	if !associationInheritanceActive(state.response.Status) || state.response.JobID == 0 {
		return
	}
	j := rs.jobs.GetJob(state.response.JobID)
	if j == nil || j.Status == job.StatusCancelled {
		state.response.Status = "cancelled"
		state.updated = time.Now()
	} else if j.Status == job.StatusFailed {
		state.response.Status = "failed"
		state.response.Error = "inheritance job failed"
		if j.Error != nil {
			state.response.Error = *j.Error
		}
		state.updated = time.Now()
	}
}

func (rs *associationInheritanceRoutes) prune() {
	// Caller holds the registry lock.
	for id, state := range rs.states {
		state.mu.Lock()
		rs.normalizeState(state)
		expired := !associationInheritanceActive(state.response.Status) && time.Since(state.updated) > associationInheritanceReviewTTL
		state.mu.Unlock()
		if expired {
			delete(rs.states, id)
		}
	}
}

func (rs *associationInheritanceRoutes) find(id string) *associationInheritanceState {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.prune()
	return rs.states[id]
}

func (rs *associationInheritanceRoutes) snapshot(state *associationInheritanceState) associationInheritanceResponse {
	state.mu.Lock()
	defer state.mu.Unlock()
	rs.normalizeState(state)
	return state.response
}

func (rs *associationInheritanceRoutes) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := r.URL.Query().Get("reviewID")
	if id == "" {
		writeVisualSimilarityJSON(w, map[string]config.AssociationInheritanceSettings{"current": rs.config.GetAssociationInheritanceSettings()})
		return
	}
	state := rs.find(id)
	if state == nil {
		http.Error(w, "inheritance review expired or was not found; run a new preview", http.StatusNotFound)
		return
	}
	writeVisualSimilarityJSON(w, rs.snapshot(state))
}

func (rs *associationInheritanceRoutes) Post(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request associationInheritanceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding inheritance review: %v", err), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		http.Error(w, "inheritance review must contain one JSON object", http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action == "preview" {
		rs.preview(w, r, request)
		return
	}
	state := rs.find(request.ReviewID)
	if state == nil {
		http.Error(w, "inheritance review expired or was not found; run a new preview", http.StatusNotFound)
		return
	}
	switch action {
	case "apply":
		state.mu.Lock()
		rs.normalizeState(state)
		if state.response.Status == "applied" {
			response := state.response
			state.mu.Unlock()
			writeVisualSimilarityJSON(w, response)
			return
		}
		if state.response.Status != "ready" {
			state.mu.Unlock()
			http.Error(w, "inheritance review must finish successfully before apply", http.StatusConflict)
			return
		}
		rs.queueLocked(r.Context(), state, true)
		state.mu.Unlock()
		writeVisualSimilarityJSON(w, rs.snapshot(state))
	case "cancel":
		state.mu.Lock()
		rs.normalizeState(state)
		if associationInheritanceActive(state.response.Status) {
			state.response.Status = "cancelling"
			state.updated = time.Now()
		}
		jobID := state.response.JobID
		state.mu.Unlock()
		rs.jobs.CancelJob(jobID)
		writeVisualSimilarityJSON(w, rs.snapshot(state))
	case "discard":
		rs.mu.Lock()
		state.mu.Lock()
		rs.normalizeState(state)
		active := associationInheritanceActive(state.response.Status)
		if !active {
			delete(rs.states, request.ReviewID)
		}
		state.mu.Unlock()
		rs.mu.Unlock()
		if active {
			http.Error(w, "cancel the inheritance job and wait for it to stop before discarding", http.StatusConflict)
			return
		}
		writeVisualSimilarityJSON(w, map[string]bool{"discarded": true})
	default:
		http.Error(w, "unknown inheritance review action", http.StatusBadRequest)
	}
}

func (rs *associationInheritanceRoutes) preview(w http.ResponseWriter, r *http.Request, request associationInheritanceRequest) {
	input := request.Proposed
	if input == nil || input.Characters == nil || input.Artists == nil || input.Copyrights == nil || input.Tags == nil {
		http.Error(w, "preview requires all four inheritance defaults", http.StatusBadRequest)
		return
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		http.Error(w, "could not create inheritance review", http.StatusInternalServerError)
		return
	}
	id := hex.EncodeToString(randomID[:])
	state := &associationInheritanceState{updated: time.Now(), response: associationInheritanceResponse{
		ReviewID: id,
		Current:  rs.config.GetAssociationInheritanceSettings(),
		Proposed: config.AssociationInheritanceSettings{
			Characters: *input.Characters, Artists: *input.Artists, Copyrights: *input.Copyrights, Tags: *input.Tags,
		},
	}}
	rs.mu.Lock()
	rs.prune()
	if len(rs.states) >= associationInheritanceReviewLimit {
		rs.mu.Unlock()
		http.Error(w, "too many inheritance reviews; cancel or discard an existing review", http.StatusTooManyRequests)
		return
	}
	rs.states[id] = state
	state.mu.Lock()
	rs.queueLocked(r.Context(), state, false)
	state.mu.Unlock()
	rs.mu.Unlock()
	writeVisualSimilarityJSON(w, rs.snapshot(state))
}

func (rs *associationInheritanceRoutes) queueLocked(ctx context.Context, state *associationInheritanceState, apply bool) {
	// Hold the state lock until JobID is assigned, including if the native
	// queue starts the executor immediately.
	state.response.Processed = 0
	state.response.Total = 0
	state.response.Error = ""
	state.updated = time.Now()
	description := "Preview Image and Video association inheritance"
	state.response.Status = "queued_preview"
	if apply {
		description = "Apply reviewed Image and Video association inheritance"
		state.response.Status = "queued_apply"
	}
	state.response.JobID = rs.jobs.Add(ctx, description, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		state.mu.Lock()
		if state.response.Status == "cancelling" || state.response.Status == "cancelled" {
			state.response.Status = "cancelled"
			state.updated = time.Now()
			state.mu.Unlock()
			return nil
		}
		state.response.Status = "previewing"
		if apply {
			state.response.Status = "applying"
		}
		current, proposed, reviewed := state.response.Current, state.response.Proposed, state.response.Report
		state.mu.Unlock()
		update := func(processed, total int) {
			state.mu.Lock()
			state.response.Processed, state.response.Total = processed, total
			state.mu.Unlock()
			if processed == 0 {
				progress.SetTotal(total)
			}
			progress.SetProcessed(processed)
		}
		var report *associationInheritanceReport
		var err error
		if apply {
			err = applyAssociationInheritanceReview(ctx, rs.repository, rs.config, reviewed, update)
		} else {
			err = rs.repository.WithReadTxn(ctx, func(ctx context.Context) error {
				report, err = scanAssociationInheritanceReview(ctx, rs.repository, current, proposed, update)
				return err
			})
			if err == nil && report.ErrorCount > 0 {
				err = fmt.Errorf("%d media items could not be reviewed", report.ErrorCount)
			}
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		state.updated = time.Now()
		if !apply {
			state.response.Report = report
		}
		switch {
		case apply && err == nil:
			state.response.Status = "applied"
		case errors.Is(err, context.Canceled) || ctx.Err() != nil || state.response.Status == "cancelling":
			state.response.Status = "cancelled"
			return nil
		case err != nil:
			state.response.Status = "failed"
			state.response.Error = err.Error()
		default:
			state.response.Status = "ready"
		}
		return err
	}))
}
