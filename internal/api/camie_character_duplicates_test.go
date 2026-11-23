package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/mock"
)

func TestReversedCamieCharacterName(t *testing.T) {
	for input, want := range map[string]string{
		"Hatsune Miku":            "Miku Hatsune",
		"hatsune_miku":            "miku hatsune",
		"Saber":                   "",
		"Artoria Pendragon Alter": "",
		"Mimi Mimi":               "",
	} {
		if got := reversedCamieCharacterName(input); got != want {
			t.Errorf("reversedCamieCharacterName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFindCamiePerformerPredictionMatchesReversedName(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	existing := &models.Performer{ID: 9, Name: "Hatsune Miku"}
	performerRepository.On("FindByNames", mock.Anything, []string{"Miku Hatsune", "miku_hatsune"}, true).Return([]*models.Performer{}, nil).Once()
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{existing}, nil).Once()
	performerRepository.On("GetAliases", mock.Anything, 9).Return([]string{}, nil).Once()
	performerRepository.On("FindByNames", mock.Anything, []string{"Hatsune Miku"}, true).Return([]*models.Performer{existing}, nil).Once()

	resolved, err := findCamiePerformerPrediction(context.Background(), repository, camietagger.Tag{Name: "miku_hatsune", Category: "character"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ID != 9 {
		t.Fatalf("expected the reversed-name Character, got %#v", resolved)
	}
	performerRepository.AssertExpectations(t)
}

func TestFindCamieReversedNamePerformerNeedsOneMatch(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	performerRepository.On("FindByNames", mock.Anything, []string{"Miku Hatsune"}, true).Return([]*models.Performer{
		{ID: 1, Name: "Miku Hatsune", Disambiguation: "Vocaloid"},
		{ID: 2, Name: "Miku Hatsune", Disambiguation: "Other"},
	}, nil)
	ctx := context.Background()
	if found, err := findCamieReversedNamePerformer(ctx, repository, "Hatsune Miku", ""); err != nil || found != nil {
		t.Fatalf("two candidates must not be a match: %#v %v", found, err)
	}
	if found, err := findCamieReversedNamePerformer(ctx, repository, "Hatsune Miku", "vocaloid"); err != nil || found == nil || found.ID != 1 {
		t.Fatalf("the matching disambiguation must win: %#v %v", found, err)
	}
}

func TestMergeBulkImageTagsTreatsReversedCharacterNamesAsOne(t *testing.T) {
	pixai := []camietagger.Tag{{Name: "hatsune_miku", Category: "character", Score: .9, Source: "pixai"}}
	camie := []camietagger.Tag{{Name: "miku_hatsune", Category: "character", Score: .95, Source: "model"}}
	merged := mergeBulkImageTags(nil, nil, pixai, camie)
	if len(merged) != 1 || merged[0].Source != "pixai" {
		t.Fatalf("reversed names must merge into the earlier source's prediction: %+v", merged)
	}
}

func TestWithoutSoloConflictCharacters(t *testing.T) {
	tag := func(name, category, source string) camietagger.Tag {
		return normalizeCamiePrediction(camietagger.Tag{Name: name, Category: category, Source: source, Score: 0.9})
	}
	models := []camietagger.Tag{
		tag("1girl", "general", "model"),
		tag("saber", "character", "model"),
		tag("artoria_pendragon", "character", "pixai"),
	}
	kept, dropped := withoutSoloConflictCharacters(models)
	if len(dropped) != 2 || len(kept) != 1 {
		t.Fatalf("1girl with two inferred Characters must drop them: kept %+v dropped %v", kept, dropped)
	}

	group := append([]camietagger.Tag{tag("2girls", "general", "model")}, models...)
	if _, dropped := withoutSoloConflictCharacters(group); len(dropped) != 0 {
		t.Fatalf("a group count allows several Characters: %v", dropped)
	}
	mixed := append([]camietagger.Tag{tag("1boy", "general", "model")}, models...)
	if _, dropped := withoutSoloConflictCharacters(mixed); len(dropped) != 0 {
		t.Fatalf("1girl and 1boy show two people: %v", dropped)
	}

	// An MD5 match may name a cosplayed Character next to the original.
	booruTags := []camietagger.Tag{tag("saber", "character", "danbooru"), tag("artoria_pendragon", "character", "danbooru")}
	trusted := []camietagger.Tag{tag("solo", "general", "danbooru"), booruTags[0], booruTags[1]}
	if _, dropped := withoutSoloConflictCharacters(trusted, booruTags); len(dropped) != 0 {
		t.Fatalf("authoritative Characters must be kept: %v", dropped)
	}
}

type fakeCamieTagger struct{ tags []camietagger.Tag }

func (f fakeCamieTagger) Status(context.Context) (camietagger.Status, error) {
	return camietagger.Status{Installed: true}, nil
}

func (f fakeCamieTagger) Tag(context.Context, string, float64, int) ([]camietagger.Tag, error) {
	return f.tags, nil
}

func (f fakeCamieTagger) Close() error { return nil }

// Bulk jobs and automatic tagging after a Booru download share this path.
func TestInferBulkImageTagsSkipsModelCharactersOnSoloImages(t *testing.T) {
	config := defaultCamieConfig()
	var request bulkImageTagRequest
	request.UseCamie = true
	camie := fakeCamieTagger{tags: []camietagger.Tag{
		{Name: "1girl", Category: "general", Score: .9},
		{Name: "saber", Category: "character", Score: .9},
		{Name: "artoria_pendragon", Category: "character", Score: .8},
	}}
	predictions, err := inferBulkImageTags(context.Background(), 1, "/library/a.png", config, request, bulkImageTagClients{camie: camie}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(predictions) != 1 || predictions[0].Category != "general" {
		t.Fatalf("model Characters on a solo Image must be skipped: %+v", predictions)
	}
}

func TestEnrichmentTargetsReversedNameCharacter(t *testing.T) {
	performerRepository := &mocks.PerformerReaderWriter{}
	repository := models.Repository{Performer: performerRepository}
	existing := &models.Performer{ID: 9, Name: "Hatsune Miku"}
	performerRepository.On("FindByNames", mock.Anything, []string{"Miku Hatsune"}, true).Return([]*models.Performer{}, nil)
	performerRepository.On("FindByNames", mock.Anything, []string{"Miku Hatsune", "miku_hatsune"}, true).Return([]*models.Performer{}, nil)
	performerRepository.On("All", mock.Anything).Return([]*models.Performer{existing}, nil)
	performerRepository.On("GetAliases", mock.Anything, 9).Return([]string{}, nil)
	performerRepository.On("FindByNames", mock.Anything, []string{"Hatsune Miku"}, true).Return([]*models.Performer{existing}, nil)

	enriched, err := enrichNativeCamiePredictionTargetsInTxn(context.Background(), repository, []camietagger.Tag{{Name: "miku_hatsune", Category: "character", Score: .9}})
	if err != nil {
		t.Fatal(err)
	}
	if len(enriched) != 1 || enriched[0].TargetPath != "/performers/9" {
		t.Fatalf("review must show the existing reversed-name Character: %+v", enriched)
	}
}
