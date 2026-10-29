package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingOriginalNativeRequestRequiresAcknowledgementAndSavedID(t *testing.T) {
	good := missingOriginalRequest{
		CandidateID:  "stack-17-delta-0123456789abcdef01234567",
		Acknowledged: true,
	}
	if err := validateMissingOriginalRequest(good); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []missingOriginalRequest{
		{},
		{CandidateID: good.CandidateID},
		{CandidateID: "../../escape", Acknowledged: true},
		{CandidateID: "stack-0-delta-0123456789abcdef01234567", Acknowledged: true},
	} {
		if err := validateMissingOriginalRequest(invalid); err == nil {
			t.Fatalf("accepted unsafe missing-original rehearsal request: %+v", invalid)
		}
	}
}

func missingOriginalProofFixture(t *testing.T) ([]byte, string, string, string, map[string]string) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "restored")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "extraction.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	id := "stack-17-delta-0123456789abcdef01234567"
	snapshot := "20261029T102030Z-012345abcdef"
	hashes := map[string]string{
		"image:171": strings.Repeat("a", 64),
		"image:172": strings.Repeat("b", 64),
	}
	report := missingOriginalReport{}
	report.Verified = true
	report.ReadOnlyAgainstLibrary = true
	report.OriginalSourcesNotRead = true
	report.ArchiveSHA256Verified = true
	report.MediaBackupObjectsVerified = true
	report.BackupContentDigestsCovered = true
	report.CandidateID = id
	report.Snapshot = snapshot
	report.OutputPath = output
	report.RestoredFiles = 2
	report.RestoredSHA256 = hashes
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return encoded, id, snapshot, output, hashes
}

func TestMissingOriginalNativeReportCannotAuthorizeLiveActivation(t *testing.T) {
	data, id, snapshot, output, hashes := missingOriginalProofFixture(t)
	if _, err := validateMissingOriginalReport(data, id, snapshot, output, hashes); err != nil {
		t.Fatalf("valid scratch-only rehearsal evidence refused: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"canActivate", "originalEvictionEnabled", "databaseRecoveryVerified",
		"nativeMediaServingVerified", "crashSafeLiveRollbackVerified",
		"sourceToBackupPathMappingVerified", "requiresOriginalSources",
	} {
		fields[key] = true
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := validateMissingOriginalReport(encoded, id, snapshot, output, hashes); err == nil {
			t.Fatalf("accepted prohibited activation/recovery flag %s", key)
		}
		fields[key] = false
	}
}

func TestMissingOriginalNativeReportRejectsWrongHashesDestinationAndReceipt(t *testing.T) {
	data, id, snapshot, output, hashes := missingOriginalProofFixture(t)
	if _, err := validateMissingOriginalReport(data, "different", snapshot, output, hashes); err == nil {
		t.Fatal("accepted wrong candidate selection")
	}
	if _, err := validateMissingOriginalReport(data, id, "wrong", output, hashes); err == nil {
		t.Fatal("accepted wrong backup snapshot")
	}
	if _, err := validateMissingOriginalReport(data, id, snapshot, output+"-wrong", hashes); err == nil {
		t.Fatal("accepted wrong recovery destination")
	}
	changed := map[string]string{
		"image:171": strings.Repeat("a", 64),
		"image:172": strings.Repeat("c", 64),
	}
	if _, err := validateMissingOriginalReport(data, id, snapshot, output, changed); err == nil {
		t.Fatal("accepted altered restored member digest")
	}
	if err := os.Remove(filepath.Join(output, "extraction.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := validateMissingOriginalReport(data, id, snapshot, output, hashes); err == nil {
		t.Fatal("accepted missing extraction completion receipt")
	}
}
