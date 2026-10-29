package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeRecoveryRequiresExplicitAck(t *testing.T) {
	for _, request := range []storageRecoveryRequest{
		{StackID: 0, Acknowledged: true},
		{StackID: 1, Acknowledged: false},
	} {
		if validateStorageRecoveryRequest(request) == nil {
			t.Fatalf("accepted unsafe reconstruction request: %+v", request)
		}
	}
	if err := validateStorageRecoveryRequest(storageRecoveryRequest{StackID: 9, Acknowledged: true}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRecoveryRejectsPathEscapes(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "candidates")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(directory, "new.pack")
	if err := os.WriteFile(good, []byte("candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateStorageRecoveryArchive(good, directory); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"relative.pack", filepath.Join(root, "outside.pack"),
		filepath.Join(directory, "sub", "wrong.pack"),
		filepath.Join(directory, "wrong.txt"),
	} {
		if err := validateStorageRecoveryArchive(path, directory); err == nil {
			t.Fatalf("accepted escape or invalid archive: %s", path)
		}
	}
	alias := filepath.Join(directory, "alias.pack")
	if err := os.Symlink(good, alias); err == nil {
		if err := validateStorageRecoveryArchive(alias, directory); err == nil {
			t.Fatal("accepted symlinked candidate")
		}
	}
}

func TestNativeRecoveryProofRequiresSourceHashesAndExplicitFalseFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "extraction.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	candidate := storageCandidateProof{
		Group:               "stack:7",
		Method:              "delta",
		MediaBackupSnapshot: "20261029T102030Z-012345abcdef",
		SourceSHA256: map[string]string{
			"image:1": strings.Repeat("a", 64),
			"image:2": strings.Repeat("b", 64),
		},
	}
	proof := storageRecoveryProof{
		Verified:        true,
		ByteExact:       true,
		Complete:        true,
		SourcesRetained: true,
		Method:          candidate.Method,
		Group:           candidate.Group,
		Snapshot:        candidate.MediaBackupSnapshot,
		RestoredFiles:   2,
		OutputPath:      root,
		RestoredSHA256:  candidate.SourceSHA256,
	}
	encode := func(value storageRecoveryProof) []byte {
		t.Helper()
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return payload
	}
	if _, err := validateStorageRecoveryProof(encode(proof), candidate, preflightTestGroup(), root); err != nil {
		t.Fatalf("valid independent recovery proof rejected: %v", err)
	}
	proof.OriginalEviction = true
	if _, err := validateStorageRecoveryProof(encode(proof), candidate, preflightTestGroup(), root); err == nil {
		t.Fatal("accepted original eviction")
	}
	proof.OriginalEviction = false
	proof.DatabaseModified = true
	if _, err := validateStorageRecoveryProof(encode(proof), candidate, preflightTestGroup(), root); err == nil {
		t.Fatal("accepted catalogue modification")
	}
	proof.DatabaseModified = false
	proof.RestoredSHA256 = map[string]string{"image:1": strings.Repeat("a", 64)}
	if _, err := validateStorageRecoveryProof(encode(proof), candidate, preflightTestGroup(), root); err == nil {
		t.Fatal("accepted missing reconstruction member")
	}
	proof.RestoredSHA256 = candidate.SourceSHA256
	proof.OutputPath = filepath.Join(root, "wrong")
	if _, err := validateStorageRecoveryProof(encode(proof), candidate, preflightTestGroup(), root); err == nil {
		t.Fatal("accepted wrong reconstruction destination")
	}
}
