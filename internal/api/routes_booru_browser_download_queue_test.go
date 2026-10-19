package api

import (
	"testing"
	"time"
)

func TestBooruDownloadQueueReclaimsOldestTerminalJob(t *testing.T) {
	now := time.Now()
	states := map[int]*booruDownloadState{
		1: {Status: "queued"},
		2: {Status: "downloading"},
		3: {Status: "complete", FinishedAt: now.Add(-time.Minute)},
		4: {Status: "failed", FinishedAt: now},
	}
	if !reclaimBooruDownloadSlot(states, 4) {
		t.Fatal("finished jobs should free queue capacity")
	}
	if len(states) != 3 || states[3] != nil || states[1] == nil || states[2] == nil || states[4] == nil {
		t.Fatalf("wrong job evicted; active jobs must be retained: %+v", states)
	}
}

func TestBooruDownloadQueueReclaimsCancelledJob(t *testing.T) {
	states := map[int]*booruDownloadState{
		1: {Status: "cancelled", FinishedAt: time.Now()},
		2: {Status: "importing"},
	}
	if !reclaimBooruDownloadSlot(states, 2) || states[1] != nil || states[2] == nil {
		t.Fatalf("cancelled job did not free queue slot: %+v", states)
	}
}

func TestBooruDownloadQueueRejectsOnlyActiveJobs(t *testing.T) {
	states := map[int]*booruDownloadState{
		1: {Status: "queued"},
		2: {Status: "downloading"},
		3: {Status: "importing/converting"},
	}
	if reclaimBooruDownloadSlot(states, len(states)) {
		t.Fatal("all-active queue should not accept another job")
	}
	if len(states) != 3 {
		t.Fatal("active job was removed to make room")
	}
}

func TestBooruDownloadQueueRecoversFromHundredCompletedJobs(t *testing.T) {
	states := make(map[int]*booruDownloadState)
	for i := 1; i <= booruDownloadJobLimit; i++ {
		states[i] = &booruDownloadState{
			Status:     "complete",
			FinishedAt: time.Unix(int64(i), 0),
		}
	}
	if !reclaimBooruDownloadSlot(states, booruDownloadJobLimit) {
		t.Fatal("full registry of completed jobs should not block new downloads")
	}
	if len(states) != booruDownloadJobLimit-1 || states[1] != nil {
		t.Fatal("oldest completed job was not evicted")
	}
	states[101] = &booruDownloadState{Status: "queued"}
	if len(states) != booruDownloadJobLimit || !reclaimBooruDownloadSlot(states, booruDownloadJobLimit) {
		t.Fatal("new requests should continue to reuse completed slots")
	}
	if states[101] == nil || states[2] != nil {
		t.Fatal("queued request was evicted instead of an older completed job")
	}
}

func TestBooruDownloadQueuePreservesSlotsBelowLimit(t *testing.T) {
	states := map[int]*booruDownloadState{
		1: {Status: "complete", FinishedAt: time.Now()},
	}
	if !reclaimBooruDownloadSlot(states, 2) || states[1] == nil {
		t.Fatal("completed jobs should remain visible while room exists")
	}
}
