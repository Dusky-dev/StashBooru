package mediaconvert

import (
	"context"
	"testing"
)

func TestToggleRestoreKeepsBothVersions(t *testing.T) {
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
	if !restored.Cached || !s.ConvertedCached(restored) {
		t.Fatal("restore did not retain both versions")
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
	if !reapplied.Cached || !s.ConvertedCached(reapplied) {
		t.Fatal("unrestore did not retain both versions")
	}
}

func TestTrimVersionsPrefersInactiveVersion(t *testing.T) {
	s, _, client, _ := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	r, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Converted is active, so if only one cached version fits, retain original.
	if err := s.Configure(Config{CacheLimitBytes: r.Before.File().Base().Size, FormatDefaults: DefaultFormatDefaults()}); err != nil {
		t.Fatal(err)
	}
	if err := s.TrimVersions(); err != nil {
		t.Fatal(err)
	}
	r, err = s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Cached || s.ConvertedCached(r) {
		t.Fatal("cache trimming did not prioritize the restorable original")
	}
}

func TestAccountConvertedCache(t *testing.T) {
	s, _, client, _ := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	r, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	stats := s.AccountConvertedCache(Summarize([]*Record{r}), []*Record{r})
	want := r.Before.File().Base().Size + r.After.File().Base().Size
	if stats.CacheBytes != want {
		t.Fatalf("cache bytes %d, want %d", stats.CacheBytes, want)
	}
}
