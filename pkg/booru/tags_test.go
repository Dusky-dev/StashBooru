package booru

import (
	"context"
	"strings"
	"testing"
)

func TestTagDatabaseFormatsAliasesAndRanking(t *testing.T) {
	for _, csv := range []string{
		"\ufeffblue_hair,0,200,azure_hair\nblue_eyes,0,500,azure_eyes\nartist_a,1,10,\nseries_a,3,30,\nhero,4,20,\n",
		"id,name,category,post_count,aliases\n1,blue_hair,0,200,azure_hair\n2,blue_eyes,0,500,azure_eyes\n3,artist_a,1,10,\n4,series_a,3,30,\n5,hero,4,20,\n",
	} {
		index, err := ReadTagCSV(strings.NewReader(csv))
		if err != nil || index.Count() != 5 {
			t.Fatal(err)
		}
		got := index.Suggest(context.Background(), "AZURE")
		if len(got) != 2 || got[0].Name != "blue_eyes" || got[0].Alias != "azure_eyes" {
			t.Fatal(got)
		}
		if got := index.Suggest(context.Background(), "artist"); len(got) != 1 || got[0].Category != "artist" {
			t.Fatal(got)
		}
	}
}
func TestTagDatabaseRejectsMalformedReplacement(t *testing.T) {
	for _, csv := range []string{"", "name,post_count\nfoo,3", "foo,0,-1,", "foo,9,1,", "foo,0,nope,", "foo,0,1,\nfoo,0,2,", "\"unterminated", "foo,0,1,\"alias,\ncontrol\""} {
		if _, err := ReadTagCSV(strings.NewReader(csv)); err == nil {
			t.Fatalf("accepted %q", csv)
		}
	}
	var input strings.Builder
	for i := 0; i < 25; i++ {
		input.WriteString("blue_")
		input.WriteByte(byte('a' + i))
		input.WriteString(",0,10,blue_alias\n")
	}
	index, err := ReadTagCSV(strings.NewReader(input.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got := index.Suggest(context.Background(), "blue"); len(got) != 20 {
		t.Fatal(len(got))
	}
	if got := index.Suggest(context.Background(), "b"); len(got) != 0 {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := index.Suggest(ctx, "blue"); len(got) != 0 {
		t.Fatal("cancelled search returned tags")
	}
}

func TestE621NumericCategoriesRemainSourceSpecific(t *testing.T) {
	csv := "contributor_a,2,1,\ncanine,5,100,\ntagging,7,5,\nlore_tag,8,10,\n"
	index, err := ReadTagCSVForEngine(strings.NewReader(csv), "e621")
	if err != nil {
		t.Fatal(err)
	}
	for prefix, want := range map[string]string{"co": "contributor", "ca": "species", "ta": "meta", "lo": "lore"} {
		got := index.Suggest(context.Background(), prefix)
		if len(got) != 1 || got[0].Category != want {
			t.Fatal(prefix, got)
		}
	}
	if _, err := ReadTagCSV(strings.NewReader(csv)); err == nil {
		t.Fatal("e621 numbers silently reclassified as Danbooru")
	}
}
