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
func TestRule34CSVReorderedHeaderAndDeprecatedType(t *testing.T) {
	// Rule34 exports type,count,name,ambiguous,id (not name-first tagcomplete).
	csv := "\ufefftype,count,name,ambiguous,id\n" +
		"1,100,artist_a,False,1001\n" +
		"4,90,hero_a,False,1002\n" +
		"3,80,series_a,False,1003\n" +
		"0,70,ordinary,False,1004\n" +
		"5,60,metadata,False,1005\n" +
		"2,50,old_tag,False,1006\n"
	index, err := ReadTagCSVForEngine(strings.NewReader(csv), "gelbooru")
	if err != nil {
		t.Fatal(err)
	}
	if index.Count() != 6 {
		t.Fatalf("lost Rule34 tags: %d", index.Count())
	}
	for name, want := range map[string]string{
		"artist_a": "artist", "hero_a": "character", "series_a": "copyright",
		"ordinary": "general", "metadata": "meta", "old_tag": "unknown",
	} {
		if got := index.Category(name); got != want {
			t.Errorf("%s: got category %q, expected %q", name, got, want)
		}
	}
	if got := index.Category("not_in_file"); got != "" {
		t.Fatalf("invented absent tag category %q", got)
	}
}

// Published Rule34/Gelbooru exports may have occasional missing or
// unrepresentable names. Skipping those rows preserves the other valid
// categories, including tags appearing after the bad record.
func TestRule34CSVSkipsBoundedMalformedNames(t *testing.T) {
	input := "type,count,name,ambiguous,id\n" +
		"1,20,artist_a,False,1\n" +
		"4,10,,False,2\n" +
		"0,7,control\x00tag,False,3\n" +
		"0,5," + strings.Repeat("a", 257) + ",False,4\n" +
		"3,4,series_a,False,5\n" +
		"0,2,scene_a,False,6\n"
	index, err := ReadTagCSVForEngine(strings.NewReader(input), "gelbooru")
	if err != nil {
		t.Fatal("valid provider rows should survive isolated invalid names", err)
	}
	if index.Count() != 3 || index.SkippedRows() != 3 {
		t.Fatalf("unexpected import statistics: count=%d skipped=%d", index.Count(), index.SkippedRows())
	}
	for name, want := range map[string]string{
		"artist_a": "artist", "series_a": "copyright", "scene_a": "general",
	} {
		if got := index.Category(name); got != want {
			t.Errorf("lost valid tag after bad record %s: %s != %s", name, got, want)
		}
	}
}

// Unrecognized layouts, all-invalid snapshots and non-Gelbooru
// formats retain strict validation rather than silently dropping rows.
func TestTagCSVInvalidNameToleranceIsSourceScoped(t *testing.T) {
	for _, test := range []struct {
		engine, data string
	}{
		{"gelbooru", ",4,12,\nartist_a,1,20,\n"},
		{"danbooru", "name,type,count\nartist_a,1,20\n,4,12\n"},
		{"e621", "name,type,count\nartist_a,1,20\n,4,12\n"},
		{"gelbooru", "name,type,count\n,1,20\n"},
		{"gelbooru", "name,type,count\nartist_a,1,20\nfoo,999,2\n"},
		{"gelbooru", "name,type,count\nartist_a,1,20\n\\\"unterminated"},
	} {
		if _, err := ReadTagCSVForEngine(strings.NewReader(test.data), test.engine); err == nil {
			t.Errorf("%s unexpectedly accepted malformed CSV %q", test.engine, test.data)
		}
	}
	// Headerless positional tagcomplete format is also supported once
	// at least one earlier tag has established valid columns.
	headerless, err := ReadTagCSVForEngine(strings.NewReader(
		"artist_a,1,20,\n,4,12,\ncharacter_a,4,30,\n"), "gelbooru")
	if err != nil || headerless.SkippedRows() != 1 ||
		headerless.Category("character_a") != "character" {
		t.Fatalf("headerless export lost tags after malformed name: index=%v err=%v", headerless, err)
	}

	var input strings.Builder
	input.WriteString("type,count,name,ambiguous,id\n")
	for i := 0; i < maxSkippedGelbooruCSVRows+1; i++ {
		input.WriteString("0,1,,False,1\n")
	}
	input.WriteString("1,10,valid_artist,False,2\n")
	if _, err := ReadTagCSVForEngine(strings.NewReader(input.String()), "gelbooru"); err == nil {
		t.Fatal("accepted unlimited malformed provider names")
	}
}

func TestRule34CSVRecordsAfterRow78268CanBeRead(t *testing.T) {
	// Reproduce the failing row position without claiming access to the
	// owner's actual CSV. The error message does not reveal its contents.
	var input strings.Builder
	input.WriteString("type,count,name,ambiguous,id\n")
	input.WriteString(strings.Repeat("0,1,common_tag,False,1\n", 78266))
	input.WriteString("0,1,,False,78268\n") // CSV record 78268 is malformed
	input.WriteString("4,1,new_character,False,78269\n")
	index, err := ReadTagCSVForEngine(strings.NewReader(input.String()), "gelbooru")
	if err != nil || index.Count() != 2 || index.SkippedRows() != 1 ||
		index.Category("new_character") != "character" {
		t.Fatalf("skipped wrong record or lost later tags: index=%v err=%v", index, err)
	}
}

func TestReorderedCSVHeaderRequiresBothNameAndCategory(t *testing.T) {
	index, err := ReadTagCSVForEngine(strings.NewReader(
		"count,type,name,id\n99,1,known_artist,22\n"), "gelbooru")
	if err != nil || index.Category("known_artist") != "artist" {
		t.Fatalf("reordered header misread: index=%v err=%v", index, err)
	}
	for _, input := range []string{
		"count,category,id\n10,artist,123\n",
		"count,name,id\n10,known,123\n",
	} {
		if _, err := ReadTagCSVForEngine(strings.NewReader(input), "gelbooru"); err == nil {
			t.Fatalf("accepted incomplete header %q", input)
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
