package api

import (
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
)

type taggingChangeAction string

const (
	taggingChangeReuse  taggingChangeAction = "reuse"
	taggingChangeCreate taggingChangeAction = "create"
	taggingChangeReview taggingChangeAction = "review"
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
}

type taggingChangePlan struct {
	Items       []taggingChangePlanItem `json:"items"`
	CanApply    bool                    `json:"canApply"`
	ReviewCount int                     `json:"reviewCount"`
}

// buildTaggingChangePlan turns enriched tagging predictions into the exact
// create/reuse/review decisions that preview and apply consume. The planner is
// side-effect free: no native entity is created until the resulting plan is
// accepted by the apply path.
func buildTaggingChangePlan(predictions []camietagger.Tag, replaceArtists bool) taggingChangePlan {
	plan := taggingChangePlan{Items: make([]taggingChangePlanItem, 0, len(predictions)), CanApply: true}
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
	return plan
}

func taggingChangePlanPredictions(plan taggingChangePlan) []camietagger.Tag {
	predictions := make([]camietagger.Tag, 0, len(plan.Items))
	for _, item := range plan.Items {
		if item.Action == taggingChangeReview {
			continue
		}
		predictions = append(predictions, item.Prediction)
	}
	return predictions
}
