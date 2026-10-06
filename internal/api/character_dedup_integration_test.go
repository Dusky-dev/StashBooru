//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/characterdedup"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stretchr/testify/require"
)

func TestCharacterDedupReviewRoutes(t *testing.T) {
	repo := p07Fixture(t)
	jobs := job.NewManager()
	defer jobs.Stop()
	routes := characterDedupRoutes{repository: repo, jobs: jobs}
	response := httptest.NewRecorder()
	routes.Get(response, httptest.NewRequest(http.MethodGet, "/character-dedup", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var preview characterdedup.Plan
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &preview))
	require.Len(t, preview.Fingerprint, 64)
	for _, body := range []string{`{}`, `{"fingerprint":"short"}`, `{"fingerprint":"` + preview.Fingerprint + `","ids":[1]}`, `{"fingerprint":"` + preview.Fingerprint + `"}{}`, strings.Repeat(" ", 1025) + `{}`} {
		response = httptest.NewRecorder()
		routes.Post(response, httptest.NewRequest(http.MethodPost, "/character-dedup", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
	require.Empty(t, jobs.GetQueue())
	response = httptest.NewRecorder()
	routes.Post(response, httptest.NewRequest(http.MethodPost, "/character-dedup", strings.NewReader(`{"fingerprint":"`+strings.Repeat("a", 64)+`"}`)))
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		JobID int `json:"jobID"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Eventually(t, func() bool { j := jobs.GetJob(result.JobID); return j != nil && j.Status == job.StatusFailed }, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, jobs.GetJob(result.JobID).Error)
	require.Contains(t, *jobs.GetJob(result.JobID).Error, "preview again")
	require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
		count, err := repo.Performer.Count(ctx)
		require.Zero(t, count)
		return err
	}))
}
