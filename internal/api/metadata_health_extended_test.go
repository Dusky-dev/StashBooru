package api

import (
	"strings"
	"testing"
)

func TestFindSameNameCharacterContextIncludesCopyrightContext(t *testing.T) {
	findings := findSameNameCharacterContext([]metadataHealthCharacterContext{
		{ID: 9, Name: "Same_Name", CopyrightIDs: []int{7, 3, 7}},
		{ID: 4, Name: "same name", CopyrightIDs: nil},
		{ID: 12, Name: "Other", CopyrightIDs: []int{1}},
	})
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	finding := findings[0]
	if finding.Code != metadataHealthSameNameCharacterContext || finding.Value != "same name" {
		t.Fatalf("unexpected finding: %+v", finding)
	}
	if len(finding.EntityIDs) != 2 || finding.EntityIDs[0] != 4 || finding.EntityIDs[1] != 9 {
		t.Fatalf("unexpected Character IDs: %v", finding.EntityIDs)
	}
	if !strings.Contains(finding.Detail, "Character #4 Copyrights none") || !strings.Contains(finding.Detail, "Character #9 Copyrights [3 7]") {
		t.Fatalf("missing disambiguation context: %q", finding.Detail)
	}
}

func TestFindOrphanMetadataRelations(t *testing.T) {
	legacyArtist := 44
	findings := findOrphanMetadataRelations(
		[]metadataHealthMediaRelations{{
			MediaKind: "image", MediaID: 21,
			CharacterIDs: []int{1, 9, 9}, TagIDs: []int{2, 8}, LegacyArtist: &legacyArtist,
		}},
		map[int]struct{}{1: {}},
		map[int]struct{}{2: {}},
		map[int]struct{}{5: {}},
	)
	if len(findings) != 3 {
		t.Fatalf("got %d findings, want 3: %+v", len(findings), findings)
	}
	byCode := make(map[string]metadataHealthFinding)
	for _, finding := range findings {
		byCode[finding.Code] = finding
	}
	if got := byCode[metadataHealthOrphanCharacterRelation].EntityIDs; len(got) != 1 || got[0] != 9 {
		t.Fatalf("orphan Characters = %v", got)
	}
	if got := byCode[metadataHealthOrphanTagRelation].EntityIDs; len(got) != 1 || got[0] != 8 {
		t.Fatalf("orphan Tags = %v", got)
	}
	if got := byCode[metadataHealthOrphanLegacyArtist].EntityIDs; len(got) != 1 || got[0] != 44 {
		t.Fatalf("orphan legacy Artist = %v", got)
	}
}

func TestFindLegacyTagMigrationAmbiguityUsesCanonicalAndAliases(t *testing.T) {
	tags := []metadataHealthLegacyTag{
		{ID: 30, Name: "series_old", Aliases: []string{"Hero"}},
		{ID: 31, Name: "unique"},
	}
	targets := []metadataHealthMigrationEntity{
		{ID: 3, Kind: "character", Name: "Hero"},
		{ID: 7, Kind: "copyright", Name: "Series", Aliases: []string{"hero"}},
		{ID: 9, Kind: "artist", Name: "Unique"},
	}
	findings := findLegacyTagMigrationAmbiguity(tags, targets)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != metadataHealthTagMigrationAmbiguous || finding.Value != "hero" {
		t.Fatalf("unexpected finding: %+v", finding)
	}
	if len(finding.EntityIDs) != 1 || finding.EntityIDs[0] != 30 {
		t.Fatalf("unexpected legacy Tag IDs: %v", finding.EntityIDs)
	}
	if !strings.Contains(finding.Detail, "character #3") || !strings.Contains(finding.Detail, "copyright #7") {
		t.Fatalf("missing candidate provenance: %q", finding.Detail)
	}
}
