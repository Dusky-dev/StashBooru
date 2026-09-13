package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type camieTestPerformerRepository struct {
	models.PerformerReaderWriter
	performers []*models.Performer
	aliases    map[int][]string
}

func (r *camieTestPerformerRepository) FindByNames(_ context.Context, names []string, _ bool) ([]*models.Performer, error) {
	result := []*models.Performer{}
	for _, performerEntity := range r.performers {
		for _, name := range names {
			if strings.EqualFold(strings.TrimSpace(performerEntity.Name), strings.TrimSpace(name)) {
				result = append(result, performerEntity)
				break
			}
		}
	}
	return result, nil
}

func (r *camieTestPerformerRepository) GetAliases(_ context.Context, id int) ([]string, error) {
	return append([]string(nil), r.aliases[id]...), nil
}

type camieTestCopyrightRepository struct {
	models.CopyrightReaderWriter
	copyrights []*models.Copyright
	relations  map[int][]*models.Copyright
}

func (r *camieTestCopyrightRepository) FindByName(_ context.Context, name string, _ bool) (*models.Copyright, error) {
	for _, copyrightEntity := range r.copyrights {
		if strings.EqualFold(strings.TrimSpace(copyrightEntity.Name), strings.TrimSpace(name)) {
			return copyrightEntity, nil
		}
	}
	return nil, nil
}

func (r *camieTestCopyrightRepository) FindByAlias(_ context.Context, alias string, _ bool) (*models.Copyright, error) {
	for _, copyrightEntity := range r.copyrights {
		for _, candidate := range copyrightEntity.Aliases {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(alias)) {
				return copyrightEntity, nil
			}
		}
	}
	return nil, nil
}

func (r *camieTestCopyrightRepository) FindByPerformerID(_ context.Context, performerID int) ([]*models.Copyright, error) {
	return append([]*models.Copyright(nil), r.relations[performerID]...), nil
}

func camieCharacterCopyrightTestRepository(performers []*models.Performer, copyrights []*models.Copyright, relations map[int][]*models.Copyright) models.Repository {
	return models.Repository{
		Performer: &camieTestPerformerRepository{
			performers: performers,
			aliases:    map[int][]string{},
		},
		Copyright: &camieTestCopyrightRepository{
			copyrights: copyrights,
			relations:  relations,
		},
	}
}

func TestCamieCharacterResolutionUsesCopyrightDisambiguation(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana", Disambiguation: "Pokemon"}
	fireEmblemLana := &models.Performer{ID: 2, Name: "Lana", Disambiguation: "Fire Emblem"}
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana, fireEmblemLana},
		[]*models.Copyright{pokemon},
		nil,
	)

	character := camietagger.Tag{Name: "lana", RawName: "lana", Category: "character"}
	copyright := camietagger.Tag{Name: "pokemon", RawName: "pokemon", Category: "copyright"}
	resolved, err := findCamiePerformerPredictionWithCopyrightContext(context.Background(), repository, character, []camietagger.Tag{character, copyright})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemonLana.ID {
		t.Fatalf("expected Pokemon Lana (%d), got %#v", pokemonLana.ID, resolved)
	}
}

func TestCamieCharacterResolutionUsesNativeCopyrightRelation(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana"}
	fireEmblemLana := &models.Performer{ID: 2, Name: "Lana"}
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	fireEmblem := &models.Copyright{ID: 11, Name: "Fire Emblem"}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana, fireEmblemLana},
		[]*models.Copyright{pokemon, fireEmblem},
		map[int][]*models.Copyright{
			pokemonLana.ID:    {pokemon},
			fireEmblemLana.ID: {fireEmblem},
		},
	)

	character := camietagger.Tag{Name: "lana", RawName: "lana", Category: "character"}
	copyright := camietagger.Tag{Name: "pokemon", RawName: "pokemon", Category: "copyright"}
	resolved, err := findCamiePerformerPredictionWithCopyrightContext(context.Background(), repository, character, []camietagger.Tag{character, copyright})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemonLana.ID {
		t.Fatalf("expected Copyright-linked Pokemon Lana (%d), got %#v", pokemonLana.ID, resolved)
	}
}

func TestCamieCharacterResolutionUsesCopyrightAlias(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana", Disambiguation: "Pocket Monsters"}
	fireEmblemLana := &models.Performer{ID: 2, Name: "Lana", Disambiguation: "Fire Emblem"}
	pokemon := &models.Copyright{ID: 10, Name: "Pocket Monsters", Aliases: []string{"Pokemon"}}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana, fireEmblemLana},
		[]*models.Copyright{pokemon},
		nil,
	)

	prediction := camietagger.Tag{Name: "lana_(pokemon)", RawName: "lana_(pokemon)", Category: "character"}
	resolved, err := findCamiePerformerPredictionWithCopyrightContext(context.Background(), repository, prediction, []camietagger.Tag{prediction})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemonLana.ID {
		t.Fatalf("expected Copyright alias to resolve Pokemon Lana (%d), got %#v", pokemonLana.ID, resolved)
	}
}

func TestCamieCharacterResolutionRejectsUnresolvedDuplicateName(t *testing.T) {
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{
			{ID: 1, Name: "Lana", Disambiguation: "Pokemon"},
			{ID: 2, Name: "Lana", Disambiguation: "Fire Emblem"},
		},
		nil,
		nil,
	)

	prediction := camietagger.Tag{Name: "lana", RawName: "lana", Category: "character"}
	resolved, err := findCamiePerformerPredictionWithCopyrightContext(context.Background(), repository, prediction, []camietagger.Tag{prediction})
	if err == nil {
		t.Fatalf("expected ambiguity error, got Character %#v", resolved)
	}
	if !strings.Contains(err.Error(), "Copyright (series)") {
		t.Fatalf("expected actionable ambiguity error, got %v", err)
	}
}

func TestCamieCopyrightMatchUsesAliases(t *testing.T) {
	pokemon := &models.Copyright{ID: 10, Name: "Pocket Monsters", Aliases: []string{"Pokemon", "Pokemon (game)"}}
	if !camieCopyrightMatchesValue(pokemon, "pokemon") {
		t.Fatal("expected Copyright alias to match Character disambiguation")
	}
	if camieCopyrightMatchesValue(pokemon, "Fire Emblem") {
		t.Fatal("unexpected Copyright alias match")
	}
}

func TestFindCamieCopyrightForCharacterAvoidsCrossoverGuess(t *testing.T) {
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	fireEmblem := &models.Copyright{ID: 11, Name: "Fire Emblem"}
	repository := models.Repository{}

	prediction := camietagger.Tag{Name: "lana_(pokemon)", RawName: "lana_(pokemon)", Category: "character"}
	resolved, err := findCamieCopyrightForCharacter(context.Background(), repository, prediction, nil, []*models.Copyright{pokemon, fireEmblem})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemon.ID {
		t.Fatalf("expected only Pokemon Copyright, got %#v", resolved)
	}
}

func TestFindCamieCopyrightForCharacterDisambiguationOutranksSoleConflictingCopyright(t *testing.T) {
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	fireEmblem := &models.Copyright{ID: 11, Name: "Fire Emblem"}
	repository := camieCharacterCopyrightTestRepository(nil, []*models.Copyright{pokemon, fireEmblem}, nil)

	prediction := camietagger.Tag{Name: "lana_(pokemon)", RawName: "lana_(pokemon)", Category: "character"}
	resolved, err := findCamieCopyrightForCharacter(context.Background(), repository, prediction, nil, []*models.Copyright{fireEmblem})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemon.ID {
		t.Fatalf("expected disambiguated Pokemon Copyright instead of conflicting Fire Emblem, got %#v", resolved)
	}
}

func TestFindCamieCopyrightForCharacterDoesNotAttachUnknownConflictingCopyright(t *testing.T) {
	fireEmblem := &models.Copyright{ID: 11, Name: "Fire Emblem"}
	repository := camieCharacterCopyrightTestRepository(nil, []*models.Copyright{fireEmblem}, nil)

	prediction := camietagger.Tag{Name: "lana_(pokemon)", RawName: "lana_(pokemon)", Category: "character"}
	resolved, err := findCamieCopyrightForCharacter(context.Background(), repository, prediction, nil, []*models.Copyright{fireEmblem})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != nil {
		t.Fatalf("expected no Copyright link rather than conflicting Fire Emblem, got %#v", resolved)
	}
}
