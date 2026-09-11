package api

import "testing"

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
		prediction := normalizeCamiePrediction(newCamieTestPrediction(test.name, test.category))
		if prediction.RawName != test.name {
			t.Fatalf("expected raw alias %q, got %q", test.name, prediction.RawName)
		}
		if prediction.Name == prediction.RawName {
			t.Fatalf("expected normalized name to differ from raw underscore name %q", test.name)
		}
	}
}
