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
	types := map[string]string{"artist_a": "1", "hero_a": "4", "series_a": "3", "common": "0"}
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		name := q.Get("name")
		if r.URL.Host != "api.rule34.xxx" || r.URL.Path != "/index.php" ||
			q.Get("page") != "dapi" || q.Get("s") != "tag" || q.Get("q") != "index" ||
			name == "" || q.Has("names") || q.Has("json") ||
			r.Header.Get("Accept") != "application/xml" ||
			q.Get("api_key") != "private-secret" || q.Get("user_id") != "42" {
			t.Errorf("incorrect official exact-name tag DAPI request: %s", r.URL.Path)
		}
		return response(200, fmt.Sprintf("<tag name=%q type=%q/>", name, types[name])), nil
	})
	posts := []Post{
		{Tags: []Tag{{Name: "artist_a", Category: "unknown"}, {Name: "hero_a", Category: "unknown"},
			{Name: "series_a", Category: "unknown"}, {Name: "common", Category: "unknown"}}},
		{Tags: []Tag{{Name: "hero_a", Category: "unknown"}, {Name: "common", Category: "unknown"}}},
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if calls != 4 {
		t.Fatalf("expected four exact tag lookups, got %d", calls)
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
	if calls != 4 || again[0].Tags[0].Category != "character" {
		t.Fatalf("cached tag category missing or unnecessarily fetched: %v (%d calls)", again, calls)
	}
}

func TestRule34LiveLookupAfterSlowPublicSidebar(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "secret", UserID: "42"}
	publicCalls, apiCalls := 0, 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "rule34.xxx" {
			publicCalls++
			if r.URL.Query().Has("api_key") || r.URL.Query().Has("user_id") {
				t.Error("website page leaked API credentials")
			}
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		apiCalls++
		if r.URL.Host != "api.rule34.xxx" || r.URL.Query().Get("name") != "artist_a" {
			t.Error("exact lookup did not reach the authenticated API")
		}
		return response(200, "<tags><tag name=\"artist_a\" type=\"1\"/></tags>"), nil
	})
	posts := []Post{{ID: "123", Tags: []Tag{{Name: "artist_a", Category: "unknown"}}}}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if publicCalls != 1 || apiCalls != 1 || posts[0].Tags[0].Category != "artist" {
		t.Fatalf("slow sidebar prevented live classification: public=%d api=%d category=%s",
			publicCalls, apiCalls, posts[0].Tags[0].Category)
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

func TestRule34CategoryLookupBoundsExactRequests(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://api.rule34.xxx", APIKey: "secret", UserID: "42"}
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		name := r.URL.Query().Get("name")
		if len(name) == 0 || len(name) > 256 || r.URL.Query().Has("names") {
			t.Errorf("invalid exact-name lookup %q", name)
		}
		return response(200, fmt.Sprintf("<tags count=\"1\"><tag name=%q type=\"0\"/></tags>", name)), nil
	})
	posts := []Post{{Tags: make([]Tag, 800)}}
	for i := range posts[0].Tags {
		posts[0].Tags[i] = Tag{Name: fmt.Sprintf("tag_%04d", i), Category: "unknown"}
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if calls != rule34CategoryMaxExactLookups {
		t.Fatalf("category fetch made %d requests, expected bounded %d", calls, rule34CategoryMaxExactLookups)
	}
}

func TestRule34LiveLookupProgressesPastAbsentTags(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "secret", UserID: "42"}
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		name := r.URL.Query().Get("name")
		if name == "tag_06" {
			return response(200, "<tag name=\"tag_06\" type=\"1\"/>"), nil
		}
		return response(200, "<tags count=\"0\"/>"), nil
	})
	post := Post{Tags: make([]Tag, 7)}
	for i := range post.Tags {
		post.Tags[i] = Tag{Name: fmt.Sprintf("tag_%02d", i), Category: "unknown"}
	}
	first := []Post{post}
	client.CategorizeRule34Tags(context.Background(), source, first)
	if calls != rule34CategoryMaxExactLookups {
		t.Fatalf("unexpected first live request count: %d", calls)
	}
	// Missing records are cached briefly, so repeating the search should
	// classify additional tags instead of requesting the same absent six.
	second := []Post{post}
	client.CategorizeRule34Tags(context.Background(), source, second)
	if calls != rule34CategoryMaxExactLookups+1 || second[0].Tags[6].Category != "artist" {
		t.Fatalf("live classification was starved by absent tags: calls=%d last=%s",
			calls, second[0].Tags[6].Category)
	}
}

func TestRule34CategoryLookupLongNamesStayBounded(t *testing.T) {
	source := Source{ID: "rule34", Name: "Rule34", Engine: "gelbooru",
		URL: "https://rule34.xxx", APIKey: "secret", UserID: "42"}
	calls := 0
	client := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		name := r.URL.Query().Get("name")
		if len(name) > 256 {
			t.Error("unbounded Rule34 exact-name query")
		}
		return response(200, fmt.Sprintf("<tags><tag name=%q type=\"0\"/></tags>", name)), nil
	})
	posts := []Post{{Tags: make([]Tag, 20)}}
	for i := range posts[0].Tags {
		posts[0].Tags[i] = Tag{Name: fmt.Sprintf("%03d_%s", i, strings.Repeat("a", 240)), Category: "unknown"}
	}
	client.CategorizeRule34Tags(context.Background(), source, posts)
	if calls != rule34CategoryMaxExactLookups {
		t.Fatalf("expected %d bounded exact requests, got %d", rule34CategoryMaxExactLookups, calls)
	}
}

func TestRule34TagCategoriesXMLResponse(t *testing.T) {
	requested := map[string]bool{
		"artist_a": true, "hero_a": true, "series_a": true,
		"series_b": true, "common": true, "meta_tag": true,
	}
	body := `<?xml version="1.0" encoding="UTF-8"?>
<tags type="array" count="7">
  <tag id="1" name="artist_a" count="999" type="1"/>
  <tag id="2" name="hero_a" type="4"/>
  <tag id="3" name="series_a" type="3"/>
  <tag id="4" name="series_b" type="2"/>
  <tag id="5" name="common" type="0"/>
  <tag id="6" name="meta_tag" type="5"/>
  <tag id="7" name="unrelated_tag" type="1"/>
</tags>`
	categories := parseRule34TagCategories([]byte(body), requested)
	for name, category := range map[string]string{
		"artist_a": "artist", "hero_a": "character",
		"series_a": "copyright",
		"common":   "general", "meta_tag": "meta",
	} {
		if categories[name] != category {
			t.Errorf("%s: got %q, expected %q", name, categories[name], category)
		}
	}
	if len(categories) != 5 || categories["series_b"] != "" {
		t.Fatalf("XML parser included missing or unrequested categories: %+v", categories)
	}
	for _, invalid := range []string{
		`<html><tags><tag name="artist_a" type="1"/></tags></html>`,
		`<!DOCTYPE tags [<!ENTITY x "fake">]><tags><tag name="artist_a" type="1"/></tags>`,
		`<tags><tag name="artist_a" type="1"></tags>`,
		`<tags><tag name="artist_a" type="999"/></tags>`,
	} {
		if got := parseRule34TagCategories([]byte(invalid), requested); len(got) != 0 {
			t.Errorf("unexpected categories from invalid XML: %+v", got)
		}
	}
}

func TestRule34DeprecatedTagType2RemainsUncategorized(t *testing.T) {
	requested := map[string]bool{"deprecated_tag": true, "series": true}
	for _, body := range []string{
		`<tags><tag name="deprecated_tag" type="2"/><tag name="series" type="3"/></tags>`,
		`{"tag":[{"name":"deprecated_tag","type":2},{"name":"series","type":3}]}`,
	} {
		got := parseRule34TagCategories([]byte(body), requested)
		if got["deprecated_tag"] != "" || got["series"] != "copyright" || len(got) != 1 {
			t.Fatalf("deprecated type must not be cached as copyright: %+v", got)
		}
	}
}

func TestRule34SingularXMLResponsePreservesTagIdentity(t *testing.T) {
	requested := map[string]bool{"hero": true}
	for _, payload := range []string{
		"<tag name=\"hero\" type=\"4\"/>",
		"<?xml version=\"1.0\"?><tag name=\"hero\" type=\"4\"/>",
	} {
		got := parseRule34TagCategories([]byte(payload), requested)
		if got["hero"] != "character" || len(got) != 1 {
			t.Fatalf("singular Rule34 response not classified: %+v", got)
		}
	}
	if got := parseRule34TagCategories([]byte("<tag name=\"other\" type=\"1\"/>"), requested); len(got) != 0 {
		t.Fatalf("unrelated tag category accepted: %+v", got)
	}
}

func TestRule34TagCategoriesXMLPreservesExactTagIdentity(t *testing.T) {
	requested := map[string]bool{"my_tag": true}
	categories := parseRule34TagCategories(
		[]byte(`<tags><tag name="my_tag_extra" type="1"/><tag name="MY_TAG" type="4"/></tags>`),
		requested,
	)
	if categories["my_tag"] != "character" || len(categories) != 1 {
		t.Fatalf("XML tag lookup must match exact names case-insensitively: %+v", categories)
	}
}
