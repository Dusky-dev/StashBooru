package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestBuildTaggingChangePlan(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "Existing Character", Category: "character", TargetExists: true, TargetPath: "/performers/1"},
		{Name: "New Copyright", Category: "copyright"},
		{Name: "Artist", Category: "artist", TargetExists: true, TargetPath: "/studios/2"},
	}

	plan := buildTaggingChangePlan(predictions, true)
	if !plan.CanApply || plan.ReviewCount != 0 || len(plan.Items) != 3 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Items[0].Action != taggingChangeReuse {
		t.Fatalf("expected existing Character reuse, got %q", plan.Items[0].Action)
	}
	if plan.Items[1].Action != taggingChangeCreate {
		t.Fatalf("expected missing Copyright create, got %q", plan.Items[1].Action)
	}
	if plan.Items[2].RelationshipMode != taggingRelationshipReplace {
		t.Fatalf("expected Artist replace mode, got %q", plan.Items[2].RelationshipMode)
	}
}

func TestBuildTaggingChangePlanRequiresReviewForAmbiguousCharacter(t *testing.T) {
	predictions := []camietagger.Tag{{
		Name:     "Lana",
		Category: "character",
		TargetCandidates: []camietagger.TargetCandidate{
			{ID: 1, Name: "Lana", Disambiguation: "Pokemon"},
			{ID: 2, Name: "Lana", Disambiguation: "Another Series"},
		},
	}}

	plan := buildTaggingChangePlan(predictions, false)
	if plan.CanApply || plan.ReviewCount != 1 || len(plan.Items) != 1 {
		t.Fatalf("unexpected ambiguous plan: %+v", plan)
	}
	if plan.Items[0].Action != taggingChangeReview {
		t.Fatalf("expected review action, got %q", plan.Items[0].Action)
	}
}
