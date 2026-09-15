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

func TestFindCopyrightTagCollisions(t *testing.T) {
	tags := []metadataHealthEntity{
		{ID: 10, Kind: "tag", Name: "Re_Zero"},
		{ID: 11, Kind: "tag", Name: "solo"},
	}
	copyrights := []metadataHealthEntity{
		{ID: 20, Kind: "copyright", Name: "re zero"},
		{ID: 21, Kind: "copyright", Name: "Other"},
	}

	got := findCopyrightTagCollisions(tags, copyrights)
	want := []metadataHealthFinding{
		{
			Code:      metadataHealthCopyrightTagCollision,
			Kind:      "copyright",
			Value:     "re zero",
			EntityIDs: []int{10, 20},
			Detail:    "legacy Tag shares a canonical name with a native Copyright",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Copyright/Tag findings: got %#v want %#v", got, want)
	}
}

func TestFindArtistRelationshipDrift(t *testing.T) {
	legacyOnly := 7
	mismatchLegacy := 9
	relations := []metadataHealthArtistRelation{
		{MediaKind: "image", MediaID: 1, LegacyStudio: &legacyOnly},
		{MediaKind: "video", MediaID: 2, NativeArtists: []int{3, 4}},
		{MediaKind: "image", MediaID: 3, LegacyStudio: &mismatchLegacy, NativeArtists: []int{5, 6}},
		{MediaKind: "video", MediaID: 4, LegacyStudio: &legacyOnly, NativeArtists: []int{7, 8}},
	}

	got := findArtistRelationshipDrift(relations)
	want := []metadataHealthFinding{
		{
			Code:      metadataHealthLegacyArtistMismatch,
			Kind:      "image",
			MediaID:   3,
			EntityIDs: []int{9, 5, 6},
			Detail:    "legacy StudioID is not one of the native Artists",
		},
		{
			Code:      metadataHealthLegacyArtistMissing,
			Kind:      "image",
			MediaID:   1,
			EntityIDs: []int{7},
			Detail:    "legacy StudioID is set but native Artist relationships are empty",
		},
		{
			Code:      metadataHealthNativeArtistMissingLegacy,
			Kind:      "video",
			MediaID:   2,
			EntityIDs: []int{3, 4},
			Detail:    "native Artist relationships exist but legacy StudioID is empty",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Artist drift findings: got %#v want %#v", got, want)
	}
}
