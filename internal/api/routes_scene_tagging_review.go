package api

import (
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
)

const (
	sceneTaggingReviewReasonAmbiguousCharacter = "ambiguous-character"
	sceneTaggingReviewReasonLocalConflict      = "local-identity-conflict"
)

type sceneTaggingReviewItem struct {
	Prediction camietagger.Tag `json:"prediction"`
	Source     string          `json:"source"`
	Reason     string          `json:"reason"`
}

type sceneTaggingReviewPlan struct {
	AutoApply   []camietagger.Tag        `json:"autoApply"`
	NeedsReview []sceneTaggingReviewItem `json:"needsReview"`
}

func sceneTaggingIdentityCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "character", "artist", "copyright":
		return true
	default:
		return false
	}
}

func sceneTaggingPredictionIdentity(prediction camietagger.Tag) string {
	prediction = normalizeCamiePrediction(prediction)
	return prediction.Category + "\x00" + strings.ToLower(strings.TrimSpace(prediction.Name))
}

func sceneTaggingPredictionNeedsReview(prediction camietagger.Tag) string {
	prediction = normalizeCamiePrediction(prediction)
	if prediction.Category == "character" && len(prediction.TargetCandidates) > 1 && !prediction.TargetExists {
		return sceneTaggingReviewReasonAmbiguousCharacter
	}
	return ""
}

// planSceneTaggingReview applies the batch Video Tagging review policy without
// mutating metadata. Local filename identity is authoritative. Matching booru
// identity is treated as duplicate provenance, while conflicting lower-priority
// identity and ambiguous same-name Characters are held for explicit review.
func planSceneTaggingReview(local, booru []camietagger.Tag) sceneTaggingReviewPlan {
	plan := sceneTaggingReviewPlan{
		AutoApply:   []camietagger.Tag{},
		NeedsReview: []sceneTaggingReviewItem{},
	}

	localIdentityCategories := make(map[string]struct{}, 3)
	localIdentityKeys := make(map[string]struct{}, len(local))
	for _, rawPrediction := range local {
		prediction := normalizeCamiePrediction(rawPrediction)
		if sceneTaggingIdentityCategory(prediction.Category) {
			localIdentityCategories[prediction.Category] = struct{}{}
			localIdentityKeys[sceneTaggingPredictionIdentity(prediction)] = struct{}{}
		}
		if reason := sceneTaggingPredictionNeedsReview(prediction); reason != "" {
			plan.NeedsReview = append(plan.NeedsReview, sceneTaggingReviewItem{
				Prediction: prediction,
				Source:     "local",
				Reason:     reason,
			})
			continue
		}
		plan.AutoApply = append(plan.AutoApply, prediction)
	}

	for _, rawPrediction := range booru {
		prediction := normalizeCamiePrediction(rawPrediction)
		if sceneTaggingIdentityCategory(prediction.Category) {
			if _, authoritative := localIdentityCategories[prediction.Category]; authoritative {
				if _, duplicate := localIdentityKeys[sceneTaggingPredictionIdentity(prediction)]; duplicate {
					continue
				}
				plan.NeedsReview = append(plan.NeedsReview, sceneTaggingReviewItem{
					Prediction: prediction,
					Source:     "booru",
					Reason:     sceneTaggingReviewReasonLocalConflict,
				})
				continue
			}
		}
		if reason := sceneTaggingPredictionNeedsReview(prediction); reason != "" {
			plan.NeedsReview = append(plan.NeedsReview, sceneTaggingReviewItem{
				Prediction: prediction,
				Source:     "booru",
				Reason:     reason,
			})
			continue
		}
		plan.AutoApply = append(plan.AutoApply, prediction)
	}

	return plan
}
