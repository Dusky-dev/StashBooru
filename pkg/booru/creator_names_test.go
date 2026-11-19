package booru

import (
	"context"
	"net/http"
	"testing"
)

func TestLookupCreatorNameUsesProfileAndCaches(t *testing.T) {
	s := testSource("pawchive")
	calls := 0
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/api/v1/patreon/user/42/profile" {
			t.Fatalf("unexpected request %s", r.URL)
		}
		return response(http.StatusOK, `{"id":"42","name":"Meili's Hideout","service":"patreon","public_id":"meili"}`), nil
	})
	if _, ok := c.CachedCreatorName(s, "patreon", "42"); ok {
		t.Fatal("name known before any lookup")
	}
	for range 2 {
		name, err := c.LookupCreatorName(context.Background(), s, "patreon", "42")
		if err != nil || name != "Meili's Hideout" {
			t.Fatalf("lookup = %q, %v", name, err)
		}
	}
	if calls != 1 {
		t.Fatalf("profile fetched %d times, want 1", calls)
	}
	if name, ok := c.CachedCreatorName(s, "patreon", "42"); !ok || name != "Meili's Hideout" {
		t.Fatalf("cached name = %q, %v", name, ok)
	}
}

func TestLookupCreatorNameRemembersMissAndRejectsMismatch(t *testing.T) {
	s := testSource("pawchive")
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/patreon/user/7/profile" {
			return response(http.StatusNotFound, `{"error":"Creator not found."}`), nil
		}
		return response(http.StatusOK, `{"id":"other","name":"Wrong","service":"patreon"}`), nil
	})
	if _, err := c.LookupCreatorName(context.Background(), s, "patreon", "7"); err == nil {
		t.Fatal("missing creator returned no error")
	}
	if name, ok := c.CachedCreatorName(s, "patreon", "7"); !ok || name != "" {
		t.Fatalf("miss not remembered: %q %v", name, ok)
	}
	if name, err := c.LookupCreatorName(context.Background(), s, "patreon", "8"); err == nil || name != "" {
		t.Fatalf("mismatched profile accepted: %q %v", name, err)
	}
	if _, ok := c.CachedCreatorName(s, "patreon", "8"); ok {
		t.Fatal("mismatched profile was cached")
	}
	if _, err := c.LookupCreatorName(context.Background(), testSource("danbooru"), "patreon", "42"); err == nil {
		t.Fatal("non-archive source accepted")
	}
}

func TestCachedCreatorNameReadsLoadedDirectory(t *testing.T) {
	s := testSource("pawchive")
	c := testClient(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `[{"id":"42","name":"Directory Name","service":"patreon"}]`), nil
	})
	if _, err := c.SearchCreators(context.Background(), s, "directory", 1); err != nil {
		t.Fatal(err)
	}
	if name, ok := c.CachedCreatorName(s, "patreon", "42"); !ok || name != "Directory Name" {
		t.Fatalf("directory name = %q, %v", name, ok)
	}
}
