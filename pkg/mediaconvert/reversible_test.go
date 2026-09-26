package mediaconvert

import (
	"context"
	"testing"
)

func TestToggleRestoreKeepsInactiveVersion(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	originalPath := r.Before.File().Base().Path
	convertedPath := r.After.File().Base().Path

	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != "restored" || repo.f.Base().Path != originalPath {
		t.Fatalf("unexpected restored state: status=%s path=%s", restored.Status, repo.f.Base().Path)
	}
	if restored.Cached || !s.ConvertedCached(restored) {
		t.Fatal("restored state should cache only the converted version")
	}

	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	reapplied, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reapplied.Status != "complete" || repo.f.Base().Path != convertedPath {
		t.Fatalf("unexpected unrestore state: status=%s path=%s", reapplied.Status, repo.f.Base().Path)
	}
	if !reapplied.Cached || s.ConvertedCached(reapplied) {
		t.Fatal("converted state should cache only the original version")
	}
}

func TestTrimVersionsEvictsInactiveVersion(t *testing.T) {
	s, _, client, _ := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	r, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ConvertedCached(r) {
		t.Fatal("converted version was not cached after restore")
	}
	if err := s.Configure(Config{CacheLimitBytes: 0, FormatDefaults: DefaultFormatDefaults()}); err != nil {
		t.Fatal(err)
	}
	if err := s.TrimVersions(); err != nil {
		t.Fatal(err)
	}
	r, err = s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.ConvertedCached(r) {
		t.Fatal("zero cache should evict cached converted version")
	}
	if err := s.ToggleRestore(context.Background(), r.ID); err == nil {
		t.Fatal("unrestore should fail after converted cache eviction")
	}
}
