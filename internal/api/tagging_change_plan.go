package api

import (
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
)

type taggingChangeAction string

const (
	taggingChangeReuse      taggingChangeAction = "reuse"
	taggingChangeCreate     taggingChangeAction = "create"
	taggingChangeReview     taggingChangeAction = "review"
	taggingChangeSuppressed taggingChangeAction = "suppressed"
)

type taggingRelationshipMode string

const (
	taggingRelationshipAdd     taggingRelationshipMode = "add"
	taggingRelationshipReplace taggingRelationshipMode = "replace"
)

type taggingChangePlanItem struct {
	Prediction       camietagger.Tag         `json:"prediction"`
	Action           taggingChangeAction     `json:"action"`
	RelationshipMode taggingRelationshipMode `json:"relationshipMode"`
	Reason           string                  `json:"reason,omitempty"`
}

type taggingChangePlan struct {
	Items           []taggingChangePlanItem `json:"items"`
	CanApply        bool                    `json:"canApply"`
	ReviewCount     int                     `json:"reviewCount"`
	SuppressedCount int                     `json:"suppressedCount"`
}

// buildTaggingChangePlan turns enriched tagging predictions into the exact
// create/reuse/review decisions that preview and apply consume. The planner is
// side-effect free: no native entity is created until the resulting plan is
// accepted by the apply path.
func buildTaggingChangePlan(predictions []camietagger.Tag, replaceArtists bool) taggingChangePlan {
	return buildTaggingChangePlanWithSuppressed(predictions, nil, replaceArtists)
}

func buildTaggingChangePlanWithSuppressed(predictions, suppressed []camietagger.Tag, replaceArtists bool) taggingChangePlan {
	plan := taggingChangePlan{
		Items:    make([]taggingChangePlanItem, 0, len(predictions)+len(suppressed)),
		CanApply: true,
	}
	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		action := taggingChangeCreate
		if prediction.TargetExists {
			action = taggingChangeReuse
		} else if prediction.Category == "character" && len(prediction.TargetCandidates) > 1 {
			action = taggingChangeReview
			plan.ReviewCount++
			plan.CanApply = false
		}

		mode := taggingRelationshipAdd
		if strings.EqualFold(prediction.Category, "artist") && replaceArtists {
			mode = taggingRelationshipReplace
		}

		plan.Items = append(plan.Items, taggingChangePlanItem{
			Prediction:       prediction,
			Action:           action,
			RelationshipMode: mode,
		})
	}

	for _, rawPrediction := range suppressed {
		prediction := normalizeCamiePrediction(rawPrediction)
		mode := taggingRelationshipAdd
		if strings.EqualFold(prediction.Category, "artist") && replaceArtists {
			mode = taggingRelationshipReplace
		}
		plan.Items = append(plan.Items, taggingChangePlanItem{
			Prediction:       prediction,
			Action:           taggingChangeSuppressed,
			RelationshipMode: mode,
			Reason:           "suppressed by authoritative local filename identity",
		})
		plan.SuppressedCount++
	}

	return plan
}

func taggingChangePlanPredictions(plan taggingChangePlan) []camietagger.Tag {
	predictions := make([]camietagger.Tag, 0, len(plan.Items))
	for _, item := range plan.Items {
		if item.Action == taggingChangeReview || item.Action == taggingChangeSuppressed {
			continue
		}
		predictions = append(predictions, item.Prediction)
	}
	return predictions
}
