package api

import (
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
