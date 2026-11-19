//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func TestFollowScheduleChecksDueFollowsAndIsolatesFailures(t *testing.T) {
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "schedule.sqlite")))
	defer db.Close()
	rs := booruBrowserRoutes{configPath: t.TempDir(), repository: db.Repository()}
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	a := b06Mutation(t, rs, b06Create(dan, "artist", "artist_a"), 200)
	b06Mutation(t, rs, b06Create(dan, "copyright", "series_s"), 200)
	b06Mutation(t, rs, b06Create(gel, "artist", "artist_g"), 200)
	b06Mutation(t, rs, b06Create(gel, "artist", "artist_h"), 200)

	var mu sync.Mutex
	calls := map[string]int{}
	rs.client = feedFixtureClient{page: func(_ context.Context, _ booru.Source, _, target, cursor string) (booru.FollowPage, error) {
		mu.Lock()
		calls[target]++
		mu.Unlock()
		switch target {
		case "artist_a":
			if cursor == "" {
				return booru.FollowPage{Posts: feedPosts("12", "11"), Cursor: "page2", HasMore: true}, nil
			}
			return booru.FollowPage{Posts: feedPosts("10"), Cursor: "done"}, nil
		case "series_s":
			return booru.FollowPage{}, &booru.Error{Code: "provider-error", Message: "Provider returned HTTP 500."}
		default:
			return booru.FollowPage{}, &booru.Error{Code: "rate-limited", Message: "Provider requested a pause before retrying."}
		}
	}}

	// Off by default: a tick starts nothing.
	require.False(t, rs.followSchedulerTick(context.Background(), time.Now()))
	feedRequest(t, rs.FollowScheduleRoute, map[string]any{"action": "set", "intervalHours": 2}, 400)
	feedRequest(t, rs.FollowScheduleRoute, map[string]any{"action": "set", "intervalHours": 1}, 200)

	require.True(t, rs.followSchedulerTick(context.Background(), time.Now()))
	var run followScheduleRun
	require.Eventually(t, func() bool {
		followSchedules.Lock()
		defer followSchedules.Unlock()
		state := followSchedules.states[rs.configPath]
		if state == nil || state.running || state.lastRun == nil {
			return false
		}
		run = *state.lastRun
		return true
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, 3, run.Checked, "the second follow on a site that asked to slow down waits")
	require.Equal(t, 3, run.NewPosts, "a follow is caught up across pages")
	require.Equal(t, 2, run.Failed)
	require.Contains(t, run.Note, "asked to slow down")
	require.Equal(t, 1, calls["artist_g"]+calls["artist_h"], "no retry against a rate-limited site")
	require.Equal(t, 2, calls["artist_a"])

	views, err := rs.followViews(context.Background())
	require.NoError(t, err)
	for _, view := range views {
		if view.ID == a.ID {
			require.Equal(t, 3, view.Unseen)
		}
	}
	// Checked and failed follows wait for the interval; the skipped one is due.
	now := time.Now()
	due := dueFollows(views, now, time.Hour)
	require.Len(t, due, 1)
	require.Equal(t, "artist_h", due[0].Target)
	require.Len(t, dueFollows(views, now.Add(2*time.Hour), time.Hour), 4)

	// The status shows the setting, the run and the distinct unseen total.
	w := httptest.NewRecorder()
	rs.FollowScheduleRoute(w, httptest.NewRequest("GET", "/following/schedule", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	var status struct {
		IntervalHours int                `json:"intervalHours"`
		Unseen        int                `json:"unseen"`
		LastRun       *followScheduleRun `json:"lastRun"`
		NextRun       string             `json:"nextRun"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	require.Equal(t, 1, status.IntervalHours)
	require.Equal(t, 3, status.Unseen)
	require.NotNil(t, status.LastRun)
	require.NotEmpty(t, status.NextRun)

	// A cancelled run checks nothing and says so.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopped := rs.checkFollows(ctx, views, nil)
	require.Zero(t, stopped.Checked)
	require.True(t, strings.HasPrefix(stopped.Note, "Stopped"))

	// A broken setting file turns automatic checks off rather than guessing.
	require.NoError(t, os.WriteFile(rs.followSchedulePath(), []byte(`{"intervalHours":5}`), 0o600))
	require.False(t, rs.followSchedulerTick(context.Background(), now.Add(3*time.Hour)))
}
