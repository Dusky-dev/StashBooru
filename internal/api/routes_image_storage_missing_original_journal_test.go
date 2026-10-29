package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPrivateRecoveryJournal(t *testing.T) (string, missingOriginalJournalRecord) {
	t.Helper()
	dir := t.TempDir()
	id := "stack-17-delta-0123456789abcdef01234567"
	out := filepath.Join(dir, "stack-17-orphan-rehearsal-aaaaaaaaaaaaaaaaaaaaaaaa")
	r := missingOriginalNewJournal(id, "20261029T102030Z-012345abcdef", out)
	return dir, r
}

func TestPrivateScratchJournalSurvivesReadAndFailsClosedAfterRestart(t *testing.T) {
	dir, record := testPrivateRecoveryJournal(t)
	if err := missingOriginalWriteJournal(dir, record); err != nil {
		t.Fatal(err)
	}
	loaded, err := missingOriginalReadJournal(dir)
	if err != nil || loaded == nil || loaded.CandidateID != record.CandidateID {
		t.Fatalf("journal did not survive write and reload: %+v, %v", loaded, err)
	}
	display := missingOriginalJournalForDisplay(loaded, false)
	if display.State != "interrupted_or_unconfirmed" || !display.RequiresReaudit ||
		display.CanActivate || display.EvictionEnabled {
		t.Fatalf("stale running recovery was reported as safe: %+v", display)
	}
	if current := missingOriginalJournalForDisplay(loaded, true); current.State != "running" {
		t.Fatalf("active in-process recovery was not distinguished: %+v", current)
	}
	if info, err := os.Stat(missingOriginalJournalPath(dir)); err != nil || info.Size() == 0 {
		t.Fatalf("missing private journal: %v", err)
	}
}

func TestPrivateScratchJournalCompletionNeverClaimsNativeRecovery(t *testing.T) {
	dir, r := testPrivateRecoveryJournal(t)
	if err := missingOriginalWriteJournal(dir, r); err != nil {
		t.Fatal(err)
	}
	missingOriginalJournalFinish(&r, json.RawMessage(`{"verified":true}`), 2)
	if err := missingOriginalWriteJournal(dir, r); err != nil {
		t.Fatal(err)
	}
	reloaded, err := missingOriginalReadJournal(dir)
	if err != nil || reloaded == nil || reloaded.State != "scratch_complete" {
		t.Fatalf("completed scratch journal rejected: %+v, %v", reloaded, err)
	}
	status := missingOriginalJournalForDisplay(reloaded, false)
	if status.State != "scratch_complete" || !status.RequiresReaudit || status.CanActivate || status.EvictionEnabled {
		t.Fatalf("saved scratch result became live activation: %+v", status)
	}
	missingOriginalJournalFail(&r)
	if err := missingOriginalWriteJournal(dir, r); err != nil {
		t.Fatal(err)
	}
	after, err := missingOriginalReadJournal(dir)
	if err != nil || after.State != "failed" || after.ScratchProofAtCompletion {
		t.Fatalf("failed journal retained success proof: %+v %v", after, err)
	}
}

func TestPrivateScratchJournalRejectsUnsafePathsAndFlags(t *testing.T) {
	dir, original := testPrivateRecoveryJournal(t)
	for _, mutate := range []func(*missingOriginalJournalRecord){
		func(r *missingOriginalJournalRecord) { r.OutputPath = filepath.Join(dir, "..", "outside") },
		func(r *missingOriginalJournalRecord) {
			r.OutputPath = filepath.Join(dir, "stack-18-orphan-rehearsal-aaaaaaaaaaaaaaaaaaaaaaaa")
		},
		func(r *missingOriginalJournalRecord) { r.CanActivate = true },
		func(r *missingOriginalJournalRecord) { r.OriginalEvictionEnabled = true },
		func(r *missingOriginalJournalRecord) { r.DatabaseRecoveryVerified = true },
		func(r *missingOriginalJournalRecord) { r.CrashSafeRollbackVerified = true },
		func(r *missingOriginalJournalRecord) { r.State = "scratch_complete" },
		func(r *missingOriginalJournalRecord) { r.UpdatedUnix = time.Now().Add(48 * time.Hour).Unix() },
	} {
		r := original
		mutate(&r)
		if err := missingOriginalWriteJournal(dir, r); err == nil {
			t.Fatalf("wrote unsafe journal: %+v", r)
		}
	}
}

func TestPrivateScratchJournalRejectsSymlinkCorruptionAndOversize(t *testing.T) {
	dir, r := testPrivateRecoveryJournal(t)
	filename := missingOriginalJournalPath(dir)
	if err := os.Symlink(filepath.Join(dir, "untrusted"), filename); err == nil {
		if _, err := missingOriginalReadJournal(dir); err == nil {
			t.Fatal("accepted symlink journal")
		}
		if err := missingOriginalWriteJournal(dir, r); err == nil {
			t.Fatal("overwrote symlinked journal")
		}
		if err := os.Remove(filename); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filename, []byte("{malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := missingOriginalWriteJournal(dir, r); err == nil {
		t.Fatal("overwrote malformed journal")
	}
	if err := os.WriteFile(filename, []byte(strings.Repeat("a", missingOriginalJournalMax+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := missingOriginalReadJournal(dir); err == nil {
		t.Fatal("accepted oversized journal")
	}
}
