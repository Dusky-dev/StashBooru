package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShadowPreflightRequiresStablePreparedSelection(t *testing.T) {
	dir := t.TempDir()
	prepared := testShadowSelectionRecord(t, dir)
	observed := prepared
	if !shadowPreflightStable(&prepared, &observed) {
		t.Fatal("identical prepared selection must be stable")
	}
	if shadowPreflightStable(nil, &observed) || shadowPreflightStable(&prepared, nil) {
		t.Fatal("missing state was accepted")
	}
	observed.State = "disarmed"
	if shadowPreflightStable(&prepared, &observed) {
		t.Fatal("disarmed selection was accepted")
	}
	observed = prepared
	observed.Revision++
	if shadowPreflightStable(&prepared, &observed) {
		t.Fatal("new selection revision was accepted")
	}
	observed = prepared
	observed.ArchiveSHA256 = strings.Repeat("0", 64)
	if shadowPreflightStable(&prepared, &observed) {
		t.Fatal("changed archive evidence was accepted")
	}
}

func TestShadowPreflightReportCannotAuthorizeActivation(t *testing.T) {
	dir := t.TempDir()
	prepared := testShadowSelectionRecord(t, dir)
	report, err := makeShadowPreflightReport(prepared, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReadOnly || !report.AllOriginalBytesVerified ||
		!report.MediaBackupVerified || !report.ArchiveSHA256Verified ||
		!report.OriginalsAuthoritative || report.MembersVerified != 2 ||
		report.CandidateID != prepared.CandidateID || report.Revision != prepared.Revision ||
		report.NativeRoutingEnabled || report.CanActivate || report.OriginalEvictionEnabled ||
		report.TransactionalRollbackVerified || len(report.Blockers) == 0 {
		t.Fatalf("unsafe shadow preflight report: %+v", report)
	}
	for _, count := range []int{0, 1, 17} {
		if _, err := makeShadowPreflightReport(prepared, count); err == nil {
			t.Fatalf("accepted unsupported member count %d", count)
		}
	}
	disarmed := prepared
	disarmed.State = "disarmed"
	if _, err := makeShadowPreflightReport(disarmed, 2); err == nil {
		t.Fatal("disarmed selection passed")
	}
	unsafe := prepared
	unsafe.NativeRoutingEnabled = true
	if _, err := makeShadowPreflightReport(unsafe, 2); err == nil {
		t.Fatal("forged routing authorization passed")
	}
}

func TestShadowPreflightRequiresExplicitAcknowledgementAndNoPaths(t *testing.T) {
	tests := []struct {
		body string
		url  string
	}{
		{`{}`, "/image/storage-shadow-preflight"},
		{`{"Acknowledged":false}`, "/image/storage-shadow-preflight"},
		{`{"Acknowledged":true,"archive":"/tmp/forged.pack"}`, "/image/storage-shadow-preflight"},
		{`{"Acknowledged":true}{}`, "/image/storage-shadow-preflight"},
		{`{"Acknowledged":true}`, "/image/storage-shadow-preflight?candidateID=forged"},
	}
	for _, tc := range tests {
		request := httptest.NewRequest(http.MethodPost, tc.url, strings.NewReader(tc.body))
		response := httptest.NewRecorder()
		(imageRoutes{}).StorageShadowPreflight(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("unexpected preflight status %d for %s", response.Code, tc.body)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("preflight response can be cached")
		}
	}
}
