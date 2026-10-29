package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsumerAuditRejectsUnsafeRequest(t *testing.T) {
	validID := "stack-7-delta-0123456789abcdef01234567"
	if err := validateStorageConsumerAuditRequest(storageConsumerAuditRequest{StackID: 7, CandidateID: validID}); err != nil {
		t.Fatal(err)
	}
	for _, req := range []storageConsumerAuditRequest{
		{StackID: 0, CandidateID: validID},
		{StackID: 7, CandidateID: "../../bad"},
		{StackID: 7, CandidateID: ""},
	} {
		if err := validateStorageConsumerAuditRequest(req); err == nil {
			t.Fatalf("accepted unsafe request: %+v", req)
		}
	}
}

func auditFixture(t *testing.T) ([]byte, estimateSourceGroup, storageCandidateProof, map[string][2]string) {
	t.Helper()
	group, candidate, _ := restoreReadinessFixtures()
	members := []map[string]any{
		{"id": "image:1", "bytes": 1200, "sha256": strings.Repeat("a", 64), "byteExact": true, "originalPresent": true},
		{"id": "image:2", "bytes": 1220, "sha256": strings.Repeat("b", 64), "byteExact": true, "originalPresent": true},
	}
	payload := make(map[string]any)
	payload["verified"] = true
	payload["readOnly"] = true
	payload["group"] = group.ID
	payload["method"] = candidate.Method
	payload["mediaBackupSnapshot"] = candidate.MediaBackupSnapshot
	payload["members"] = members
	payload["archiveActivated"] = false
	payload["databaseModified"] = false
	payload["nativeServingActivated"] = false
	payload["originalEvictionEnabled"] = false
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	caches := map[string][2]string{
		"image:1": {"cached_regular_unverified", "missing"},
		"image:2": {"missing", "cached_regular_unverified"},
	}
	return data, group, candidate, caches
}

func TestConsumerAuditRequiresAllHashesAndKeepsServingDisabled(t *testing.T) {
	data, group, candidate, caches := auditFixture(t)
	report, err := validateStorageConsumerAuditReport(data, group, candidate, "saved", caches)
	if err != nil {
		t.Fatal(err)
	}
	var decoded storageConsumerAuditReport
	if err := json.Unmarshal(report, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Verified || !decoded.ReadOnly || !decoded.MediaBackupVerified || !decoded.ArchiveSHA256Verified ||
		decoded.NativeConsumersVerified || decoded.NativeServingActivated ||
		decoded.CanActivate || decoded.OriginalEvictionEnabled || len(decoded.Members) != 2 {
		t.Fatalf("audit escalated read-only proof: %+v", decoded)
	}
	if decoded.Members[0].ThumbnailCache != "cached_regular_unverified" ||
		decoded.Members[1].PreviewCache != "cached_regular_unverified" {
		t.Fatal("missing clearly unverified cache observations")
	}
}

func TestConsumerAuditRejectsUnsafeFlagsAndMissingMembers(t *testing.T) {
	data, group, candidate, caches := auditFixture(t)
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	fail := func() {
		t.Helper()
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := validateStorageConsumerAuditReport(encoded, group, candidate, "saved", caches); err == nil {
			t.Fatal("accepted invalid shadow audit proof")
		}
	}
	payload["originalEvictionEnabled"] = true
	fail()
	payload["originalEvictionEnabled"] = false
	payload["nativeServingActivated"] = true
	fail()
	payload["nativeServingActivated"] = false
	payload["verified"] = false
	fail()
	payload["verified"] = true
	payload["members"] = []any{
		map[string]any{"id": "image:1", "bytes": 1200, "sha256": strings.Repeat("a", 64), "byteExact": true, "originalPresent": true},
	}
	fail()
	payload["members"] = []any{
		map[string]any{"id": "image:1", "bytes": 1200, "sha256": strings.Repeat("a", 64), "byteExact": true, "originalPresent": true},
		map[string]any{"id": "image:2", "bytes": 1220, "sha256": strings.Repeat("f", 64), "byteExact": true, "originalPresent": true},
	}
	fail()
}

func TestConsumerAuditCacheStatesAreReadOnlyAndRejectSymlinks(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "cached")
	if err := os.WriteFile(existing, []byte("cache"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := storageConsumerCacheState(existing); got != "cached_regular_unverified" {
		t.Fatalf("unexpected native cache observation %q", got)
	}
	if got := storageConsumerCacheState(filepath.Join(dir, "absent")); got != "missing" {
		t.Fatalf("unexpected missing cache observation %q", got)
	}
	link := filepath.Join(dir, "alias")
	if err := os.Symlink(existing, link); err == nil {
		if got := storageConsumerCacheState(link); got != "unsafe_or_unreadable" {
			t.Fatalf("symlink cache should be unsafe: %q", got)
		}
	}
	if raw, err := os.ReadFile(existing); err != nil || string(raw) != "cache" {
		t.Fatal("read-only cache inspection changed the generated file")
	}
}
