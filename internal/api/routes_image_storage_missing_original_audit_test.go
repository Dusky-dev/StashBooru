package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeScratchAuditRequestDoesNotAcceptArbitraryPaths(t *testing.T) {
	id := "stack-17-shared-tiles-0123456789abcdef01234567"
	if err := validateMissingOriginalScratchAuditRequest(missingOriginalScratchAuditRequest{CandidateID: id}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "../../outside", "/tmp/scratch", "stack-0-delta-0123456789abcdef01234567"} {
		if err := validateMissingOriginalScratchAuditRequest(missingOriginalScratchAuditRequest{CandidateID: invalid}); err == nil {
			t.Fatalf("accepted invalid candidate: %q", invalid)
		}
	}
}

func scratchAuditProof(t *testing.T, state string) ([]byte, string, string) {
	t.Helper()
	id, output := "stack-17-delta-0123456789abcdef01234567", "/private/stack-17-orphan-rehearsal-aaaaaaaaaaaaaaaaaaaaaaaa"
	record := missingOriginalScratchAuditReport{
		CandidateID: id, OutputPath: output, State: state, ReadOnlyInspection: true,
	}
	if state == "complete_verified" {
		record.Complete = true
		record.ScratchFilesVerified = 2
		record.ScratchBytesVerified = 40
	}
	blob, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return blob, id, output
}

func TestNativeScratchAuditFailClosedProofStates(t *testing.T) {
	for _, state := range []string{"no_output", "incomplete", "complete_verified"} {
		data, id, output := scratchAuditProof(t, state)
		report, err := validateMissingOriginalScratchAuditReport(data, id, output, 2)
		if err != nil || report.State != state {
			t.Fatalf("valid %q scratch inspection failed: %v", state, err)
		}
		if report.CanActivate || report.OriginalEvictionEnabled {
			t.Fatal("scratch audit incorrectly authorized live activation")
		}
	}
}

func TestNativeScratchAuditRejectsForgedCapabilitiesAndMemberCounts(t *testing.T) {
	data, id, output := scratchAuditProof(t, "complete_verified")
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"canActivate", "originalEvictionEnabled", "databaseRecoveryVerified", "crashSafeLiveRollbackVerified", "mediaBackupReverified", "nativeServingVerified"} {
		fields[field] = true
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := validateMissingOriginalScratchAuditReport(encoded, id, output, 2); err == nil {
			t.Fatalf("accepted forbidden capability %s", field)
		}
		fields[field] = false
	}
	if _, err := validateMissingOriginalScratchAuditReport(data, id, output, 3); err == nil {
		t.Fatal("accepted incomplete member coverage")
	}
	if _, err := validateMissingOriginalScratchAuditReport(data, id, output+"-other", 2); err == nil {
		t.Fatal("accepted wrong scratch directory")
	}
	fields["state"] = "incomplete"
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateMissingOriginalScratchAuditReport(encoded, id, output, 2); err == nil {
		t.Fatal("accepted completed members with incomplete state")
	}
	if _, err := validateMissingOriginalScratchAuditReport([]byte(strings.Repeat("X", 16385)), id, output, 2); err == nil {
		t.Fatal("accepted unbounded audit report")
	}
}
