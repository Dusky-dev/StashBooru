package booru

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRule34SidebarCategoriesReadExactLinksOnly(t *testing.T) {
	requested := map[string]bool{
		"artist_a": true, "character_a": true, "series_a": true,
		"meta_a": true, "general_a": true, "unrelated": true,
	}
	body := `<!doctype html><html><body>
		<a href="/index.php?page=post&amp;s=list&amp;tags=unrelated">outside</a>
		<ul id="tag-sidebar">
			<li class="tag-type-artist"><a href="/index.php?page=wiki&amp;s=list&amp;search=artist_a">?</a>
				<a href="/index.php?page=post&amp;s=list&amp;tags=artist_a">artist</a></li>
			<li class="tag-type-character"><a href="index.php?page=post&amp;s=list&amp;tags=character_a">character</a></li>
			<li class="tag-type-copyright"><a href="/index.php?page=post&amp;s=list&amp;tags=series_a">series</a></li>
			<li class="tag-type-meta"><a href="/index.php?page=post&amp;s=list&amp;tags=meta_a">meta</a></li>
			<li class="tag-type-general"><a href="/index.php?page=post&amp;s=list&amp;tags=general_a">general</a></li>
			<li class="tag-type-artist"><a href="https://elsewhere.invalid/index.php?page=post&amp;s=list&amp;tags=unrelated">untrusted</a></li>
		</ul>
		<script>const fake = '<li class="tag-type-artist"><a href="/index.php?page=post&amp;s=list&amp;tags=unrelated">'</script>
	</body></html>`
	got := parseRule34SidebarCategories([]byte(body), requested)
	for name, category := range map[string]string{
		"artist_a": "artist", "character_a": "character",
		"series_a": "copyright", "meta_a": "meta", "general_a": "general",
	} {
		if got[name] != category {
			t.Errorf("%s categorized as %q instead of %q", name, got[name], category)
		}
	}
	if got["unrelated"] != "" || len(got) != 5 {
		t.Fatalf("untrusted or unrelated tag accepted: %+v", got)
	}
	if got := parseRule34SidebarCategories([]byte("<html>verification required</html>"), requested); len(got) != 0 {
		t.Fatalf("challenge page produced categories: %+v", got)
	}
}

func TestRule34SidebarResolvedTagsAvoidRedundantLiveLookups(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "private-secret", UserID: "42"}
	apiCalls, pageCalls := 0, 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.rule34.xxx":
			apiCalls++
			t.Error("fully classified sidebar tags should not trigger tag API lookups")
			return response(200, "<tags count=\"0\"/>"), nil
		case "rule34.xxx":
			pageCalls++
			if r.URL.Path != "/index.php" || r.URL.Query().Get("page") != "post" ||
				r.URL.Query().Get("s") != "view" || r.URL.Query().Get("api_key") != "" ||
				r.URL.Query().Get("user_id") != "" || r.Header.Get("Authorization") != "" {
				t.Error("sidebar request leaked API credentials or used wrong endpoint")
			}
			switch r.URL.Query().Get("id") {
			case "123":
				return response(200, `<ul id="tag-sidebar">
					<li class="tag-type-artist"><a href="/index.php?page=post&amp;s=list&amp;tags=artist_a">artist_a</a></li>
					<li class="tag-type-character"><a href="/index.php?page=post&amp;s=list&amp;tags=hero_a">hero_a</a></li>
					</ul>`), nil
			case "124":
				return response(200, `<ul id="tag-sidebar">
					<li class="tag-type-copyright"><a href="/index.php?page=post&amp;s=list&amp;tags=series_a">series_a</a></li>
					<li class="tag-type-general"><a href="/index.php?page=post&amp;s=list&amp;tags=scene">scene</a></li>
					</ul>`), nil
			default:
				t.Errorf("unexpected Rule34 post ID %q", r.URL.Query().Get("id"))
			}
		default:
			t.Errorf("unexpected network destination %q", r.URL.Host)
		}
		return response(404, "not found"), nil
	})
	posts := []Post{
		{ID: "123", Tags: []Tag{{Name: "artist_a", Category: "unknown"}, {Name: "hero_a", Category: "unknown"}}},
		{ID: "124", Tags: []Tag{{Name: "series_a", Category: "unknown"}, {Name: "scene", Category: "unknown"}}},
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if apiCalls != 0 || pageCalls != 2 {
		t.Fatalf("expected no redundant tag API lookup and 2 HTML sidebars, got %d API and %d pages", apiCalls, pageCalls)
	}
	for _, post := range posts {
		for _, tag := range post.Tags {
			if tag.Category == "unknown" {
				t.Fatalf("sidebars did not categorize %s", tag.Name)
			}
		}
	}
	again := []Post{{ID: "125", Tags: []Tag{{Name: "hero_a", Category: "unknown"}}}}
	client.CategorizeRule34Tags(context.Background(), source, again)
	if apiCalls != 0 || pageCalls != 2 || again[0].Tags[0].Category != "character" {
		t.Fatal("sidebar categories should reuse the existing cache")
	}
}

func TestRule34SidebarBlockedFailsOpenAndStopsRetrying(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "secret", UserID: "42"}
	apiCalls, pageCalls := 0, 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Host, "api.") {
			apiCalls++
			return response(200, `<tags count="0"/>`), nil
		}
		pageCalls++
		return response(http.StatusForbidden, "Cloudflare challenge"), nil
	})
	posts := []Post{
		{ID: "123", Tags: []Tag{{Name: "a", Category: "unknown"}}},
		{ID: "124", Tags: []Tag{{Name: "b", Category: "unknown"}}},
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if apiCalls != 2 || pageCalls != 1 {
		t.Fatalf("blocked sidebar should fall back to two exact tag lookups: api=%d page=%d", apiCalls, pageCalls)
	}
	for _, post := range posts {
		if post.Tags[0].Category != "unknown" {
			t.Fatal("blocked site must not invent tag types")
		}
	}
}

func TestRule34SidebarRejectsOtherProviderAndUntrustedID(t *testing.T) {
	client := testClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid post must not make a network request")
		return response(500, ""), nil
	})
	for _, tc := range []struct {
		source Source
		id     string
	}{
		{source: testSource("gelbooru"), id: "123"},
		{source: Source{ID: "r", Name: "Rule34", Engine: "gelbooru", URL: "https://rule34.xxx"}, id: "../../bad"},
	} {
		_, err := client.rule34PostSidebar(context.Background(), tc.source, tc.id, map[string]bool{"test": true})
		if err == nil {
			t.Fatal("invalid source or post ID was accepted")
		}
	}
}
