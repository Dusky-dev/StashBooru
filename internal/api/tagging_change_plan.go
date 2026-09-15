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
	Prediction       camietagger.Tag
	Action           taggingChangeAction
	RelationshipMode taggingRelationshipMode
}

type taggingChangePlan struct {
	Items       []taggingChangePlanItem
	CanApply    bool
	ReviewCount int
}

// buildTaggingChangePlan turns enriched tagging predictions into the exact
// create/reuse/review decisions that an apply path can consume later. It is
// side-effect free so preview and apply can share the same policy.
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
