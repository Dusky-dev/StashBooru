package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestBooruLookupProvidersDefaultToTheOriginalSites(t *testing.T) {
	sources := booru.Defaults()
	providers := booruLookupProviders(nil, sources, nil)
	if len(providers) != len(booruProviders) {
		t.Fatalf("got %d default providers, want %d", len(providers), len(booruProviders))
	}
	for index, provider := range providers {
		if provider.name != booruProviders[index].name || provider.fetch != nil {
			t.Fatalf("default provider %d = %q, want the original %q lookup", index, provider.name, booruProviders[index].name)
		}
	}
	if got := booruLookupProviders([]string{}, sources, nil); len(got) != 0 {
		t.Fatalf("an empty choice must turn lookup off, got %d providers", len(got))
	}
}

func TestBooruLookupProvidersUseChosenSourcesWithCredentials(t *testing.T) {
	sources := booru.Defaults()
	for index := range sources {
		if sources[index].ID == "gelbooru" {
			sources[index].APIKey = "key"
			sources[index].UserID = "1"
		}
	}
	chosen := []string{"e621", "gelbooru", "rule34", "pixiv", "danbooru", "missing"}
	providers := booruLookupProviders(chosen, sources, nil)
	var names []string
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	// Rule34 lacks credentials, Pixiv cannot search by MD5 and "missing" is
	// not a source; Danbooru has no key so it keeps the original lookup.
	want := []string{"e621", "Gelbooru", "Danbooru"}
	if len(names) != len(want) {
		t.Fatalf("providers = %v, want %v", names, want)
	}
	for index := range want {
		if names[index] != want[index] {
			t.Fatalf("providers = %v, want %v", names, want)
		}
	}
	if providers[0].fetch == nil || providers[1].fetch == nil || providers[2].fetch != nil {
		t.Fatal("credentialed and non-preset sources must use the Booru browser client")
	}
}

func TestClientBooruProviderRequiresTheExactMD5(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef"
	var posts []booru.Post
	search := func(context.Context, booru.Source, string) ([]booru.Post, error) { return posts, nil }
	provider := clientBooruProvider(booru.Source{Name: "e621", Engine: "e621"}, search)

	for name, candidate := range map[string][]booru.Post{
		"empty MD5": {{ID: "1"}},
		"other MD5": {{ID: "1", MD5: "ffffffffffffffffffffffffffffffff"}},
		"no posts":  nil,
	} {
		posts = candidate
		if _, err := provider.fetch(context.Background(), hash); !errors.Is(err, errBooruNoMatch) {
			t.Fatalf("%s: got %v, want no match", name, err)
		}
	}

	posts = []booru.Post{{
		ID:       "e621:42",
		RemoteID: "42",
		MD5:      "0123456789ABCDEF0123456789ABCDEF",
		PostURL:  "https://e621.net/posts/42",
		Tags: []booru.Tag{
			{Name: "artist name", Category: "artist"},
			{Name: "wolf", Category: "species"},
			{Name: "hero", Category: "character"},
		},
	}}
	post, err := provider.fetch(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "42" || post.TagStringArtist != "artist_name" || post.TagStringCharacter != "hero" || post.TagStringGeneral != "wolf" {
		t.Fatalf("unexpected converted post %+v", post)
	}
	if got := booruPostURL(provider, post); got != "https://e621.net/posts/42" {
		t.Fatalf("post URL = %q", got)
	}
}

func TestLookupBooruPostWithPrefersPriorityAndToleratesFailures(t *testing.T) {
	fixed := func(name string, post *booruPost, err error) booruProvider {
		return booruProvider{name: name, fetch: func(context.Context, string) (*booruPost, error) { return post, err }}
	}
	failing := fixed("down", nil, errors.New("HTTP 503"))
	none := fixed("none", nil, errBooruNoMatch)
	if _, _, err := lookupBooruPostWith(context.Background(), []booruProvider{failing, none}, "h"); !errors.Is(err, errBooruNoMatch) {
		t.Fatalf("one failing site must not hide a no-match, got %v", err)
	}
	if _, _, err := lookupBooruPostWith(context.Background(), []booruProvider{failing}, "h"); err == nil || errors.Is(err, errBooruNoMatch) {
		t.Fatalf("all sites failing must be an error, got %v", err)
	}
	first := fixed("first", &booruPost{ID: "1"}, nil)
	second := fixed("second", &booruPost{ID: "2"}, nil)
	provider, post, err := lookupBooruPostWith(context.Background(), []booruProvider{failing, first, second}, "h")
	if err != nil || provider.name != "first" || post.ID != "1" {
		t.Fatalf("got %q %+v %v, want the first chosen match", provider.name, post, err)
	}
}

func TestBooruLookupSourcesConfigKeepsLegacyDefault(t *testing.T) {
	var legacy camieConfig
	if err := json.Unmarshal([]byte(`{"threshold":0.5,"eva02Threshold":0.35,"limit":50}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.BooruLookupSources != nil {
		t.Fatal("a config without the field must use the default sites")
	}
	config := defaultCamieConfig()
	config.BooruLookupSources = []string{}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var restored camieConfig
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.BooruLookupSources == nil || len(restored.BooruLookupSources) != 0 {
		t.Fatal("an empty choice must survive saving")
	}
	config.BooruLookupSources = []string{"danbooru", "danbooru"}
	if validateCamieConfig(config) == nil {
		t.Fatal("duplicate sites must be rejected")
	}
}

func TestTaggingSourceURLsAreValidatedAndAddOnly(t *testing.T) {
	if _, err := validateTaggingSourceURLs([]string{"javascript:alert(1)"}); err == nil {
		t.Fatal("non-http links must be rejected")
	}
	urls, err := validateTaggingSourceURLs([]string{" https://danbooru.donmai.us/posts/1 "})
	if err != nil || urls[0] != "https://danbooru.donmai.us/posts/1" {
		t.Fatalf("got %v %v", urls, err)
	}
	missing := missingTaggingURLs(
		[]string{"https://danbooru.donmai.us/posts/1/"},
		[]string{"https://danbooru.donmai.us/posts/1", "https://e621.net/posts/2", "https://e621.net/posts/2"},
	)
	if len(missing) != 1 || missing[0] != "https://e621.net/posts/2" {
		t.Fatalf("missing = %v", missing)
	}
}

// An MD5 match types its tags like the Booru browser: the installed tag CSV
// fills what the site left untyped, so they are not all General.
func TestBooruMD5LookupUsesInstalledTagCSV(t *testing.T) {
	dir := t.TempDir()
	source := booru.Source{ID: "gb", Name: "Board", Engine: "gelbooru", URL: "https://board.example.org"}
	if err := os.MkdirAll(filepath.Join(dir, "booru-tags"), 0700); err != nil {
		t.Fatal(err)
	}
	csv := "type,count,name,ambiguous,id\n1,5,artist_a,False,1\n4,3,char_b,False,2\n3,3,series_c,False,3\n"
	if err := os.WriteFile(filepath.Join(dir, "booru-tags", source.Instance()+".csv"), []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	posts := []booru.Post{{ID: "1", MD5: "abc", Tags: []booru.Tag{
		{Name: "artist_a", Category: "unknown"}, {Name: "char_b", Category: "unknown"},
		{Name: "series_c", Category: "unknown"}, {Name: "solo", Category: "unknown"},
	}}}
	booruBrowserRoutes{configPath: dir}.categorizeBooruPostsWith(context.Background(), source, posts, true)
	post := booruPostFromBrowser(posts[0])
	if post.TagStringArtist != "artist_a" || post.TagStringCharacter != "char_b" ||
		post.TagStringCopyright != "series_c" || post.TagStringGeneral != "solo" ||
		!post.Uncategorized["solo"] || post.Uncategorized["artist_a"] {
		t.Fatalf("tags not typed from the CSV: %#v", post)
	}
}
