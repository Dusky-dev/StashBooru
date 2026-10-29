package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageCandidateRequiresExplicitSafeRequest(t *testing.T) {
	ok := storageCandidateRequest{StackID: 19, Snapshot: "20261029T102030Z-012345abcdef", Method: "delta", Acknowledged: true}
	if err := validateStorageCandidateRequest(ok); err != nil {
		t.Fatal(err)
	}
	for _, req := range []storageCandidateRequest{
		{StackID: 0, Snapshot: ok.Snapshot, Method: "delta", Acknowledged: true},
		{StackID: 3, Snapshot: "../backup", Method: "delta", Acknowledged: true},
		{StackID: 3, Snapshot: ok.Snapshot, Method: "pixel-exact", Acknowledged: true},
		{StackID: 3, Snapshot: ok.Snapshot, Method: "shared-tiles", Acknowledged: false},
	} {
		if validateStorageCandidateRequest(req) == nil {
			t.Fatalf("accepted unsafe candidate request: %+v", req)
		}
	}
	ok.Method = "shared-tiles"
	if err := validateStorageCandidateRequest(ok); err != nil {
		t.Fatal(err)
	}
}

func TestStorageCandidateDirectoryProtectsMediaAndBackup(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	media := filepath.Join(root, "media")
	backup := filepath.Join(root, "backup")
	for _, dir := range []string{config, media, backup} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	dest, err := storageCandidateDirectory(config, backup, []string{media})
	if err != nil {
		t.Fatalf("separate candidate destination rejected: %v", err)
	}
	if dest != filepath.Join(config, storageCandidateDirName) {
		t.Fatalf("unexpected destination: %s", dest)
	}
	if err := prepareStorageCandidateDirectory(dest); err != nil {
		t.Fatal(err)
	}
	output, err := nextStorageCandidateOutput(dest, 19, "delta")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(output) != dest || !strings.HasPrefix(filepath.Base(output), "stack-19-delta-") {
		t.Fatalf("unsafe output: %s", output)
	}
	if _, err := storageCandidateDirectory(config, backup, []string{config}); err == nil {
		t.Fatal("allowed candidate inside configured media root")
	}
	if _, err := storageCandidateDirectory(config, config, []string{media}); err == nil {
		t.Fatal("allowed candidate inside backup store")
	}
	if _, err := storageCandidateDirectory(config, backup, []string{root}); err == nil {
		t.Fatal("allowed candidate in parent media root")
	}
	if _, err := storageCandidateDirectory("relative", backup, []string{media}); err == nil {
		t.Fatal("accepted relative config")
	}
}

func TestStorageCandidateDirectoryRejectsAliasesAndOccupiedFiles(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	backup := filepath.Join(root, "backup")
	media := filepath.Join(root, "media")
	for _, dir := range []string{config, backup, media} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(config, storageCandidateDirName)
	if err := os.Symlink(media, output); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := storageCandidateDirectory(config, backup, []string{media}); err == nil {
		t.Fatal("accepted symlinked candidate directory")
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storageCandidateDirectory(config, backup, []string{media}); err == nil {
		t.Fatal("accepted a file as candidate directory")
	}
}

func validStorageCandidateReport() map[string]any {
	return map[string]any{
		"group":	"stack:7",
		"mediaBackupSnapshot":	"20261029T102030Z-012345abcdef",
		"method":	"delta",
		"fidelity":	"byte-exact",
		"sourceFiles":	2,
		"sourceBytes":	100,
		"archiveBytes":	4,
		"sourceSHA256":	map[string]string{
			"image:1":	strings.Repeat("a", 64),
			"image:2":	strings.Repeat("b", 64),
		},
		"verified":	true,
		"backupVerified":	true,
		"sourcesRetained":	true,
		"nativePackingCandidateOnly":	true,
		"nativeServing":	false,
		"nativeServingVerified":	false,
		"databaseBackupVerified":	false,
		"originalEvictionEnabled":	false,
		"archiveActivated":	false,
	}
}

func checkCandidateReport(t *testing.T, report map[string]any, output string) error {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateStorageCandidateProof(body, preflightTestGroup(), "20261029T102030Z-012345abcdef", "delta", output)
	return err
}

func TestStorageCandidateEvidenceIsFailClosed(t *testing.T) {
	output := filepath.Join(t.TempDir(), "proof.pack")
	if err := os.WriteFile(output, []byte("PACK"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(validStorageCandidateReport())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := validateStorageCandidateProof(body, preflightTestGroup(), "20261029T102030Z-012345abcdef", "delta", output)
	if err != nil {
		t.Fatal(err)
	}
	var outputReport map[string]any
	if err := json.Unmarshal(raw, &outputReport); err != nil || outputReport["archivePath"] != output {
		t.Fatalf("candidate path was not server-derived: %v %v", outputReport, err)
	}
	for _, key := range []string{"verified", "backupVerified", "sourcesRetained", "nativePackingCandidateOnly",
		"nativeServing", "nativeServingVerified", "databaseBackupVerified", "originalEvictionEnabled", "archiveActivated"} {
		t.Run("missing_"+key, func(t *testing.T) {
			report := validStorageCandidateReport()
			delete(report, key)
			if err := checkCandidateReport(t, report, output); err == nil {
				t.Fatalf("accepted missing candidate safety field: %s", key)
			}
		})
	}
	for _, key := range []string{"nativeServing", "nativeServingVerified", "databaseBackupVerified", "originalEvictionEnabled", "archiveActivated"} {
		t.Run("enabled_"+key, func(t *testing.T) {
			report := validStorageCandidateReport()
			report[key] = true
			if err := checkCandidateReport(t, report, output); err == nil {
				t.Fatalf("accepted unsafe candidate proof: %s", key)
			}
		})
	}
	for _, key := range []string{"group", "mediaBackupSnapshot", "method", "fidelity"} {
		t.Run("wrong_"+key, func(t *testing.T) {
			report := validStorageCandidateReport()
			report[key] = "mismatch"
			if err := checkCandidateReport(t, report, output); err == nil {
				t.Fatalf("accepted mismatched candidate proof: %s", key)
			}
		})
	}
	for _, key := range []string{"sourceFiles", "sourceBytes", "archiveBytes"} {
		t.Run("wrong_"+key, func(t *testing.T) {
			report := validStorageCandidateReport()
			report[key] = 0
			if err := checkCandidateReport(t, report, output); err == nil {
				t.Fatalf("accepted invalid byte count or number of sources: %s", key)
			}
		})
	}
	bad := validStorageCandidateReport()
	bad["sourceSHA256"] = map[string]string{"image:1": strings.Repeat("a", 64)}
	if err := checkCandidateReport(t, bad, output); err == nil {
		t.Fatal("accepted missing reconstructed image digest")
	}
	if err := checkCandidateReport(t, validStorageCandidateReport(), output+"-absent"); err == nil {
		t.Fatal("accepted missing candidate archive")
	}
}
