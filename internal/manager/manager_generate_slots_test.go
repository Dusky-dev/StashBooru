package manager

import (
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
)

// ScanNow and queued Scan jobs must use the same cap, not multiply parallelTasks.
func TestScanGenerateJobsShareTaskSlots(t *testing.T) {
	config.InitializeEmpty()
	m := &Manager{Config: config.GetInstance()}
	fromScan := m.generateTaskSlots()
	fromScanNow := m.generateTaskSlots()
	if fromScan != fromScanNow {
		t.Fatal("ScanNow and queued Scan must share the same generation task slots")
	}
	if cap(fromScan) != m.Config.GetParallelTasksWithAutoDetection() {
		t.Fatalf("shared task limit %d does not match configured limit", cap(fromScan))
	}
}
