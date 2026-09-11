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
