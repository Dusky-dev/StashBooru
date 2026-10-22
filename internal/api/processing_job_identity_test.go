package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/job"
)

func TestProcessingJobReferencesRejectReusedIDs(t *testing.T) {
	oldBatch, newBatch := strings.Repeat("a", 32), strings.Repeat("b", 32)
	// A fresh process can assign the same numeric ID to unrelated work.
	registry := processingJobRegistry{jobs: map[int]*conversionJob{
		1: {JobID: 1, Batch: newBatch, Status: "queued", Items: []conversionItem{{Target: conversionTarget{Kind: "image", ID: 7}}}},
	}}
	for _, batch := range []string{"", oldBatch} {
		if got := registry.snapshot(1, batch, nil); got != nil {
			t.Fatalf("exposed reused job for batch %q", batch)
		}
		called := false
		if registry.cancel(1, batch, func(int) { called = true }) || called {
			t.Fatalf("cancelled reused job for batch %q", batch)
		}
		if registry.jobs[1].Status != "queued" {
			t.Fatal("stale cancellation changed replacement job state")
		}
	}
	snapshot := registry.snapshot(1, newBatch, nil)
	if snapshot == nil || snapshot.Batch != newBatch {
		t.Fatal("matching reference failed to resume")
	}
	snapshot.Items[0].Target.ID = 99
	if registry.jobs[1].Items[0].Target.ID != 7 {
		t.Fatal("snapshot aliases live result items")
	}
	if !registry.cancel(1, newBatch, func(id int) {
		if id != 1 {
			t.Fatal("wrong native target")
		}
	}) {
		t.Fatal("matching cancellation rejected")
	}
	if registry.jobs[1].Status != "cancelled" {
		t.Fatal("queued job cancellation not reconciled")
	}
	delete(registry.jobs, 1)
	if registry.snapshot(1, newBatch, nil) != nil {
		t.Fatal("expired result resumed")
	}
	if registry.cancel(1, newBatch, func(int) { t.Fatal("cancel invoked for expired result") }) {
		t.Fatal("expired result cancelled")
	}
}

func TestProcessingJobNamespaceAndNativeCancellation(t *testing.T) {
	manager := job.NewManager()
	defer manager.Stop()
	started, stopped := make(chan struct{}), make(chan struct{})
	id := manager.Add(context.Background(), "bounded cancellation fixture", job.MakeJobExec(func(ctx context.Context, _ *job.Progress) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("native job did not start")
	}
	batch := strings.Repeat("c", 32)
	registry := processingJobRegistry{jobs: map[int]*conversionJob{id: {JobID: id, Batch: batch, Status: "running"}}}
	otherFeature := processingJobRegistry{jobs: map[int]*conversionJob{id: {JobID: id, Batch: strings.Repeat("d", 32), Status: "running"}}}
	if otherFeature.cancel(id, batch, manager.CancelJob) {
		t.Fatal("reference crossed feature registries")
	}
	if status := manager.GetJob(id); status == nil || status.Status == job.StatusStopping || status.Status == job.StatusCancelled {
		t.Fatal("wrong namespace interrupted native work")
	}
	if !registry.cancel(id, batch, manager.CancelJob) {
		t.Fatal("current reference could not cancel")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("native work did not receive cancellation")
	}
}
