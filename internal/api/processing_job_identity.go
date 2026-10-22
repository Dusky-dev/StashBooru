package api

import (
	"sync"

	"github.com/stashapp/stash/pkg/job"
)

// Numeric queue IDs restart with the server. A saved client reference must also
// match the batch assigned when this processing job was submitted.
type processingJobRegistry struct {
	sync.Mutex
	jobs map[int]*conversionJob
}

func (r *processingJobRegistry) snapshot(id int, batch string, queueJob *job.Job) *conversionJob {
	r.Lock()
	defer r.Unlock()
	state := r.jobs[id]
	if state == nil || id <= 0 || batch == "" || state.Batch != batch {
		return nil
	}
	if queueJob != nil && queueJob.Status == job.StatusCancelled && state.Status == "queued" {
		state.Status = "cancelled"
	}
	jobCopy := *state
	jobCopy.Items = append([]conversionItem{}, state.Items...)
	return &jobCopy
}

func (r *processingJobRegistry) cancel(id int, batch string, cancel func(int)) bool {
	r.Lock()
	state := r.jobs[id]
	matches := state != nil && id > 0 && batch != "" && state.Batch == batch
	r.Unlock()
	if !matches {
		return false
	}
	// Do not hold the registry lock while entering the native queue manager.
	// Queue IDs are monotonic for the process lifetime, so expiry cannot retarget
	// the validated ID between this check and cancellation.
	cancel(id)
	r.Lock()
	defer r.Unlock()
	if state := r.jobs[id]; state != nil && state.Batch == batch && state.Status == "queued" {
		state.Status = "cancelled"
	}
	return true
}
