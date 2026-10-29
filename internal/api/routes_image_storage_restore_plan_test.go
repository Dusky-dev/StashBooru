package api

import (
	"strings"
	"testing"
)

func restoreReadinessFixtures() (estimateSourceGroup, storageCandidateProof, storageBackupPreflightReport) {
	group := preflightTestGroup()
	hashes := map[string]string{
		"image:1": strings.Repeat("a", 64),
		"image:2": strings.Repeat("b", 64),
	}
	candidate := storageCandidateProof{
		Group:                  group.ID,
		MediaBackupSnapshot:    "20261029T102030Z-012345abcdef",
		Method:                 "delta",
		Fidelity:               "byte-exact",
		SourceFiles:            2,
		SourceBytes:            2048,
		ArchiveBytes:           700,
		SourceSHA256:           hashes,
		Verified:               true,
		BackupVerified:         true,
		SourcesRetained:        true,
		NativePackingCandidate: true,
	}
	backup := storageBackupPreflightReport{
		Experimental:          true,
		ReadOnly:              true,
		Verified:              true,
		MediaBackupVerified:   true,
		SourcesRetained:       true,
		Group:                 group.ID,
		Snapshot:              candidate.MediaBackupSnapshot,
		SelectedImagesCovered: 2,
		ImageSHA256:           hashes,
	}
	return group, candidate, backup
}

func TestRestoreReadinessAcceptsOnlySafeIdentifiers(t *testing.T) {
	id := "stack-7-delta-0123456789abcdef01234567"
	if err := validateStorageRestorePlanRequest(storageRestorePlanRequest{StackID: 7, CandidateID: id}); err != nil {
		t.Fatal(err)
	}
	for _, request := range []storageRestorePlanRequest{
		{StackID: 0, CandidateID: id},
		{StackID: 7, CandidateID: "../escape"},
		{StackID: 7, CandidateID: "stack-7-foo-0123456789abcdef01234567"},
		{StackID: 7, CandidateID: ""},
	} {
		if validateStorageRestorePlanRequest(request) == nil {
			t.Fatalf("accepted invalid restore plan input: %+v", request)
		}
	}
}

func TestRestoreReadinessRequiresCandidateBackupSourceAgreement(t *testing.T) {
	group, candidate, backup := restoreReadinessFixtures()
	plan, err := buildStorageRestorePlan(group, "stack-7-delta-0123456789abcdef01234567", candidate, backup)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.ReadOnly || !plan.Inspected || !plan.ArchiveSHA256Verified ||
		!plan.MediaBackupVerified || !plan.CurrentSourcesMatch || !plan.CandidateHashesMatch {
		t.Fatal("complete read-only proof missing")
	}
	if plan.CanActivate || plan.CanReplaceOriginals || plan.OriginalEvictionEnabled ||
		plan.DatabaseBackupVerified || plan.RollbackVerified || plan.NativeServingVerified {
		t.Fatal("readiness plan must never authorize activation or eviction")
	}
	if plan.Images != 2 || plan.SourceBytes != 2048 || plan.ArchiveBytes != 700 || len(plan.Blockers) < 4 {
		t.Fatalf("missing sizes or safety blockers: %+v", plan)
	}
	backup.ImageSHA256 = map[string]string{
		"image:1": strings.Repeat("a", 64),
		"image:2": strings.Repeat("c", 64),
	}
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("candidate and backup have mismatched member hash")
	}
}

func TestRestoreReadinessRefusesUnsafeFlagsAndMissingMembers(t *testing.T) {
	group, candidate, backup := restoreReadinessFixtures()
	candidate.NativeServingVerified = true
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted candidate with native serving activated")
	}
	candidate.NativeServingVerified = false
	candidate.OriginalEviction = true
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted candidate permitting original eviction")
	}
	candidate.OriginalEviction = false
	backup.MediaBackupVerified = false
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted unverified original-media backup")
	}
	backup.MediaBackupVerified = true
	backup.ImageSHA256 = map[string]string{"image:1": strings.Repeat("a", 64)}
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted missing source hash")
	}
}

func TestRestoreReadinessRejectsMismatchedSnapshotOrSelection(t *testing.T) {
	group, candidate, backup := restoreReadinessFixtures()
	backup.Snapshot = "20261029T111111Z-012345abcdef"
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted wrong snapshot")
	}
	backup.Snapshot = candidate.MediaBackupSnapshot
	candidate.Method = "pixel-exact"
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted lossy/canonical representation method")
	}
	candidate.Method = "delta"
	group.ID = "stack:99"
	if _, err := buildStorageRestorePlan(group, "saved", candidate, backup); err == nil {
		t.Fatal("accepted mismatched Visual Stack")
	}
}
