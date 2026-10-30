package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestShadowCanaryOnlyAllowsPreparedSafeIntent(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	if !shadowCanaryAllowed(&record, 42) {
		t.Fatal("valid prepared candidate cannot enter guarded trial")
	}
	for _, imageID := range []int{-1, 0} {
		if shadowCanaryAllowed(&record, imageID) {
			t.Fatalf("accepted invalid native image ID %d", imageID)
		}
	}
	if shadowCanaryAllowed(nil, 42) {
		t.Fatal("accepted missing selection")
	}
	for _, modify := range []func(*shadowSelectionRecord){
		func(r *shadowSelectionRecord) { r.State = "disarmed" },
		func(r *shadowSelectionRecord) { r.NativeRoutingEnabled = true },
		func(r *shadowSelectionRecord) { r.OriginalEvictionEnabled = true },
		func(r *shadowSelectionRecord) { r.TransactionalRollbackVerified = true },
		func(r *shadowSelectionRecord) { r.ArchiveSHA256 = strings.Repeat("0", 16) },
	} {
		unsafe := record
		modify(&unsafe)
		if shadowCanaryAllowed(&unsafe, 42) {
			t.Fatalf("unsafe saved state could enter canary: %+v", unsafe)
		}
	}
}

func TestShadowCanaryRejectsQueryAndNonCanonicalIDsBeforeAccess(t *testing.T) {
	for _, item := range []struct {
		route string
		id    string
	}{
		{"/image/042/storage-canary", "042"},
		{"/image/checksum/storage-canary", "checksum"},
		{"/image/-1/storage-canary", "-1"},
		{"/image/0/storage-canary", "0"},
		{"/image/42/storage-canary?archive=/etc/shadow", "42"},
		{"/image/42/storage-canary?storageShadowCandidate=forged", "42"},
		{"/image/42/storage-canary?candidateID=abc", "42"},
		{"/image/42/storage-canary?%ZZ=bad", "42"},
	} {
		req := httptest.NewRequest(http.MethodGet, item.route, nil)
		ctx := chi.NewRouteContext()
		ctx.URLParams.Add("imageId", item.id)
		req = req.WithContext(
			context.WithValue(req.Context(), chi.RouteCtxKey, ctx),
		)
		w := httptest.NewRecorder()
		(imageRoutes{}).StorageShadowCanary(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status %d, want 400", item.route, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s lacked no-store", item.route)
		}
	}
}
