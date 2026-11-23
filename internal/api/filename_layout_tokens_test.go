package api

import "testing"

func TestFilenameLayoutTokens(t *testing.T) {
	layout := "[%artist%](%copyright%).%character%_%md5%.%ext%"
	tokens, err := filenameLayoutTokens("/media/[afrobull](hyrule_warriors+the_legend_of_zelda).link+linkle_0123456789abcdef0123456789abcdef.png", layout)
	if err != nil {
		t.Fatal(err)
	}
	want := []filenameLayoutToken{
		{"artist", "afrobull"},
		{"copyright", "hyrule_warriors+the_legend_of_zelda"},
		{"character", "link+linkle"},
		{"md5", "0123456789abcdef0123456789abcdef"},
		{"ext", "png"},
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens: %+v", tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Fatalf("token %d: %+v, want %+v", i, tokens[i], want[i])
		}
	}
	if tokens, err := filenameLayoutTokens("/media/holiday.png", layout); err != nil || len(tokens) != 0 {
		t.Fatalf("a filename outside the layout has no tokens: %+v %v", tokens, err)
	}
}
