package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestPlanSceneTaggingReviewLocalIdentityWins(t *testing.T) {
	local := []camietagger.Tag{{Name: "Lana (Pokemon)", Category: "character", Source: "filename"}}
	booru := []camietagger.Tag{
		{Name: "Lana (Pokemon)", Category: "character", Source: "booru"},
		{Name: "Lana (Another Series)", Category: "character", Source: "booru"},
		{Name: "solo", Category: "general", Source: "booru"},
	}

	plan := planSceneTaggingReview(local, booru)
	if len(plan.AutoApply) != 2 {
		t.Fatalf("expected local Character and general tag to auto-apply, got %d", len(plan.AutoApply))
	}
	if len(plan.NeedsReview) != 1 {
		t.Fatalf("expected one conflicting identity for review, got %d", len(plan.NeedsReview))
	}
	if plan.NeedsReview[0].Reason != sceneTaggingReviewReasonLocalConflict {
		t.Fatalf("unexpected review reason %q", plan.NeedsReview[0].Reason)
	}
}

func TestPlanSceneTaggingReviewBooruFillsMissingIdentity(t *testing.T) {
	local := []camietagger.Tag{{Name: "Series", Category: "copyright", Source: "filename"}}
	booru := []camietagger.Tag{{Name: "Artist Name", Category: "artist", Source: "booru"}}

	plan := planSceneTaggingReview(local, booru)
	if len(plan.AutoApply) != 2 || len(plan.NeedsReview) != 0 {
		t.Fatalf("expected non-conflicting identity categories to auto-apply, got auto=%d review=%d", len(plan.AutoApply), len(plan.NeedsReview))
	}
}

func TestPlanSceneTaggingReviewAmbiguousCharacterNeedsReview(t *testing.T) {
	booru := []camietagger.Tag{{
		Name:     "Lana",
		Category: "character",
		Source:   "booru",
		TargetCandidates: []camietagger.TargetCandidate{
			{ID: 1, Name: "Lana", Disambiguation: "Pokemon"},
			{ID: 2, Name: "Lana", Disambiguation: "Another Series"},
		},
	}}

	plan := planSceneTaggingReview(nil, booru)
	if len(plan.AutoApply) != 0 || len(plan.NeedsReview) != 1 {
		t.Fatalf("expected ambiguous Character to require review, got auto=%d review=%d", len(plan.AutoApply), len(plan.NeedsReview))
	}
	if plan.NeedsReview[0].Reason != sceneTaggingReviewReasonAmbiguousCharacter {
		t.Fatalf("unexpected review reason %q", plan.NeedsReview[0].Reason)
	}
}

func TestPlanSceneTaggingReviewResolvedCharacterCanAutoApply(t *testing.T) {
	booru := []camietagger.Tag{{
		Name:         "Lana (Pokemon)",
		Category:     "character",
		Source:       "booru",
		TargetPath:   "/performers/1",
		TargetExists: true,
		TargetCandidates: []camietagger.TargetCandidate{
			{ID: 1, Name: "Lana", Disambiguation: "Pokemon"},
			{ID: 2, Name: "Lana", Disambiguation: "Another Series"},
		},
	}}

	plan := planSceneTaggingReview(nil, booru)
	if len(plan.AutoApply) != 1 || len(plan.NeedsReview) != 0 {
		t.Fatalf("expected explicitly resolved Character to auto-apply, got auto=%d review=%d", len(plan.AutoApply), len(plan.NeedsReview))
	}
}
