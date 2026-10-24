package task

import (
	"os"
	"path/filepath"
	"testing"
)

type migrationBackupConfig struct{ directory string }

func (c migrationBackupConfig) GetBackupDirectoryPath() string          { return c.directory }
func (c migrationBackupConfig) GetBackupDirectoryPathOrDefault() string { return c.directory }

func TestMigrateBackupDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	valid := filepath.Join(root, "valid")
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(valid, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, configured, selected string
		wantError                  bool
	}{
		{"absolute override bypasses unavailable default", blocked, filepath.Join(valid, "backup.sqlite"), false},
		{"relative path override bypasses unavailable default", blocked, filepath.Join("valid", "backup.sqlite"), false},
		{"selected path has unavailable parent", valid, filepath.Join(blocked, "backup.sqlite"), true},
		{"filename uses configured default", blocked, "backup.sqlite", true},
		{"blank uses configured default", blocked, "", true},
		{"filename creates configured directory", filepath.Join(root, "new"), "backup.sqlite", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := MigrateJob{Config: migrationBackupConfig{directory: test.configured}, BackupPath: test.selected}
			err := job.PreExecute()
			if (err != nil) != test.wantError {
				t.Fatalf("PreExecute() = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
