package api

import "github.com/stashapp/stash/pkg/camietagger"

// validateSceneTaggingPredictions enforces the same filename-authoritative
// identity policy used by Image Tagging before Video Tagging selections can
// reach native entity resolution or creation.
func validateSceneTaggingPredictions(predictions []camietagger.Tag) ([]camietagger.Tag, error) {
	prioritized := filterCamieFilenameAuthoritativeSelections(predictions)
	return validateCamiePredictionsV2(prioritized)
}
