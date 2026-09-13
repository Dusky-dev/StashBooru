package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestFilterCamieFilenameAuthoritativeSelectionsDropsLowerPriorityIdentityPredictions(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "filename"},
		{Name: "lana_(fire_emblem)", Category: "character", Score: 0.95, Source: "model"},
		{Name: "other_character", Category: "character", Score: 0.9, Source: "eva02"},
		{Name: "local_artist", Category: "artist", Score: 1, Source: "filename"},
		{Name: "model_artist", Category: "artist", Score: 0.92, Source: "model"},
		{Name: "pokemon", Category: "copyright", Score: 1, Source: "filename"},
		{Name: "fire_emblem", Category: "copyright", Score: 0.91, Source: "booru:danbooru"},
		{Name: "long_hair", Category: "general", Score: 0.99, Source: "eva02"},
	}

	filtered := filterCamieFilenameAuthoritativeSelections(predictions)
	found := make(map[string]bool, len(filtered))
	for _, prediction := range filtered {
		found[prediction.Category+"\x00"+prediction.Name] = true
	}

	for _, key := range []string{
		"character\x00Lana (Pokemon)",
		"artist\x00local artist",
		"copyright\x00Pokemon",
		"general\x00long hair",
	} {
		if !found[key] {
			t.Fatalf("expected %q to survive Local-priority filtering, got %#v", key, filtered)
		}
	}
	for _, key := range []string{
		"character\x00Lana (Fire Emblem)",
		"character\x00Other Character",
		"artist\x00model artist",
		"copyright\x00Fire Emblem",
	} {
		if found[key] {
			t.Fatalf("suppressed lower-priority identity prediction %q must not reach apply", key)
		}
	}
}

func TestFilterCamieFilenameAuthoritativeSelectionsFillsMissingLocalCategories(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "filename"},
		{Name: "model_artist", Category: "artist", Score: 0.92, Source: "model"},
		{Name: "pokemon", Category: "copyright", Score: 0.91, Source: "model"},
	}

	filtered := filterCamieFilenameAuthoritativeSelections(predictions)
	if len(filtered) != 3 {
		t.Fatalf("Local Character must not suppress missing Artist/Copyright categories: %#v", filtered)
	}
}

func TestFilterCamieFilenameAuthoritativeSelectionsKeepsCombinedFilenameSource(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "model+filename"},
		{Name: "lana_(fire_emblem)", Category: "character", Score: 0.95, Source: "model"},
	}

	filtered := filterCamieFilenameAuthoritativeSelections(predictions)
	if len(filtered) != 1 {
		t.Fatalf("expected only the filename-confirmed Character, got %#v", filtered)
	}
	if filtered[0].Name != "Lana (Pokemon)" || filtered[0].Source != "model+filename" {
		t.Fatalf("unexpected surviving Character: %#v", filtered[0])
	}
}

func TestFilterCamieFilenameAuthoritativeSelectionsPrefersFilenameBeforeDeduplication(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "model"},
		{Name: "lana_(pokemon)", Category: "character", Score: 1, Source: "filename"},
		{Name: "lana_(fire_emblem)", Category: "character", Score: 0.95, Source: "eva02"},
	}

	prioritized := filterCamieFilenameAuthoritativeSelections(predictions)
	selected, err := validateCamiePredictionsV2(prioritized)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected only the Local Character after priority and deduplication, got %#v", selected)
	}
	if selected[0].Source != "filename" || selected[0].Name != "Lana (Pokemon)" {
		t.Fatalf("expected filename Character to win regardless of input order, got %#v", selected[0])
	}
}
