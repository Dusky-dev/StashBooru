package booru

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAutocompleteEngineRequestsAndCategories(t *testing.T) {
	for _, tt := range []struct{ engine, host, path, key, body, category string }{
		{"danbooru", "board.example.org", "/autocomplete.json", "search[query]", `[{"value":"blue_hair","category":0,"post_count":123}]`, "general"},
		{"gelbooru", "board.example.org", "/index.php", "term", `[{"value":"blue_hair","category":"4","post_count":"123"}]`, "character"},
		{"gelbooru", "rule34.xxx", "/autocomplete.php", "q", `[{"value":"blue_hair","label":"blue_hair (123)"}]`, "unknown"},
		{"moebooru", "board.example.org", "/tag.json", "name", `[{"name":"blue_hair","type":5,"count":123}]`, "circle"},
		{"e621", "board.example.org", "/tags.json", "search[name_matches]", `[{"name":"blue_hair","category":5,"post_count":123}]`, "species"},
		{"philomena", "board.example.org", "/autocomplete/tags", "term", `[{"value":"blue hair","label":"blue hair (123)"}]`, "unknown"},
	} {
		t.Run(tt.engine+tt.host, func(t *testing.T) {
			s := testSource(tt.engine)
			s.URL = "https://" + tt.host
			if tt.host == "rule34.xxx" {
				// Rule34 requires account credentials for API access.
				s.APIKey, s.UserID = "test-secret", "42"
			}
			client := testClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != tt.path || !strings.HasPrefix(r.URL.Query().Get(tt.key), "blue") {
					t.Fatalf("unexpected endpoint: %s", r.URL.Redacted())
				}
				if tt.host == "rule34.xxx" {
					if r.URL.Hostname() != "api.rule34.xxx" {
						t.Fatal("Rule34 autocomplete must use its API host")
					}
					if r.URL.Query().Get("api_key") != s.APIKey || r.URL.Query().Get("user_id") != s.UserID {
						t.Fatal("Rule34 autocomplete must include server-side credentials")
					}
				}
				return response(200, tt.body), nil
			})
			items, err := client.Autocomplete(context.Background(), s, "blue")
			if err != nil || len(items) != 1 || items[0].Count != 123 || items[0].Category != tt.category {
				t.Fatalf("%+v %v", items, err)
			}
		})
	}
}

func TestAutocompleteCacheCredentialsBoundsAndCancellation(t *testing.T) {
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		return response(200, `[{"name":"blue_hair","category":0,"post_count":3}]`), nil
	})
	s := testSource("e621")
	for i := 0; i < 2; i++ {
		items, err := client.Autocomplete(context.Background(), s, "blue")
		if err != nil {
			t.Fatal(err)
		}
		items[0].Name = "changed"
	}
	if calls != 1 {
		t.Fatal("did not cache successful response", calls)
	}
	s.Login, s.APIKey = "owner", "private-key"
	items, err := client.Autocomplete(context.Background(), s, "blue")
	if err != nil || calls != 2 || items[0].Name != "blue_hair" {
		t.Fatal("cache leaked across credentials or caller mutated cached data", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Autocomplete(ctx, s, "blue"); err == nil {
		t.Fatal("cancelled cached lookup succeeded")
	}
	for _, prefix := range []string{"x", "  ", "x\nsecret", "foo*", strings.Repeat("x", 81)} {
		if _, err := client.Autocomplete(context.Background(), s, prefix); err == nil {
			t.Fatal("accepted invalid prefix")
		}
	}
	for _, body := range []string{"null", "{}", "<html>secret</html>", strings.Repeat("x", 513<<10)} {
		client := testClient(func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, err := client.Autocomplete(context.Background(), s, "blue"); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid response accepted or reflected", err)
		}
	}
}

func TestAutocompleteAndSearchShareThrottleAndDeadline(t *testing.T) {
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		res := response(429, `secret`)
		res.Header.Set("Retry-After", "60")
		return res, nil
	})
	s := testSource("gelbooru")
	if _, err := client.Autocomplete(context.Background(), s, "blue"); err == nil {
		t.Fatal("accepted throttle")
	}
	if _, err := client.Search(context.Background(), s, "blue", 1, 24); err == nil || calls != 1 {
		t.Fatal("search bypassed shared cooldown")
	}
	client = testClient(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.Autocomplete(ctx, s, "blue"); err == nil {
		t.Fatal("deadline ignored")
	}
}

func TestSearchFiltersUseProviderSyntax(t *testing.T) {
	for _, tt := range []struct{ engine, want string }{
		{"danbooru", "cat order:score rating:g"}, {"gelbooru", "cat sort:score:desc rating:general"},
		{"e621", "cat order:score rating:s"}, {"moebooru", "cat order:score rating:s"}, {"philomena", "(cat) AND (safe)"},
	} {
		s := testSource(tt.engine)
		got, err := s.filterQuery("cat", SearchOptions{Sort: "popular", Rating: "safe"})
		if err != nil || got != tt.want {
			t.Fatal(tt.engine, got, err)
		}
	}
	if _, err := testSource("gelbooru").filterQuery("cat", SearchOptions{Sort: "updated"}); err == nil {
		t.Fatal("pretended unsupported sorting worked")
	}
	for _, tt := range []struct{ engine, sort, want string }{
		{"danbooru", "updated", "cat order:change"},
		{"e621", "updated", "cat order:change"},
		{"moebooru", "updated", "cat order:change_desc"},
		{"moebooru", "newest", "cat order:id_desc"},
	} {
		got, err := testSource(tt.engine).filterQuery("cat", SearchOptions{Sort: tt.sort})
		if err != nil || got != tt.want {
			t.Fatal(tt.engine, got, err)
		}
	}
}
