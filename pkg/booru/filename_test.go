package booru

import (
	"strings"
	"testing"
)

func TestRenderFilenameGrabberCategorySeparators(t *testing.T) {
	source := Source{ID: "fixture", URL: "https://board.example.org"}
	post := Post{
		ID: "42",
		Tags: []Tag{
			{Category: "artist", Name: "maho_malice"},
			{Category: "artist", Name: "mimi_doomi"},
			{Category: "artist", Name: "myahogao"},
			{Category: "copyright", Name: "sono_bisque_doll_wa_koi_wo_suru"},
			{Category: "copyright", Name: "second_series"},
			{Category: "character", Name: "kitagawa_marin"},
			{Category: "character", Name: "other_character"},
		},
	}
	md5 := "2c20e78d4c3163a761873936d4380ead"
	got, err := RenderFilename("[%artist%]/(%copyright%).%character%_%md5%.%ext%", source, post, md5, "jpg", 0)
	want := "[maho_malice+mimi_doomi+myahogao]/(sono_bisque_doll_wa_koi_wo_suru+second_series).kitagawa_marin+other_character_" + md5 + ".jpg"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestRenderFilenameCategorySeparatorKeepsOneTagAndLabels(t *testing.T) {
	source := Source{ID: "fixture", URL: "https://board.example.org"}
	post := Post{ID: "1", Tags: []Tag{
		{Category: "artist", Name: "underlying_artist", Label: "named_artist"},
		{Category: "copyright", Name: "one_series"},
	}}
	got, err := RenderFilename("%artist%-%copyright%-%md5%.%ext%", source, post, strings.Repeat("a", 32), "png", 0)
	want := "named_artist-one_series-" + strings.Repeat("a", 32) + ".png"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestFilenameTemplateBackslashesAreStillRejected(t *testing.T) {
	if err := ValidateFilenameTemplate("[%artist%]\\(%copyright%).%character%\\_%md5%.%ext%"); err == nil {
		t.Fatal("backslashes must remain rejected, not silently change path layout")
	}
}
