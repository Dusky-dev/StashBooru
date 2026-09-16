package api

import (
	"reflect"
	"testing"
)

func TestFindAliasCollisionsCanonicalAndAlias(t *testing.T) {
	entities := []aliasCollisionEntity{
		{ID: 1, Kind: "character", Name: "Lana", Aliases: []string{"Lana Pokemon"}},
		{ID: 2, Kind: "character", Name: "Lana Pokemon", Aliases: []string{"Lana P."}},
	}

	got := findAliasCollisions(entities)
	want := []aliasCollision{{
		Kind:  "character",
		Value: "lana pokemon",
		References: []aliasCollisionReference{
			{EntityID: 1, Name: "Lana", MatchKinds: []string{"alias"}},
			{EntityID: 2, Name: "Lana Pokemon", MatchKinds: []string{"canonical"}},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected collisions: got %#v want %#v", got, want)
	}
}

func TestFindAliasCollisionsAliasAliasAndSameEntityDedup(t *testing.T) {
	entities := []aliasCollisionEntity{
		{ID: 1, Kind: "artist", Name: "Artist One", Aliases: []string{"Shared", "shared"}},
		{ID: 2, Kind: "artist", Name: "Artist Two", Aliases: []string{"Shared"}},
		{ID: 3, Kind: "tag", Name: "Shared"},
	}

	got := findAliasCollisions(entities)
	if len(got) != 1 {
		t.Fatalf("expected one same-kind alias collision, got %#v", got)
	}
	if got[0].Kind != "artist" || got[0].Value != "shared" || len(got[0].References) != 2 {
		t.Fatalf("unexpected alias collision: %#v", got[0])
	}
}
