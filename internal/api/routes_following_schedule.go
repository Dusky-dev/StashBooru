package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

// Automatic follow checks: off by default. When on, a "Check followed sites"
// job runs whenever a follow's last check (or failed check) is older than the
// chosen interval. Manual Check new posts keeps working as before.

var followIntervals = []int{0, 1, 3, 6, 12, 24}

// A single follow's catch-up is bounded per run; the rest waits for the next.
const followRunPages = 25

type followScheduleSettings struct {
	IntervalHours int `json:"intervalHours"`
	// MarkSeenOnView marks posts seen once they have been shown, so the
	// next visit only lists newer posts. Off by default.
	MarkSeenOnView bool `json:"markSeenOnView,omitempty"`
}

type followScheduleRun struct {
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Checked    int    `json:"checked"`
	NewPosts   int    `json:"newPosts"`
	Failed     int    `json:"failed"`
	Note       string `json:"note,omitempty"`
}

type followScheduleState struct {
	running bool
	jobID   int
	lastRun *followScheduleRun
}

var followSchedules = struct {
	sync.Mutex
	states map[string]*followScheduleState
}{states: map[string]*followScheduleState{}}

func (rs booruBrowserRoutes) followSchedulePath() string {
	return filepath.Join(rs.configPath, "following-schedule.json")
}

func (rs booruBrowserRoutes) followSchedule() (followScheduleSettings, error) {
	var value followScheduleSettings
	data, err := os.ReadFile(rs.followSchedulePath())
	if errors.Is(err, os.ErrNotExist) {
		return value, nil
	}
	if err != nil || len(data) > 1024 || json.Unmarshal(data, &value) != nil || !slices.Contains(followIntervals, value.IntervalHours) {
		return followScheduleSettings{}, fmt.Errorf("cannot read following schedule")
	}
	return value, nil
}

// lastFollowAttempt is the later of a follow's last finished check and its
// last failed check; zero when never checked.
func lastFollowAttempt(view booruFollowView) time.Time {
	var last time.Time
	if view.Feed == nil {
		return last
	}
	for _, value := range []string{view.Feed.CheckedAt, view.Feed.ErrorAt} {
		if at, err := time.Parse(time.RFC3339Nano, value); err == nil && at.After(last) {
			last = at
		}
	}
	return last
}

// dueFollows picks follows to check now: active, checkable, and last tried
// longer ago than interval (or never, or with a check left unfinished).
func dueFollows(views []booruFollowView, now time.Time, interval time.Duration) []booruFollowView {
	due := []booruFollowView{}
	for _, view := range views {
		if !view.Active || !view.SyncSupported {
			continue
		}
		last := lastFollowAttempt(view)
		unfinished := view.Feed != nil && view.Feed.HasMore && view.Feed.ErrorAt == ""
		if last.IsZero() || unfinished || now.Sub(last) >= interval {
			due = append(due, view)
		}
	}
	sort.SliceStable(due, func(i, j int) bool { return lastFollowAttempt(due[i]).Before(lastFollowAttempt(due[j])) })
	return due
}

// nextFollowRun is when the next follow becomes due; zero if none can be.
func nextFollowRun(views []booruFollowView, interval time.Duration) time.Time {
	var next time.Time
	for _, view := range views {
		if !view.Active || !view.SyncSupported {
			continue
		}
		at := lastFollowAttempt(view).Add(interval)
		if next.IsZero() || at.Before(next) {
			next = at
		}
	}
	return next
}

// checkFollows checks each follow until it is caught up. A site that asks
// to slow down is skipped for the rest of the run; a full snapshot cache
// stops the run. Other failures are recorded on that follow only.
func (rs booruBrowserRoutes) checkFollows(ctx context.Context, follows []booruFollowView, progress func(done, total int)) followScheduleRun {
	run := followScheduleRun{StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	paused := map[string]bool{}
	notes := []string{}
	for i, follow := range follows {
		if ctx.Err() != nil {
			notes = append(notes, "Stopped before all follows were checked.")
			break
		}
		if progress != nil {
			progress(i, len(follows))
		}
		if paused[follow.Instance] {
			continue
		}
		run.Checked++
		version := follow.Version
		failed := false
		for range followRunPages {
			result, err := rs.syncFollowPage(ctx, follow.ID, version)
			if err != nil {
				var provider *booru.Error
				switch {
				case ctx.Err() != nil, errors.Is(err, models.ErrBooruFollowConflict):
					// Cancelled, or changed by a manual check or edit: skip.
				case errors.As(err, &provider) && provider.Code == "rate-limited":
					paused[follow.Instance] = true
					notes = append(notes, follow.SourceID+" asked to slow down; its other follows wait for the next run.")
					failed = true
				case errors.Is(err, models.ErrFollowingLimit):
					run.Failed++
					run.Note = "The feed cache is full. Mark posts seen and remove seen snapshots in Following → Manage; checks resume on the next run."
					run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
					return run
				default:
					failed = true
				}
				break
			}
			run.NewPosts += result.Matched
			if strings.Contains(result.Notice, "older new posts were skipped") {
				notes = append(notes, follow.Name+": more new posts than one check reads; older ones were skipped.")
			}
			if !result.HasMore {
				break
			}
		}
		if failed {
			run.Failed++
		}
	}
	if len(notes) > 0 {
		run.Note = notes[0]
		for _, note := range notes[1:] {
			run.Note += " " + note
		}
	}
	run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return run
}

// startFollowCheck queues one check job unless one is queued or running.
func (rs booruBrowserRoutes) startFollowCheck(ctx context.Context, follows []booruFollowView) bool {
	followSchedules.Lock()
	state := followSchedules.states[rs.configPath]
	if state == nil {
		state = &followScheduleState{}
		followSchedules.states[rs.configPath] = state
	}
	if state.running || len(follows) == 0 {
		followSchedules.Unlock()
		return false
	}
	state.running = true
	followSchedules.Unlock()
	work := func(ctx context.Context, progress *job.Progress) error {
		if progress != nil {
			progress.SetTotal(len(follows))
		}
		run := rs.checkFollows(ctx, follows, func(done, _ int) {
			if progress != nil {
				progress.SetProcessed(done)
			}
		})
		followSchedules.Lock()
		state.running, state.lastRun = false, &run
		followSchedules.Unlock()
		return nil
	}
	if rs.jobs == nil {
		go func() { _ = work(ctx, nil) }()
		return true
	}
	id := rs.jobs.Add(ctx, "Check followed sites", job.MakeJobExec(work))
	followSchedules.Lock()
	state.jobID = id
	followSchedules.Unlock()
	return true
}

// runFollowScheduler checks once a minute whether any follow is due. It
// runs for the life of the server.
func (rs booruBrowserRoutes) runFollowScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			rs.followSchedulerTick(ctx, now)
		}
	}
}

func (rs booruBrowserRoutes) followSchedulerTick(ctx context.Context, now time.Time) bool {
	settings, err := rs.followSchedule()
	if err != nil || settings.IntervalHours == 0 || rs.repository.BooruFollow == nil || rs.repository.Following == nil {
		return false
	}
	views, err := rs.followViews(ctx)
	if err != nil {
		return false
	}
	return rs.startFollowCheck(context.Background(), dueFollows(views, now, time.Duration(settings.IntervalHours)*time.Hour))
}

// FollowScheduleRoute reads or changes automatic checks, or starts a check
// of every follow now.
func (rs booruBrowserRoutes) FollowScheduleRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if rs.repository.BooruFollow == nil || rs.repository.Following == nil {
		writeBooruError(w, 503, "following", "Following storage is unavailable.")
		return
	}
	if r.Method == http.MethodPost {
		var input struct {
			Action         string `json:"action"`
			IntervalHours  int    `json:"intervalHours"`
			MarkSeenOnView bool   `json:"markSeenOnView"`
		}
		if !readBooruReviewJSON(w, r, &input) {
			return
		}
		switch input.Action {
		case "set", "set-mark-seen":
			settings, err := rs.followSchedule()
			if err != nil {
				settings = followScheduleSettings{}
			}
			if input.Action == "set" {
				if !slices.Contains(followIntervals, input.IntervalHours) {
					writeBooruError(w, 400, "following", "Choose Off, or every 1, 3, 6, 12 or 24 hours.")
					return
				}
				settings.IntervalHours = input.IntervalHours
			} else {
				settings.MarkSeenOnView = input.MarkSeenOnView
			}
			data, err := json.Marshal(settings)
			if err == nil {
				err = os.WriteFile(rs.followSchedulePath(), data, 0o600)
			}
			if err != nil {
				writeBooruError(w, 500, "following", "Cannot save the automatic check setting.")
				return
			}
		case "run":
			views, err := rs.followViews(r.Context())
			if err != nil {
				writeBooruError(w, 500, "following", "Cannot load follows.")
				return
			}
			rs.startFollowCheck(context.Background(), dueFollows(views, time.Now(), 0))
		default:
			writeBooruError(w, 400, "following", "Unknown action.")
			return
		}
	}
	settings, err := rs.followSchedule()
	if err != nil {
		writeBooruError(w, 500, "following", "Cannot read the automatic check setting.")
		return
	}
	views, err := rs.followViews(r.Context())
	if err != nil {
		writeBooruError(w, 500, "following", "Cannot load follows.")
		return
	}
	var unseen int
	err = rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		_, unseen, err = rs.repository.Following.UnseenCounts(ctx)
		return err
	})
	if err != nil {
		writeBooruError(w, 500, "following", "Cannot count new posts.")
		return
	}
	response := map[string]any{"intervalHours": settings.IntervalHours, "markSeenOnView": settings.MarkSeenOnView, "unseen": unseen, "running": false}
	followSchedules.Lock()
	if state := followSchedules.states[rs.configPath]; state != nil {
		response["running"] = state.running
		if state.lastRun != nil {
			response["lastRun"] = *state.lastRun
		}
	}
	followSchedules.Unlock()
	if settings.IntervalHours > 0 {
		if next := nextFollowRun(views, time.Duration(settings.IntervalHours)*time.Hour); !next.IsZero() {
			if next.Before(time.Now()) {
				next = time.Now().Add(time.Minute)
			}
			response["nextRun"] = next.UTC().Format(time.RFC3339)
		}
	}
	writeVisualSimilarityJSON(w, response)
}
