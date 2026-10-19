package booru

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRule34TagCategoriesExactNames(t *testing.T) {
	requested := map[string]bool{"artist_a": true, "hero_a": true, "series_a": true, "scene": true}
	data := []byte(`{"tag":[
		{"name":"artist_a","type":"1"},
		{"name":"hero_a","type":"4"},
		{"name":"series_a","type":"3"},
		{"name":"scene","type":"0"},
		{"name":"not_requested","type":"1"},
		{"name":"unknown_kind","type":"999"}
	]}`)
	got := parseRule34TagCategories(data, requested)
	for name, category := range map[string]string{
		"artist_a": "artist", "hero_a": "character", "series_a": "copyright", "scene": "general",
	} {
		if got[name] != category {
			t.Fatalf("category for %q is %q, expected %q", name, got[name], category)
		}
	}
	if len(got) != 4 {
		t.Fatalf("unexpected unrelated or unknown categories: %#v", got)
	}
	for _, invalid := range []string{`{"tag":"bad"}`, `{"success":false}`, `<html>not tags</html>`, `[1,2]`} {
		if len(parseRule34TagCategories([]byte(invalid), requested)) != 0 {
			t.Fatalf("accepted unsupported category payload: %s", invalid)
		}
	}
}

func TestRule34CategorizationUsesOfficialAuthenticatedTagAPI(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://www.rule34.xxx", APIKey: "private-secret", UserID: "42"}
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if r.URL.Host != "api.rule34.xxx" || r.URL.Path != "/index.php" ||
			q.Get("page") != "dapi" || q.Get("s") != "tag" || q.Get("q") != "index" ||
			q.Get("names") != "artist_a hero_a series_a common" ||
			q.Get("api_key") != "private-secret" || q.Get("user_id") != "42" {
			t.Errorf("incorrect official tag DAPI request")
		}
		return response(200, `{"tag":[{"name":"artist_a","type":"1"},
			{"name":"hero_a","type":"4"},{"name":"series_a","type":"3"},{"name":"common","type":"0"}]}`), nil
	})
	posts := []Post{
		{Tags: []Tag{{Name: "artist_a", Category: "unknown"}, {Name: "hero_a", Category: "unknown"},
			{Name: "series_a", Category: "unknown"}, {Name: "common", Category: "unknown"}}},
		{Tags: []Tag{{Name: "hero_a", Category: "unknown"}, {Name: "common", Category: "unknown"}}},
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if calls != 1 {
		t.Fatalf("expected one batched API call, got %d", calls)
	}
	for _, p := range posts {
		for _, tag := range p.Tags {
			if tag.Category == "unknown" {
				t.Fatalf("uncategorized tag: %s", tag.Name)
			}
		}
	}
	again := []Post{{Tags: []Tag{{Name: "hero_a", Category: "unknown"}}}}
	client.CategorizeRule34Tags(context.Background(), source, again)
	if calls != 1 || again[0].Tags[0].Category != "character" {
		t.Fatalf("cached tag category missing or unnecessarily fetched: %v (%d calls)", again, calls)
	}
}

func TestRule34CategoryLookupFailsOpenAndDoesNotGuess(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "secret", UserID: "42"}
	for _, body := range []string{
		`{"tag":[{"name":"unexpected_tag","type":"1"}]}`,
		`{"success":false,"reason":"blocked"}`,
	} {
		client := testClient(func(*http.Request) (*http.Response, error) {
			return response(200, body), nil
		})
		p := []Post{{Tags: []Tag{{Name: "expected_tag", Category: "unknown"}}}}
		client.CategorizeRule34Tags(context.Background(), source, p)
		if p[0].Tags[0].Category != "unknown" {
			t.Fatalf("unrelated provider tag changed post classification: %s", p[0].Tags[0].Category)
		}
	}
	client := testClient(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, "not allowed"), nil
	})
	p := []Post{{Tags: []Tag{{Name: "creator", Category: "artist"},
		{Name: "unknown", Category: "unknown"}}}}
	client.CategorizeRule34Tags(context.Background(), source, p)
	if p[0].Tags[0].Category != "artist" || p[0].Tags[1].Category != "unknown" {
		t.Fatal("unavailable API should not alter known categories")
	}
}

func TestRule34CategoryLookupLimitsBatches(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://api.rule34.xxx", APIKey: "secret", UserID: "42"}
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		names := strings.Fields(r.URL.Query().Get("names"))
		if len(names) > rule34CategoryBatchSize || len(names) == 0 {
			t.Errorf("unbounded category batch: %d", len(names))
		}
		return response(200, `{"tag":[]}`), nil
	})
	posts := []Post{{Tags: make([]Tag, 800)}}
	for i := range posts[0].Tags {
		posts[0].Tags[i] = Tag{Name: fmt.Sprintf("tag_%04d", i), Category: "unknown"}
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if calls != rule34CategoryMaxBatches {
		t.Fatalf("category fetch made %d requests, expected bounded %d", calls, rule34CategoryMaxBatches)
	}
}
