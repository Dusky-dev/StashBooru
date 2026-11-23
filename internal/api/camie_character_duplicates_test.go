package api

import (
	"context"
	"encoding/json"
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
	merged := mergeBulkImageTags(true, nil, nil, pixai, camie)
	if len(merged) != 1 || merged[0].Source != "pixai" {
		t.Fatalf("reversed names must merge into the earlier source's prediction: %+v", merged)
	}
}

func TestCamieDepictsOneCharacterAvoidsFalsePositives(t *testing.T) {
	general := func(names ...string) []camietagger.Tag {
		var tags []camietagger.Tag
		for _, name := range names {
			tags = append(tags, camietagger.Tag{Name: name, Category: "general"})
		}
		return tags
	}
	for _, tags := range [][]camietagger.Tag{
		general("1girl"),                    // count alone: others may be untagged
		general("solo", "2girls"),           // contradictory tags
		general("solo", "solo_focus"),       // one of several people
		general("solo", "cosplay"),          // one person as another Character
		general("solo", "multiple_views"),   // one Character drawn twice
		general("solo", "Multiple Persona"), // normalised spelling
		general("solo", "multiple_boys"),
	} {
		if camieDepictsOneCharacter(tags) {
			t.Errorf("%v must not count as one person", tags)
		}
	}
	if !camieDepictsOneCharacter(general("solo", "1girl", "smile")) {
		t.Error("solo with a single count is one person")
	}
}

func TestWithoutSoloConflictCharacters(t *testing.T) {
	character := func(name, source string) camietagger.Tag {
		return camietagger.Tag{Name: name, Category: "character", Source: source, Score: 0.9}
	}
	predictions := []camietagger.Tag{
		{Name: "solo", Category: "general"},
		character("Saber", "model"),
		character("Artoria Pendragon", "pixai"),
	}
	kept, dropped := withoutSoloConflictCharacters(predictions, predictions, true)
	if len(dropped) != 2 || len(kept) != 1 {
		t.Fatalf("two different model Characters on a solo Image must be skipped: kept %+v dropped %v", kept, dropped)
	}
	if _, dropped := withoutSoloConflictCharacters(predictions, predictions, false); len(dropped) != 0 {
		t.Fatal("nothing is skipped when the Image is not one person or the check is off")
	}

	// Two names for one existing Character are not a conflict.
	enriched := append([]camietagger.Tag(nil), predictions...)
	enriched[1].TargetExists, enriched[1].TargetPath = true, "/performers/4"
	enriched[2].TargetExists, enriched[2].TargetPath = true, "/performers/4"
	if _, dropped := withoutSoloConflictCharacters(predictions, enriched, true); len(dropped) != 0 {
		t.Fatalf("aliases of one Character must be kept: %v", dropped)
	}
	reversed := []camietagger.Tag{character("Hatsune Miku", "model"), character("miku_hatsune", "pixai")}
	if _, dropped := withoutSoloConflictCharacters(reversed, reversed, true); len(dropped) != 0 {
		t.Fatalf("one Character in both name orders must be kept: %v", dropped)
	}

	// Filename and MD5-match Characters are always kept and not counted.
	trusted := []camietagger.Tag{character("Saber", "filename"), character("Archer", "booru:danbooru"), character("Rin", "model")}
	if kept, dropped := withoutSoloConflictCharacters(trusted, trusted, true); len(dropped) != 0 || len(kept) != 3 {
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

func TestDuplicateDetectionSettingsGateReversedMerge(t *testing.T) {
	config := defaultCamieConfig()
	var request bulkImageTagRequest
	request.UseCamie = true
	camie := fakeCamieTagger{tags: []camietagger.Tag{
		{Name: "hatsune_miku", Category: "character", Score: .9},
		{Name: "miku_hatsune", Category: "character", Score: .8},
	}}
	run := func() int {
		t.Helper()
		predictions, err := inferBulkImageTags(context.Background(), 1, "/library/a.png", config, request, bulkImageTagClients{camie: camie}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return len(predictions)
	}
	if got := run(); got != 1 {
		t.Fatalf("reversed names must merge by default, got %d", got)
	}
	config.DuplicateDetection.ReversedNames = false
	if got := run(); got != 2 {
		t.Fatalf("reversed-name merging must follow its setting, got %d", got)
	}
	config.DuplicateDetection = defaultDuplicateDetectionConfig()
	config.DuplicateDetection.Enabled = false
	if got := run(); got != 2 {
		t.Fatalf("the main switch must turn every mode off, got %d", got)
	}
}

func TestDuplicateDetectionDefaultsSurviveOldConfigFiles(t *testing.T) {
	config := defaultCamieConfig()
	if err := json.Unmarshal([]byte(`{"threshold":0.5,"duplicateDetection":{"onePerson":false}}`), &config); err != nil {
		t.Fatal(err)
	}
	if !config.DuplicateDetection.Enabled || !config.DuplicateDetection.ReversedNames || config.DuplicateDetection.OnePerson {
		t.Fatalf("missing settings must keep their defaults: %+v", config.DuplicateDetection)
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
