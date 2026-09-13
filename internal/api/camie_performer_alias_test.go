package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/mock"
)

func TestFindOrCreateCamiePerformerPredictionReusesAlias(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	existing := &models.Performer{
		ID:             42,
		Name:           "Artoria Pendragon",
		Disambiguation: "Fate",
	}
	prediction := camietagger.Tag{
		Name:     "saber_(fate)",
		Category: "character",
		Score:    0.99,
	}

	performerRepository.On(
		"FindByNames",
		mock.Anything,
		[]string{"Saber (Fate)", "saber_(fate)", "Saber"},
		true,
	).Return([]*models.Performer{}, nil).Once()
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{existing}, nil).Once()
	performerRepository.On("GetAliases", mock.Anything, 42).Return([]string{"Saber (Fate)"}, nil).Once()

	resolved, created, err := findOrCreateCamiePerformerPrediction(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected existing Character alias to be reused, not created")
	}
	if resolved == nil || resolved.ID != existing.ID {
		t.Fatalf("expected Character %d, got %#v", existing.ID, resolved)
	}
	performerRepository.AssertExpectations(t)
}

func TestFindCamiePerformerPredictionNormalizesAliasLookup(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	existing := &models.Performer{ID: 7, Name: "Dark Princess"}
	prediction := camietagger.Tag{
		Name:     "dark_princess",
		Category: "character",
		Score:    0.9,
	}

	performerRepository.On(
		"FindByNames",
		mock.Anything,
		[]string{"Dark Princess", "dark_princess"},
		true,
	).Return([]*models.Performer{}, nil).Once()
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{existing}, nil).Once()
	performerRepository.On("GetAliases", mock.Anything, 7).Return([]string{"DARK_PRINCESS"}, nil).Once()

	resolved, err := findCamiePerformerPrediction(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != existing.ID {
		t.Fatalf("expected normalized alias to resolve Character %d, got %#v", existing.ID, resolved)
	}
	performerRepository.AssertExpectations(t)
}

func TestFindCamiePerformerPredictionRejectsAmbiguousAlias(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	first := &models.Performer{ID: 1, Name: "First Character"}
	second := &models.Performer{ID: 2, Name: "Second Character"}
	prediction := camietagger.Tag{
		Name:     "shared_alias",
		Category: "character",
		Score:    0.9,
	}

	performerRepository.On(
		"FindByNames",
		mock.Anything,
		[]string{"Shared Alias", "shared_alias"},
		true,
	).Return([]*models.Performer{}, nil).Once()
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{first, second}, nil).Once()
	performerRepository.On("GetAliases", mock.Anything, 1).Return([]string{"shared alias"}, nil).Once()
	performerRepository.On("GetAliases", mock.Anything, 2).Return([]string{"Shared_Alias"}, nil).Once()

	resolved, err := findCamiePerformerPrediction(context.Background(), repository, prediction)
	if err == nil || !strings.Contains(err.Error(), "matches multiple existing Characters") {
		t.Fatalf("expected ambiguous Character alias error, got resolved=%#v err=%v", resolved, err)
	}
	if resolved != nil {
		t.Fatalf("ambiguous alias must not resolve an arbitrary Character: %#v", resolved)
	}
	performerRepository.AssertExpectations(t)
}

func TestFindCamiePerformerPredictionDoesNotCrossDisambiguation(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	existing := &models.Performer{
		ID:             9,
		Name:           "Other Character",
		Disambiguation: "Other Series",
	}
	prediction := camietagger.Tag{
		Name:     "saber_(fate)",
		Category: "character",
		Score:    0.9,
	}

	performerRepository.On(
		"FindByNames",
		mock.Anything,
		[]string{"Saber (Fate)", "saber_(fate)", "Saber"},
		true,
	).Return([]*models.Performer{}, nil).Once()
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{existing}, nil).Once()

	resolved, err := findCamiePerformerPrediction(context.Background(), repository, prediction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != nil {
		t.Fatalf("different disambiguation must remain a distinct Character: %#v", resolved)
	}
	performerRepository.AssertExpectations(t)
}
