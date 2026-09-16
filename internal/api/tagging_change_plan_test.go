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

func TestBuildTaggingChangePlanRepresentsSuppressedPredictions(t *testing.T) {
	selected := []camietagger.Tag{{
		Name:         "Local Character",
		Category:     "character",
		Source:       "filename",
		TargetExists: true,
		TargetPath:   "/performers/1",
	}}
	suppressed := []camietagger.Tag{{
		Name:     "Booru Character",
		Category: "character",
		Source:   "danbooru",
	}}

	plan := buildTaggingChangePlanWithSuppressed(selected, suppressed, false)
	if !plan.CanApply || plan.SuppressedCount != 1 || len(plan.Items) != 2 {
		t.Fatalf("unexpected plan with suppressed prediction: %+v", plan)
	}
	if plan.Items[1].Action != taggingChangeSuppressed || plan.Items[1].Reason == "" {
		t.Fatalf("expected explained suppressed action, got %+v", plan.Items[1])
	}

	apply := taggingChangePlanPredictions(plan)
	if len(apply) != 1 || apply[0].Name != "Local Character" {
		t.Fatalf("suppressed prediction leaked into apply set: %+v", apply)
	}
}

func TestPartitionCamieFilenameAuthoritativeSelections(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "Local Character", Category: "character", Source: "filename"},
		{Name: "Booru Character", Category: "character", Source: "danbooru"},
		{Name: "solo", Category: "general", Source: "danbooru"},
	}

	kept, suppressed := partitionCamieFilenameAuthoritativeSelections(predictions)
	if len(kept) != 2 || len(suppressed) != 1 {
		t.Fatalf("unexpected partition: kept=%+v suppressed=%+v", kept, suppressed)
	}
	if suppressed[0].Name != "Booru Character" {
		t.Fatalf("unexpected suppressed prediction: %+v", suppressed[0])
	}
}
