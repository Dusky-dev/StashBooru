package api

import (
	"reflect"
	"testing"
)

func TestFindDuplicateCanonicalMetadataEntities(t *testing.T) {
	entities := []metadataHealthEntity{
		{ID: 2, Kind: "character", Name: "Lana"},
		{ID: 1, Kind: "Character", Name: "lana"},
		{ID: 3, Kind: "artist", Name: "Lana"},
		{ID: 4, Kind: "copyright", Name: "Re_Zero"},
		{ID: 5, Kind: "copyright", Name: "re zero"},
	}

	got := findDuplicateCanonicalMetadataEntities(entities)
	want := []metadataHealthFinding{
		{Code: metadataHealthDuplicateCanonical, Kind: "character", Value: "lana", EntityIDs: []int{1, 2}},
		{Code: metadataHealthDuplicateCanonical, Kind: "copyright", Value: "re zero", EntityIDs: []int{4, 5}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected findings: got %#v want %#v", got, want)
	}
}

func TestFindDuplicateCanonicalMetadataEntitiesIgnoresInvalidAndCrossKindMatches(t *testing.T) {
	entities := []metadataHealthEntity{
		{ID: 1, Kind: "character", Name: "Same"},
		{ID: 2, Kind: "artist", Name: "Same"},
		{ID: 0, Kind: "character", Name: "Same"},
		{ID: 3, Kind: "character", Name: ""},
	}

	if got := findDuplicateCanonicalMetadataEntities(entities); len(got) != 0 {
		t.Fatalf("expected no findings, got %#v", got)
	}
}
