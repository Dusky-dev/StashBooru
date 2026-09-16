package api

import "time"

// scheduleSceneTaggingBatchStateExpiry bounds review-state retention without
// making a legitimately long-running batch disappear while it is still active.
// Active jobs get another TTL window; terminal states are removed on expiry.
func scheduleSceneTaggingBatchStateExpiry(jobID int) {
	time.AfterFunc(sceneTaggingBatchStateTTL, func() {
		value, ok := sceneTaggingBatchStates.Load(jobID)
		if !ok {
			return
		}
		state, ok := value.(*sceneTaggingBatchState)
		if !ok {
			sceneTaggingBatchStates.Delete(jobID)
			return
		}
		switch state.snapshot().Status {
		case sceneTaggingBatchStatusQueued, sceneTaggingBatchStatusRunning:
			scheduleSceneTaggingBatchStateExpiry(jobID)
		default:
			sceneTaggingBatchStates.Delete(jobID)
		}
	})
}
