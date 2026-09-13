package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestCamieBareSameNameCharacterReturnsReviewCandidates(t *testing.T) {
	fixture := lanaCharacterContextTestRepository()
	prediction := camietagger.Tag{
		Name:     "lana",
		RawName:  "lana",
		Category: "character",
		Source:   "model",
	}

	candidates, err := findCamieCharacterTargetCandidates(
		context.Background(),
		fixture.repository,
		prediction,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected both same-name Characters as review candidates, got %#v", candidates)
	}

	byID := make(map[int]camietagger.TargetCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	pokemon, ok := byID[fixture.pokemonLana.ID]
	if !ok || pokemon.Name != "Lana" || pokemon.Disambiguation != "Pokemon" {
		t.Fatalf("missing Pokemon Lana candidate: %#v", candidates)
	}
	fireEmblem, ok := byID[fixture.fireEmblemLana.ID]
	if !ok || fireEmblem.Name != "Lana" || fireEmblem.Disambiguation != "Fire Emblem" {
		t.Fatalf("missing Fire Emblem Lana candidate: %#v", candidates)
	}
}

func TestCamieDisambiguatedCharacterDoesNotReturnAmbiguousCandidateList(t *testing.T) {
	fixture := lanaCharacterContextTestRepository()
	prediction := camietagger.Tag{
		Name:     "lana_(pokemon)",
		RawName:  "lana_(pokemon)",
		Category: "character",
		Source:   "model",
	}

	candidates, err := findCamieCharacterTargetCandidates(
		context.Background(),
		fixture.repository,
		prediction,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("explicitly disambiguated Character must not need a candidate chooser: %#v", candidates)
	}
}
