package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

type identityScanState struct {
	JobID        int    `json:"jobID"`
	Status       string `json:"status"`
	Scanned      int    `json:"scanned"`
	Observations int    `json:"observations"`
	Skipped      int    `json:"skipped"`
	Error        string `json:"error,omitempty"`
}

// Only the latest status per library is retained. Restarting loses job status,
// not observations or decisions; a repeat scan is idempotent.
var identityScans = struct {
	sync.Mutex
	states map[string]*identityScanState
}{states: map[string]*identityScanState{}}

// Called with identityScans locked; GetJob returns a synchronized snapshot.
// Native job-queue cancellation can bypass the scan's own cancel endpoint.
func (rs booruBrowserRoutes) reconcileIdentityScan(state *identityScanState) {
	if state == nil || rs.jobs == nil || (state.Status != "queued" && state.Status != "running") {
		return
	}
	current := rs.jobs.GetJob(state.JobID)
	switch {
	case current == nil:
		state.Status, state.Error = "failed", "Scan is no longer in the job queue. Repeat the scan; saved metadata remains."
	case current.Status == job.StatusCancelled || current.Status == job.StatusStopping:
		state.Status, state.Error = "cancelled", "Scan cancelled in the job queue. Saved observations remain."
	case current.Status == job.StatusFailed:
		state.Status, state.Error = "failed", "Scan job failed. Repeat the scan; saved metadata remains."
	}
}

func (rs booruBrowserRoutes) IdentityScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		identityScans.Lock()
		state := identityScanState{Status: "idle"}
		if current := identityScans.states[rs.configPath]; current != nil {
			rs.reconcileIdentityScan(current)
			state = *current
		}
		identityScans.Unlock()
		writeVisualSimilarityJSON(w, state)
		return
	}
	var input struct {
		Action string `json:"action"`
		JobID  int    `json:"jobID"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if rs.jobs == nil || rs.repository.ProviderIdentity == nil {
		writeBooruError(w, 503, "identities", "Identity scan jobs are unavailable.")
		return
	}
	identityScans.Lock()
	current := identityScans.states[rs.configPath]
	rs.reconcileIdentityScan(current)
	if input.Action == "cancel" {
		if current == nil || current.JobID != input.JobID || (current.Status != "queued" && current.Status != "running") {
			identityScans.Unlock()
			identityError(w, models.ErrIdentityConflict)
			return
		}
		// Queued jobs may never execute; mark cancellation here as well.
		current.Status = "cancelled"
		state := *current
		identityScans.Unlock()
		rs.jobs.CancelJob(input.JobID)
		writeVisualSimilarityJSON(w, state)
		return
	}
	if input.Action != "start" {
		identityScans.Unlock()
		identityError(w, models.ErrIdentityInvalid)
		return
	}
	if current != nil && (current.Status == "queued" || current.Status == "running") {
		state := *current
		identityScans.Unlock()
		writeVisualSimilarityJSON(w, state)
		return
	}
	state := &identityScanState{Status: "queued"}
	identityScans.states[rs.configPath] = state
	state.JobID = rs.jobs.Add(r.Context(), "Scan saved provider identities", job.MakeJobExec(func(ctx context.Context, _ *job.Progress) error {
		identityScans.Lock()
		if state.Status == "cancelled" {
			identityScans.Unlock()
			return nil
		}
		state.Status = "running"
		identityScans.Unlock()
		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		err := rs.scanIdentityMetadata(ctx, func(scanned, observations, skipped int) {
			identityScans.Lock()
			state.Scanned, state.Observations, state.Skipped = scanned, observations, skipped
			identityScans.Unlock()
		})
		identityScans.Lock()
		defer identityScans.Unlock()
		switch {
		case state.Status == "cancelled" || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			state.Status, state.Error = "cancelled", "Scan stopped. Saved observations remain; repeat the scan to continue."
		case errors.Is(err, models.ErrIdentityLimit):
			state.Status, state.Error = "failed", "Identity cache is full. Discard unreviewed metadata and retry; connections and decisions remain."
		case err != nil:
			state.Status, state.Error = "failed", "Scan could not finish: "+err.Error()
		default:
			state.Status = "completed"
		}
		return err
	}))
	result := *state
	identityScans.Unlock()
	writeVisualSimilarityJSON(w, result)
}

func (rs booruBrowserRoutes) scanIdentityMetadata(ctx context.Context, progress func(int, int, int)) error {
	scanned, observations, skipped := 0, 0, 0
	sources := map[string]booru.Source{}
	if configured, err := rs.sources(); err == nil {
		for _, source := range configured {
			sources[source.Instance()] = source
		}
	}
	if rs.repository.BooruFollow != nil {
		if err := rs.repository.WithReadTxn(ctx, func(ctx context.Context) error {
			follows, err := rs.repository.BooruFollow.List(ctx)
			for _, follow := range follows {
				source := booru.Source{ID: follow.SourceID, Name: follow.Name, Engine: follow.Engine, URL: follow.SourceURL}
				if source.Validate() == nil && source.Instance() == follow.Instance {
					sources[follow.Instance] = source
				}
			}
			return err
		}); err != nil {
			return err
		}
	}
	observe := func(source booru.Source, post booru.Post) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		input, _ := identityPostObservations(source, []booru.Post{post})
		if len(input) == 0 {
			skipped++
			return nil
		}
		err := rs.repository.WithTxn(ctx, func(ctx context.Context) error {
			_, err := rs.repository.ProviderIdentity.Observe(ctx, input)
			return err
		})
		if errors.Is(err, models.ErrIdentityInvalid) {
			skipped++
			return nil
		}
		if err == nil {
			observations += len(input)
		}
		return err
	}
	if rs.repository.Following != nil {
		before := int64(0)
		for {
			var posts []models.FollowingPost
			err := rs.repository.WithReadTxn(ctx, func(ctx context.Context) error {
				var err error
				posts, err = rs.repository.Following.List(ctx, models.FollowingFilter{Before: before, Limit: 50, History: true, ShowIgnored: true})
				return err
			})
			if err != nil {
				return err
			}
			for _, post := range posts {
				scanned++
				var payload followingStoredPost
				if source, ok := sources[post.Instance]; ok && json.Unmarshal(post.Payload, &payload) == nil {
					if err := observe(source, payload.Post); err != nil {
						progress(scanned, observations, skipped)
						return err
					}
				} else {
					skipped++
				}
				before = post.Sequence
				progress(scanned, observations, skipped)
			}
			if len(posts) < 50 {
				break
			}
		}
	}
	directory := filepath.Join(rs.configPath, "booru-download-history")
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		progress(scanned, observations, skipped)
		return nil
	}
	if err != nil {
		return errors.New("saved receipt directory is unavailable")
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("receipt directory must be a regular directory")
	}
	dir, err := os.Open(directory)
	if err != nil {
		return errors.New("saved receipt directory is unavailable")
	}
	defer dir.Close()
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		files, err := dir.ReadDir(50)
		if err != nil && !errors.Is(err, io.EOF) {
			return errors.New("cannot enumerate saved receipts")
		}
		for _, file := range files {
			entries++
			if entries > 100000 {
				return errors.New("receipt directory exceeds 100,000 entries; already scanned metadata remains")
			}
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			scanned++
			receipt, ok := readIdentityReceipt(filepath.Join(directory, file.Name()))
			source := booru.Source{ID: receipt.Source.ID, Name: receipt.Source.Name, Engine: receipt.Source.Engine, URL: receipt.Source.URL}
			if !ok || source.Validate() != nil || source.Instance() != receipt.Source.Instance {
				skipped++
			} else {
				if err := observe(source, receipt.Post); err != nil {
					progress(scanned, observations, skipped)
					return err
				}
			}
			progress(scanned, observations, skipped)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	progress(scanned, observations, skipped)
	return nil
}

func readIdentityReceipt(path string) (booruDownloadReceipt, bool) {
	var receipt booruDownloadReceipt
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 64<<10 {
		return receipt, false
	}
	file, err := os.Open(path)
	if err != nil {
		return receipt, false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return receipt, false
	}
	data, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	after, statErr := file.Stat()
	if err != nil || statErr != nil || len(data) > 64<<10 || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return receipt, false
	}
	if json.Unmarshal(data, &receipt) != nil {
		return receipt, false
	}
	return receipt, true
}
