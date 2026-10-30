package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testShadowSelectionRecord(t *testing.T, dir string) shadowSelectionRecord {
	t.Helper()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id, report := fixtureSavedCandidate(t, dir)
	if _, err := storageCandidateSaveReceipt(dir, report); err != nil {
		t.Fatal(err)
	}
	receiptDigest, archiveDigest, err := shadowSelectionCandidateEvidence(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	return shadowSelectionRecord{
		Version: 1, State: "prepared", CandidateID: id, StackID: 7,
		Snapshot: "20261029T102030Z-012345abcdef", Method: "delta",
		ReceiptSHA256: receiptDigest, ArchiveSHA256: archiveDigest,
		Revision: 1, UpdatedUnix: time.Now().Unix(), OriginalsAuthoritative: true,
	}
}

func TestShadowSelectionSurvivesRestartButDoesNotActivate(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	if err := shadowSelectionWrite(dir, record); err != nil {
		t.Fatal(err)
	}
	stored, err := shadowSelectionRead(dir)
	if err != nil || stored == nil || *stored != record {
		t.Fatalf("saved intent cannot be independently reloaded: %+v %v", stored, err)
	}
	status := shadowSelectionStatusFor(stored, true)
	if status.State != "prepared" || !status.EvidenceVerifiedNow ||
		!status.RequiresFreshByteCheck || status.NativeRoutingEnabled ||
		status.CanActivate || status.OriginalEvictionEnabled ||
		status.TransactionalRollbackVerified || !status.OriginalsAuthoritative {
		t.Fatalf("prepared shadow intent incorrectly permitted activation: %+v", status)
	}
	if _, err := os.Stat(filepath.Join(dir, record.CandidateID+".pack")); err != nil {
		t.Fatal("original archive disappeared")
	}
	if info, err := os.Stat(shadowSelectionPath(dir)); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("intent record is not private: %v", err)
	}
	disarmed := *stored
	disarmed.State = "disarmed"
	disarmed.Revision++
	disarmed.UpdatedUnix = time.Now().Unix()
	if err := shadowSelectionWrite(dir, disarmed); err != nil {
		t.Fatal(err)
	}
	restart, err := shadowSelectionRead(dir)
	if err != nil || restart.State != "disarmed" || restart.Revision != 2 {
		t.Fatalf("disarm did not survive restart: %+v %v", restart, err)
	}
	if status := shadowSelectionStatusFor(restart, true); status.EvidenceVerifiedNow ||
		status.RequiresFreshByteCheck || status.NativeRoutingEnabled ||
		status.OriginalEvictionEnabled || status.CanActivate {
		t.Fatalf("disarmed shadow source became active: %+v", status)
	}
}

func TestShadowSelectionRevisionAndStateFailClosed(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	if err := shadowSelectionWrite(dir, record); err != nil {
		t.Fatal(err)
	}
	if err := shadowSelectionWrite(dir, record); err == nil {
		t.Fatal("replayed shadow state transition")
	}
	for _, mutate := range []func(*shadowSelectionRecord){
		func(r *shadowSelectionRecord) { r.State = "active" },
		func(r *shadowSelectionRecord) { r.State = "rollback_complete" },
		func(r *shadowSelectionRecord) { r.NativeRoutingEnabled = true },
		func(r *shadowSelectionRecord) { r.CanActivate = true },
		func(r *shadowSelectionRecord) { r.OriginalEvictionEnabled = true },
		func(r *shadowSelectionRecord) { r.TransactionalRollbackVerified = true },
		func(r *shadowSelectionRecord) { r.OriginalsAuthoritative = false },
		func(r *shadowSelectionRecord) { r.ReceiptSHA256 = strings.Repeat("0", 16) },
		func(r *shadowSelectionRecord) { r.CandidateID = "../etc/passwd" },
		func(r *shadowSelectionRecord) { r.StackID = 8 },
		func(r *shadowSelectionRecord) { r.Revision = 1000001 },
		func(r *shadowSelectionRecord) { r.UpdatedUnix = time.Now().Add(48 * time.Hour).Unix() },
	} {
		next := record
		next.Revision = 2
		mutate(&next)
		if err := shadowSelectionWrite(dir, next); err == nil {
			t.Fatalf("saved unsafe durable shadow intent: %+v", next)
		}
	}
}

func TestShadowSelectionRejectsCorruptSymlinkAndOversize(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	target := shadowSelectionPath(dir)
	if err := os.Symlink(filepath.Join(dir, "other"), target); err == nil {
		if _, err := shadowSelectionRead(dir); err == nil {
			t.Fatal("accepted symlinked selection")
		}
		if err := shadowSelectionWrite(dir, record); err == nil {
			t.Fatal("replaced symlinked selection")
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(target, []byte("{broken json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := shadowSelectionWrite(dir, record); err == nil {
		t.Fatal("replaced corrupt state instead of refusing")
	}
	if err := os.WriteFile(target, []byte(strings.Repeat("x", shadowSelectionLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := shadowSelectionRead(dir); err == nil {
		t.Fatal("oversized state was accepted")
	}
}

func TestShadowSelectionDetectsMutatedArchiveAndReceipt(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	id := record.CandidateID
	hashReceipt, hashArchive, err := shadowSelectionCandidateEvidence(dir, id)
	if err != nil || hashReceipt != record.ReceiptSHA256 || hashArchive != record.ArchiveSHA256 {
		t.Fatal("valid candidate digest did not match")
	}
	archive := filepath.Join(dir, id+".pack")
	if err := os.WriteFile(archive, []byte("TAMPER"), 0600); err != nil {
		t.Fatal(err)
	}
	_, changed, err := shadowSelectionCandidateEvidence(dir, id)
	if err != nil || changed == record.ArchiveSHA256 {
		t.Fatal("modified archived media did not invalidate evidence")
	}
	receipt := filepath.Join(dir, id+".receipt.json")
	if err := os.WriteFile(receipt, []byte("TAMPERED RECEIPT"), 0600); err != nil {
		t.Fatal(err)
	}
	modified, _, err := shadowSelectionCandidateEvidence(dir, id)
	if err != nil || modified == record.ReceiptSHA256 {
		t.Fatal("modified private candidate receipt did not invalidate evidence")
	}
}

func TestShadowSelectionRejectsForgedJSONFlags(t *testing.T) {
	dir := t.TempDir()
	record := testShadowSelectionRecord(t, dir)
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"NativeRoutingEnabled", "OriginalEvictionEnabled", "TransactionalRollbackVerified"} {
		fields[flag] = true
		forged, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(shadowSelectionPath(dir), forged, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := shadowSelectionRead(dir); err == nil {
			t.Fatalf("forged safety flag %s was accepted", flag)
		}
		fields[flag] = false
	}
}
