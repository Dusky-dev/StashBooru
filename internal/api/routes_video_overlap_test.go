package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/videooverlap"
	"github.com/stretchr/testify/require"
)

func TestVideoOverlapRejectsMalformedOrMultipleRequestsBeforeQueuing(t *testing.T) {
	for _, body := range []string{
		`{"action":"index","ids":[1],"unknown":true}`,
		`{"action":"index","ids":[1]} {"action":"cancel"}`,
		`{"action":"index","ids":[1]} trailing`,
		`{"action":"index","ids":["invalid"]}`,
		strings.Repeat(" ", 256*1024+1),
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/scene/overlap", strings.NewReader(body))
		sceneRoutes{}.VideoOverlap(response, request)
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
}

func TestVideoOverlapCheckpointStateOutlivesNativeJobGraveyard(t *testing.T) {
	j := videooverlap.Job{Status: "queued", Session: videoOverlapSession, NativeID: 27}
	require.Equal(t, "queued", videoOverlapObservedStatus(j, &job.Job{Status: job.StatusReady}))
	require.Equal(t, "cancelled", videoOverlapObservedStatus(j, &job.Job{Status: job.StatusCancelled}))
	require.Equal(t, "interrupted", videoOverlapObservedStatus(j, nil), "native jobs retain only ten terminal entries; checkpoint must remain resumable")
	j.Session = "prior-server-session"
	require.Equal(t, "interrupted", videoOverlapObservedStatus(j, &job.Job{Status: job.StatusRunning}), "recycled native IDs cannot attach a checkpoint to another server session")
	j.Status = "complete"
	require.Equal(t, "complete", videoOverlapObservedStatus(j, nil))
}
