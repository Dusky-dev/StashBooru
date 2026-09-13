package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type lanaCharacterContextFixture struct {
	repository     models.Repository
	pokemonLana    *models.Performer
	fireEmblemLana *models.Performer
}

func lanaCharacterContextTestRepository() lanaCharacterContextFixture {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana", Disambiguation: "Pokemon"}
	fireEmblemLana := &models.Performer{ID: 2, Name: "Lana", Disambiguation: "Fire Emblem"}
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	fireEmblem := &models.Copyright{ID: 11, Name: "Fire Emblem"}

	return lanaCharacterContextFixture{
		repository: models.Repository{
			Performer: &camieTestPerformerRepository{
				performers: []*models.Performer{pokemonLana, fireEmblemLana},
				aliases: map[int][]string{
					pokemonLana.ID:    {"Lana (Pokemon)"},
					fireEmblemLana.ID: {"Lana (Fire Emblem)"},
				},
			},
			Copyright: &camieTestCopyrightRepository{
				copyrights: []*models.Copyright{pokemon, fireEmblem},
				relations: map[int][]*models.Copyright{
					pokemonLana.ID:    {pokemon},
					fireEmblemLana.ID: {fireEmblem},
				},
			},
		},
		pokemonLana:    pokemonLana,
		fireEmblemLana: fireEmblemLana,
	}
}

func TestCamieBareLanaDoesNotUseModelFireEmblemCopyrightForIdentity(t *testing.T) {
	fixture := lanaCharacterContextTestRepository()
	character := camietagger.Tag{
		Name:     "lana",
		RawName:  "lana",
		Category: "character",
		Source:   "model",
	}
	wrongCopyright := camietagger.Tag{
		Name:     "fire_emblem",
		RawName:  "fire_emblem",
		Category: "copyright",
		Source:   "model",
	}

	resolved, err := findCamiePerformerPredictionWithCopyrightContext(
		context.Background(),
		fixture.repository,
		character,
		[]camietagger.Tag{character, wrongCopyright},
	)
	if err == nil {
		t.Fatalf("expected bare Lana to remain ambiguous, got %#v", resolved)
	}
	if resolved != nil && resolved.ID == fixture.fireEmblemLana.ID {
		t.Fatalf("model Copyright must never silently resolve Lana to Fire Emblem: %#v", resolved)
	}
	if !strings.Contains(err.Error(), "matches multiple existing Characters") {
		t.Fatalf("expected same-name ambiguity error, got %v", err)
	}
}

func TestCamieBareLanaUsesTrustedPokemonCopyrightContext(t *testing.T) {
	fixture := lanaCharacterContextTestRepository()
	character := camietagger.Tag{
		Name:     "lana",
		RawName:  "lana",
		Category: "character",
		Source:   "model",
	}
	pokemonCopyright := camietagger.Tag{
		Name:     "pokemon",
		RawName:  "pokemon",
		Category: "copyright",
		Source:   "filename",
	}

	resolved, err := findCamiePerformerPredictionWithCopyrightContext(
		context.Background(),
		fixture.repository,
		character,
		[]camietagger.Tag{character, pokemonCopyright},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != fixture.pokemonLana.ID {
		t.Fatalf("expected trusted Pokemon context to resolve Lana (%d), got %#v", fixture.pokemonLana.ID, resolved)
	}
}

func TestCamieExplicitPokemonLanaOutranksModelFireEmblemCopyright(t *testing.T) {
	fixture := lanaCharacterContextTestRepository()
	character := camietagger.Tag{
		Name:     "lana_(pokemon)",
		RawName:  "lana_(pokemon)",
		Category: "character",
		Source:   "model",
	}
	wrongCopyright := camietagger.Tag{
		Name:     "fire_emblem",
		RawName:  "fire_emblem",
		Category: "copyright",
		Source:   "model",
	}

	resolved, err := findCamiePerformerPredictionWithCopyrightContext(
		context.Background(),
		fixture.repository,
		character,
		[]camietagger.Tag{character, wrongCopyright},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != fixture.pokemonLana.ID {
		t.Fatalf("expected explicit Lana (Pokemon) to resolve Character %d, got %#v", fixture.pokemonLana.ID, resolved)
	}
}
