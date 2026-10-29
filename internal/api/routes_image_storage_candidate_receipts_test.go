package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureSavedCandidate(t *testing.T, directory string) (string, json.RawMessage) {
	t.Helper()
	id := "stack-7-delta-0123456789abcdef01234567"
	archive := filepath.Join(directory, id+".pack")
	if err := os.WriteFile(archive, []byte("PACK"), 0600); err != nil {
		t.Fatal(err)
	}
	data := validStorageCandidateReport()
	data["archivePath"] = archive
	report, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return id, report
}

func TestCandidateReceiptsSurviveRestartWithoutSourceMutation(t *testing.T) {
	dir := t.TempDir()
	id, report := fixtureSavedCandidate(t, dir)
	savedID, err := storageCandidateSaveReceipt(dir, report)
	if err != nil || savedID != id {
		t.Fatalf("could not save candidate: %s, %v", savedID, err)
	}
	receipts := storageCandidateSavedList(dir)
	if len(receipts) != 1 || receipts[0].ID != id || receipts[0].Group != "stack:7" {
		t.Fatalf("saved candidate not listed after restart-like reload: %+v", receipts)
	}
	loaded, summary, err := storageCandidateReadReceipt(dir, id, true)
	if err != nil || summary.Method != "delta" || !json.Valid(loaded) {
		t.Fatalf("saved candidate was not independently rechecked: %v", err)
	}
	if _, err := storageCandidateSaveReceipt(dir, report); err == nil {
		t.Fatal("overwrote an immutable receipt")
	}
	receipt := filepath.Join(dir, id+".receipt.json")
	info, err := os.Stat(receipt)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("receipt is not private: %v", err)
	}
}

func TestCandidateReceiptsRejectTamperedArchiveAndUntrustedIDs(t *testing.T) {
	dir := t.TempDir()
	id, report := fixtureSavedCandidate(t, dir)
	if _, err := storageCandidateSaveReceipt(dir, report); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "../outside", id + "/else", "stack-0-delta-0123456789abcdef01234567", "stack-7-foo-0123456789abcdef01234567"} {
		if _, _, err := storageCandidateReadReceipt(dir, invalid, true); err == nil {
			t.Fatalf("accepted untrusted candidate identifier: %q", invalid)
		}
	}
	archive := filepath.Join(dir, id+".pack")
	if err := os.WriteFile(archive, []byte("EVIL"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storageCandidateReadReceipt(dir, id, true); err == nil {
		t.Fatal("accepted same-sized modified archive after restart")
	}
}

func TestCandidateReceiptsRefuseSymlinkedAndCorruptMetadata(t *testing.T) {
	dir := t.TempDir()
	id, report := fixtureSavedCandidate(t, dir)
	if _, err := storageCandidateSaveReceipt(dir, report); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(dir, id+".receipt.json")
	if err := os.WriteFile(receipt, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storageCandidateReadReceipt(dir, id, true); err == nil {
		t.Fatal("accepted corrupt saved receipt")
	}
	if err := os.Remove(receipt); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "external-receipt")
	if err := os.WriteFile(alias, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(alias, receipt); err == nil {
		if _, _, err := storageCandidateReadReceipt(dir, id, true); err == nil {
			t.Fatal("accepted symlinked candidate receipt")
		}
	}
}

func TestCandidateReceiptSafetyFlagAndPathChecks(t *testing.T) {
	dir := t.TempDir()
	id, report := fixtureSavedCandidate(t, dir)
	var fields map[string]any
	if err := json.Unmarshal(report, &fields); err != nil {
		t.Fatal(err)
	}
	fields["originalEvictionEnabled"] = true
	bad, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storageCandidateSaveReceipt(dir, bad); err == nil {
		t.Fatal("saved receipt enabling original deletion")
	}
	fields["originalEvictionEnabled"] = false
	fields["archivePath"] = filepath.Join(dir, "outside", id+".pack")
	bad, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storageCandidateSaveReceipt(dir, bad); err == nil {
		t.Fatal("saved receipt with nested output path")
	}
	if _, err := storageCandidateReceiptPath(dir, strings.Repeat("x", 256)); err == nil {
		t.Fatal("accepted unbounded receipt ID")
	}
}
