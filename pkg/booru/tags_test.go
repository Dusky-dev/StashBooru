package booru

import (
	"context"
	"strconv"
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
	for _, csv := range []string{"", "name,post_count\nfoo,3", "foo,0,-1,", "foo,9,1,", "foo,0,nope,", "\"unterminated", "foo,0,1,\"alias,\ncontrol\""} {
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

func TestTagDatabaseDeduplicatesProviderSnapshots(t *testing.T) {
	// The published Gelbooru CSV repeats final_fantasy_v at records 7129/7130,
	// and other names recur with different counts and categories.
	input := "final_fantasy_v,3,3614,\nfinal_fantasy_v,3,3614,\n" +
		"hero,0,10,old_alias\nhero,4,50,NEW_ALIAS\nhero,1,50,new_alias\nhero,0,20,older_alias\n"
	index, err := ReadTagCSVForEngine(strings.NewReader(input), "gelbooru")
	if err != nil {
		t.Fatal(err)
	}
	if index.Count() != 2 || len(index.keys) != 5 {
		t.Fatalf("duplicate names/aliases inflated index: tags=%d keys=%d", index.Count(), len(index.keys))
	}
	for _, prefix := range []string{"hero", "old_alias", "new_alias", "older_alias"} {
		got := index.Suggest(context.Background(), prefix)
		if len(got) != 1 || got[0].Name != "hero" || got[0].Category != "character" || got[0].Count != 50 {
			t.Fatalf("lost duplicate metadata or alias %q: %+v", prefix, got)
		}
	}
	for _, invalid := range []string{"hero,9,1,", "hero,0,-1,", "hero,0,1,\"bad\nname\""} {
		if _, err := ReadTagCSV(strings.NewReader(input + invalid)); err == nil {
			t.Fatalf("duplicate row bypassed validation: %q", invalid)
		}
	}
}

func TestTagDatabaseDuplicateAliasUnionRemainsBounded(t *testing.T) {
	var aliases []string
	for i := 0; i < 32; i++ {
		aliases = append(aliases, "alias_"+strconv.Itoa(i))
	}
	input := "hero,4,1,\"" + strings.Join(aliases, ",") + "\"\n"
	if _, err := ReadTagCSV(strings.NewReader(input + input)); err != nil {
		t.Fatal("identical alias list should not consume the limit twice", err)
	}
	if _, err := ReadTagCSV(strings.NewReader(input + "hero,4,2,another_alias\n")); err == nil {
		t.Fatal("duplicate rows bypassed the per-tag alias limit")
	}
}

func TestTagCategoryRequiresExactCanonicalName(t *testing.T) {
	index, err := ReadTagCSV(strings.NewReader("artist_a,1,10,alias_a\nseries_a,3,5,\nhero,4,3,\n"))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"ARTIST_A": "artist", "series_a": "copyright", "hero": "character", "alias_a": "", "artist": ""} {
		if got := index.Category(name); got != want {
			t.Fatal(name, got, want)
		}
	}
}
