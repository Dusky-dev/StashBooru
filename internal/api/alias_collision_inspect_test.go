package api

import (
	"reflect"
	"testing"
)

func TestInspectAliasValueReturnsCanonicalAndAliasMatches(t *testing.T) {
	entities := []aliasCollisionEntity{
		{ID: 1, Kind: "character", Name: "Alpha", Aliases: []string{"Hero"}},
		{ID: 2, Kind: "character", Name: "Hero", Aliases: []string{"Lead"}},
		{ID: 3, Kind: "artist", Name: "Other", Aliases: []string{"hero"}},
	}

	got := inspectAliasValue(" HERO ", entities)
	if got.Normalized != "hero" || !got.Ambiguous {
		t.Fatalf("unexpected inspection summary: %+v", got)
	}
	if len(got.Matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(got.Matches))
	}
	if got.Matches[0].Kind != "artist" || got.Matches[0].EntityID != 3 || !reflect.DeepEqual(got.Matches[0].MatchKinds, []string{"alias"}) {
		t.Fatalf("unexpected first match: %+v", got.Matches[0])
	}
	if got.Matches[1].EntityID != 1 || !reflect.DeepEqual(got.Matches[1].MatchKinds, []string{"alias"}) {
		t.Fatalf("unexpected second match: %+v", got.Matches[1])
	}
	if got.Matches[2].EntityID != 2 || !reflect.DeepEqual(got.Matches[2].MatchKinds, []string{"canonical"}) {
		t.Fatalf("unexpected third match: %+v", got.Matches[2])
	}
}

func TestInspectAliasValueDeduplicatesSameEntityProvenance(t *testing.T) {
	entities := []aliasCollisionEntity{{ID: 8, Kind: "copyright", Name: "Series", Aliases: []string{"series", "Series"}}}
	got := inspectAliasValue("series", entities)
	if got.Ambiguous || len(got.Matches) != 1 {
		t.Fatalf("unexpected inspection: %+v", got)
	}
	want := []string{"canonical", "alias"}
	if !reflect.DeepEqual(got.Matches[0].MatchKinds, want) {
		t.Fatalf("match kinds = %v, want %v", got.Matches[0].MatchKinds, want)
	}
}
