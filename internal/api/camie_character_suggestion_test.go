package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

func TestSelectCamieBarePerformerSuggestion(t *testing.T) {
	bare := &models.Performer{ID: 1, Name: "Darkness", Disambiguation: ""}
	otherVersion := &models.Performer{ID: 2, Name: "Darkness", Disambiguation: "Isekai Quartet"}

	got := selectCamieBarePerformerSuggestion(
		[]*models.Performer{otherVersion, bare},
		"Darkness",
		"Konosuba",
	)
	if got == nil || got.ID != bare.ID {
		t.Fatalf("expected bare same-name Character to be suggested, got %#v", got)
	}
}

func TestSelectCamieBarePerformerSuggestionExcludesDifferentDisambiguation(t *testing.T) {
	got := selectCamieBarePerformerSuggestion(
		[]*models.Performer{
			{ID: 2, Name: "Darkness", Disambiguation: "Isekai Quartet"},
		},
		"Darkness",
		"Konosuba",
	)
	if got != nil {
		t.Fatalf("expected differently-disambiguated Character to be excluded, got %#v", got)
	}
}

func TestSelectCamieBarePerformerSuggestionRequiresPredictedDisambiguation(t *testing.T) {
	got := selectCamieBarePerformerSuggestion(
		[]*models.Performer{{ID: 1, Name: "Darkness", Disambiguation: ""}},
		"Darkness",
		"",
	)
	if got != nil {
		t.Fatalf("plain Character predictions should not trigger an ambiguity prompt, got %#v", got)
	}
}
