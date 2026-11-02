package booru

import (
	"strings"
	"testing"
)

func TestRenderFilenameGrabberTagFormattingAndSeparators(t *testing.T) {
	source := Source{ID: "fixture", URL: "https://board.example.org"}
	post := Post{
		ID: "42",
		Tags: []Tag{
			{Category: "artist", Name: "maho_malice"},
			{Category: "artist", Name: "mimi_doomi"},
			{Category: "artist", Name: "myahogao"},
			{Category: "copyright", Name: "sono_bisque_doll_wa_koi_wo_suru"},
			{Category: "character", Name: "kitagawa_marin"},
		},
	}
	md5 := "2c20e78d4c3163a761873936d4380ead"
	want := "[maho malice+mimi doomi+myahogao]/(sono bisque doll wa koi wo suru).kitagawa marin/_" + md5 + ".jpg"
	for _, template := range []string{
		"[%artist%]\\(%copyright%).%character%\\_%md5%.%ext%",
		"[%artist%]/(%copyright%).%character%/_%md5%.%ext%",
	} {
		if err := ValidateFilenameTemplate(template); err != nil {
			t.Fatalf("valid Grabber template %q rejected: %v", template, err)
		}
		got, err := RenderFilename(template, source, post, md5, "jpg", 0)
		if err != nil || got != want {
			t.Fatalf("RenderFilename(%q) = %q, %v; want %q", template, got, err, want)
		}
	}
}

func TestFilenameTemplateRejectsUnsafeWindowsPaths(t *testing.T) {
	for _, template := range []string{
		"\\absolute\\%md5%.%ext%",
		"..\\%md5%.%ext%",
		"folder\\..\\%md5%.%ext%",
		"C:\\media\\%md5%.%ext%",
		"folder\\\\%md5%.%ext%",
	} {
		if err := ValidateFilenameTemplate(template); err == nil {
			t.Fatalf("unsafe template %q accepted", template)
		}
	}
}

func TestFilenameTagsCannotCreateDirectories(t *testing.T) {
	source := Source{ID: "fixture", URL: "https://board.example.org"}
	post := Post{ID: "1", Tags: []Tag{
		{Category: "artist", Name: "malicious/../artist"},
		{Category: "artist", Name: "plain_name"},
	}}
	got, err := RenderFilename("%artist%/%md5%.%ext%", source, post, strings.Repeat("a", 32), "png", 0)
	want := "malicious_.._artist+plain name/" + strings.Repeat("a", 32) + ".png"
	if err != nil || got != want {
		t.Fatalf("filename = %q, %v; want %q", got, err, want)
	}
}
