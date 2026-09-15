package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

func (r *camieTestPerformerRepository) Find(_ context.Context, id int) (*models.Performer, error) {
	for _, performerEntity := range r.performers {
		if performerEntity != nil && performerEntity.ID == id {
			return performerEntity, nil
		}
	}
	return nil, nil
}

func TestFindCamieExplicitCharacterTargetRejectsWrongSameNameDisambiguation(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana", Disambiguation: "Pokemon"}
	fireEmblemLana := &models.Performer{ID: 2, Name: "Lana", Disambiguation: "Fire Emblem"}
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana, fireEmblemLana},
		[]*models.Copyright{pokemon},
		nil,
	)

	prediction := camietagger.Tag{
		Name:         "lana_(pokemon)",
		RawName:      "lana_(pokemon)",
		Category:     "character",
		TargetExists: true,
		TargetPath:   "/performers/2",
	}

	resolved, err := findCamieExplicitCharacterTarget(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != nil {
		t.Fatalf("wrong-series same-name explicit target must be rejected, got %#v", resolved)
	}
}

func TestFindCamieExplicitCharacterTargetAcceptsMatchingDisambiguation(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana", Disambiguation: "Pokemon"}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana},
		nil,
		nil,
	)
	prediction := camietagger.Tag{
		Name:         "lana_(pokemon)",
		RawName:      "lana_(pokemon)",
		Category:     "character",
		TargetExists: true,
		TargetPath:   "/performers/1",
	}

	resolved, err := findCamieExplicitCharacterTarget(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemonLana.ID {
		t.Fatalf("expected matching explicit target, got %#v", resolved)
	}
}

func TestFindCamieExplicitCharacterTargetAcceptsNativeCopyrightContext(t *testing.T) {
	pokemonLana := &models.Performer{ID: 1, Name: "Lana"}
	pokemon := &models.Copyright{ID: 10, Name: "Pokemon"}
	repository := camieCharacterCopyrightTestRepository(
		[]*models.Performer{pokemonLana},
		[]*models.Copyright{pokemon},
		map[int][]*models.Copyright{pokemonLana.ID: {pokemon}},
	)
	prediction := camietagger.Tag{
		Name:         "lana_(pokemon)",
		RawName:      "lana_(pokemon)",
		Category:     "character",
		TargetExists: true,
		TargetPath:   "/performers/1",
	}

	resolved, err := findCamieExplicitCharacterTarget(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != pokemonLana.ID {
		t.Fatalf("expected native Copyright relation to validate explicit target, got %#v", resolved)
	}
}
