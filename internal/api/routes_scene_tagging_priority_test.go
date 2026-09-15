package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestValidateSceneTaggingPredictionsEnforcesFilenameIdentityPriority(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "filename"},
		{Name: "lana_(fire_emblem)", Category: "character", Score: 0.95, Source: "booru:danbooru"},
		{Name: "local_artist", Category: "artist", Score: 1, Source: "filename"},
		{Name: "booru_artist", Category: "artist", Score: 0.9, Source: "booru:danbooru"},
		{Name: "pokemon", Category: "copyright", Score: 1, Source: "filename"},
		{Name: "fire_emblem", Category: "copyright", Score: 0.9, Source: "booru:danbooru"},
		{Name: "long_hair", Category: "general", Score: 0.99, Source: "booru:danbooru"},
	}

	selected, err := validateSceneTaggingPredictions(predictions)
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]bool, len(selected))
	for _, prediction := range selected {
		found[prediction.Category+"\x00"+prediction.Name] = true
	}

	for _, key := range []string{
		"character\x00Lana (Pokemon)",
		"artist\x00local artist",
		"copyright\x00Pokemon",
		"general\x00long hair",
	} {
		if !found[key] {
			t.Fatalf("expected %q to survive Video Tagging backend priority, got %#v", key, selected)
		}
	}

	for _, key := range []string{
		"character\x00Lana (Fire Emblem)",
		"artist\x00booru artist",
		"copyright\x00Fire Emblem",
	} {
		if found[key] {
			t.Fatalf("conflicting booru identity %q must not reach Video Tagging apply", key)
		}
	}
}

func TestValidateSceneTaggingPredictionsAllowsBooruToFillMissingIdentityCategory(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "filename"},
		{Name: "booru_artist", Category: "artist", Score: 0.9, Source: "booru:danbooru"},
	}

	selected, err := validateSceneTaggingPredictions(predictions)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("booru should fill identity categories missing from Local metadata, got %#v", selected)
	}
}
