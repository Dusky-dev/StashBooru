package api

import (
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestParseCamieFilenameDefaultLayout(t *testing.T) {
	predictions, err := parseCamieFilename(
		"[afrobull](kono_subarashii_sekai_ni_shukufuku_wo!).darkness_(konosuba)_8e12eeba08d10de8d5e05253388f9cca.jxl",
		defaultCamieFilenameLayout,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(predictions) != 3 {
		t.Fatalf("expected 3 filename predictions, got %d", len(predictions))
	}

	merged := mergeCamiePredictions(nil, predictions)
	want := map[string]string{
		"artist":    "afrobull",
		"copyright": "Kono Subarashii Sekai Ni Shukufuku Wo!",
		"character": "Darkness (Konosuba)",
	}
	for _, prediction := range merged {
		if prediction.Name != want[prediction.Category] {
			t.Fatalf("unexpected %s name %q, wanted %q", prediction.Category, prediction.Name, want[prediction.Category])
		}
		if prediction.Source != "filename" {
			t.Fatalf("expected filename source for %q, got %q", prediction.Name, prediction.Source)
		}
	}
}

func TestParseCamieFilenameSplitsCopyrightList(t *testing.T) {
	const composite = "Evangelion 3.0 You Can (Not) Redo+neon Genesis Evangelion+rebuild Of Evangelion"
	predictions, err := parseCamieFilename(
		"[khara]("+composite+").ikari_shinji_8e12eeba08d10de8d5e05253388f9cca.jpg",
		defaultCamieFilenameLayout,
	)
	if err != nil {
		t.Fatal(err)
	}

	model := []camietagger.Tag{
		{Name: "neon_genesis_evangelion", Category: "copyright", Score: 0.991},
		{Name: "rebuild_of_evangelion", Category: "copyright", Score: 0.607},
	}
	merged := mergeCamiePredictions(model, predictions)

	want := map[string]bool{
		"Evangelion 3.0 You Can (Not) Redo": false,
		"Neon Genesis Evangelion":          false,
		"Rebuild Of Evangelion":            false,
	}
	copyrightCount := 0
	for _, prediction := range merged {
		if prediction.Category != "copyright" {
			continue
		}
		copyrightCount++
		if strings.Contains(prediction.Name, "+") {
			t.Fatalf("filename copyright was not split: %q", prediction.Name)
		}
		if strings.Contains(prediction.RawName, "+") {
			t.Fatalf("split copyright kept composite alias: %q", prediction.RawName)
		}
		if _, ok := want[prediction.Name]; !ok {
			t.Fatalf("unexpected copyright %q", prediction.Name)
		}
		want[prediction.Name] = true
	}
	if copyrightCount != len(want) {
		t.Fatalf("expected %d merged copyrights, got %d", len(want), copyrightCount)
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing split copyright %q", name)
		}
	}
}

func TestCamieCharacterIdentityUsesTrailingCopyright(t *testing.T) {
	prediction := camietagger.Tag{
		Name:     "darkness_(konosuba)",
		Category: "character",
		Score:    0.99,
	}

	name, disambiguation := camieCharacterIdentity(prediction)
	if name != "Darkness" {
		t.Fatalf("expected character name Darkness, got %q", name)
	}
	if disambiguation != "Konosuba" {
		t.Fatalf("expected disambiguation Konosuba, got %q", disambiguation)
	}

	aliases := camieCharacterAliases(prediction, name)
	wantAliases := map[string]bool{
		"Darkness (Konosuba)": false,
		"darkness_(konosuba)": false,
	}
	for _, alias := range aliases {
		if _, ok := wantAliases[alias]; ok {
			wantAliases[alias] = true
		}
	}
	for alias, found := range wantAliases {
		if !found {
			t.Fatalf("expected character alias %q", alias)
		}
	}
}

func TestCamieCharacterIdentityLeavesPlainNamesAlone(t *testing.T) {
	prediction := camietagger.Tag{
		Name:     "frieren",
		Category: "character",
		Score:    0.99,
	}

	name, disambiguation := camieCharacterIdentity(prediction)
	if name != "Frieren" {
		t.Fatalf("expected character name Frieren, got %q", name)
	}
	if disambiguation != "" {
		t.Fatalf("expected empty disambiguation, got %q", disambiguation)
	}
}

func TestCompileCamieFilenameLayoutRejectsUnknownToken(t *testing.T) {
	if _, err := compileCamieFilenameLayout("%artist%_%unknown%.%ext%"); err == nil {
		t.Fatal("expected unknown filename token to fail validation")
	}
}

func TestMergeCamiePredictionsPreservesRawAlias(t *testing.T) {
	model := []struct {
		name     string
		category string
	}{
		{name: "darkness_(konosuba)", category: "character"},
		{name: "kono_subarashii_sekai", category: "copyright"},
		{name: "long_hair", category: "general"},
	}

	for _, test := range model {
		prediction := normalizeCamiePrediction(camietagger.Tag{
			Name:     test.name,
			Category: test.category,
			Score:    0.9,
		})
		if prediction.RawName != test.name {
			t.Fatalf("expected raw alias %q, got %q", test.name, prediction.RawName)
		}
		if prediction.Name == prediction.RawName {
			t.Fatalf("expected normalized name to differ from raw underscore name %q", test.name)
		}
	}
}

func TestMergeCamiePredictionsFilenameOwnsIdentityCategories(t *testing.T) {
	model := []camietagger.Tag{
		{Name: "darkness_(konosuba)", Category: "character", Score: 0.98},
		{Name: "megumin_(konosuba)", Category: "character", Score: 0.97},
		{Name: "other_artist", Category: "artist", Score: 0.96},
		{Name: "rebuild_of_evangelion", Category: "copyright", Score: 0.95},
		{Name: "long_hair", Category: "general", Score: 0.94},
	}
	filename := []camietagger.Tag{
		{Name: "darkness_(konosuba)", Category: "character", Score: 1, Source: "filename"},
		{Name: "afrobull", Category: "artist", Score: 1, Source: "filename"},
		{Name: "kono_subarashii_sekai_ni_shukufuku_wo!", Category: "copyright", Score: 1, Source: "filename"},
	}

	merged := mergeCamiePredictions(model, filename)
	found := map[string]camietagger.Tag{}
	for _, prediction := range merged {
		found[prediction.Category+"\x00"+prediction.Name] = prediction
	}

	darkness, ok := found["character\x00Darkness (Konosuba)"]
	if !ok {
		t.Fatal("expected authoritative local character Darkness")
	}
	if darkness.Source != "model+filename" {
		t.Fatalf("expected matching Camie character to confirm local value, got source %q", darkness.Source)
	}
	if _, ok := found["character\x00Megumin (Konosuba)"]; ok {
		t.Fatal("conflicting Camie-only character must not mix with local characters")
	}
	if _, ok := found["artist\x00other artist"]; ok {
		t.Fatal("conflicting Camie-only artist must not mix with local artist")
	}
	if _, ok := found["copyright\x00Rebuild Of Evangelion"]; ok {
		t.Fatal("conflicting Camie-only copyright must not mix with local copyrights")
	}
	if _, ok := found["artist\x00afrobull"]; !ok {
		t.Fatal("expected local artist to be preserved")
	}
	if _, ok := found["copyright\x00Kono Subarashii Sekai Ni Shukufuku Wo!"]; !ok {
		t.Fatal("expected local copyright to be preserved")
	}
	if _, ok := found["general\x00long hair"]; !ok {
		t.Fatal("general Camie tags should still mix with local identity metadata")
	}
}

func TestMergeCamiePredictionsFillsMissingLocalCategory(t *testing.T) {
	model := []camietagger.Tag{
		{Name: "rebuild_of_evangelion", Category: "copyright", Score: 0.95},
	}
	filename := []camietagger.Tag{
		{Name: "darkness_(konosuba)", Category: "character", Score: 1, Source: "filename"},
	}

	merged := mergeCamiePredictions(model, filename)
	for _, prediction := range merged {
		if prediction.Category == "copyright" && prediction.Name == "Rebuild Of Evangelion" {
			return
		}
	}
	t.Fatal("Camie should fill a Character/Artist/Copyright category missing from local metadata")
}
