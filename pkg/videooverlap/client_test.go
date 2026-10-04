package videooverlap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplicitRemoteNeverInvokesLocalFallback(t *testing.T) {
	t.Setenv("STASH_VIDEO_OVERLAP_WORKER", filepath.Join(t.TempDir(), "absent.py"))
	_, _, backend, err := SelectClient(context.Background(), "remote", Client{}, Client{})
	require.Equal(t, "remote", backend)
	require.ErrorContains(t, err, "configure the remote worker")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}))
	defer server.Close()
	_, _, backend, err = SelectClient(context.Background(), "remote", Client{}, Client{URL: server.URL})
	require.Equal(t, "remote", backend)
	require.ErrorContains(t, err, "HTTP 401")
	require.NotContains(t, err.Error(), "absent.py", "an explicit remote failure cannot start a local worker")
}
