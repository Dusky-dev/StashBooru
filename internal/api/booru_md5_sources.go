package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/booru"
)

// Tagging's MD5 lookup asks the Booru browser sources chosen in tagging
// setup, in that order. Without a saved choice it asks the five sites it
// always asked. A preset without credentials keeps the original lookup (and
// its tag-category lookups); other sources use the Booru browser client with
// their saved credentials.
var defaultBooruLookupSources = []string{"danbooru", "gelbooru", "yandere", "konachan", "safebooru"}

var legacyBooruLookupProviders = map[string]int{
	"danbooru":  0,
	"gelbooru":  1,
	"yandere":   2,
	"konachan":  3,
	"safebooru": 4,
}

const maxBooruLookupSources = 32

var booruLookupSourceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// These engines accept an md5:<hash> search and return the post's MD5.
func booruEngineSupportsMD5(engine string) bool {
	switch engine {
	case "danbooru", "gelbooru", "moebooru", "e621":
		return true
	default:
		return false
	}
}

func validateBooruLookupSources(ids []string) error {
	if len(ids) > maxBooruLookupSources {
		return fmt.Errorf("choose at most %d MD5 lookup sites", maxBooruLookupSources)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !booruLookupSourceID.MatchString(id) {
			return fmt.Errorf("invalid MD5 lookup site %q", id)
		}
		if seen[id] {
			return fmt.Errorf("MD5 lookup site %q is listed twice", id)
		}
		seen[id] = true
	}
	return nil
}

var booruLookupClient = booru.NewClient()

func loadBooruLookupBrowserSources() ([]booru.Source, error) {
	data, err := os.ReadFile(filepath.Join(manager.GetInstance().Config.GetConfigPath(), "booru-browser.json"))
	if errors.Is(err, os.ErrNotExist) {
		return booru.Configure(nil)
	}
	if err != nil {
		return nil, err
	}
	return booru.Configure(data)
}

// booruLookupProviders resolves the chosen source IDs, in order, skipping
// sources that are missing, disabled, cannot search by MD5 or lack required
// credentials.
func booruLookupProviders(chosen []string, sources []booru.Source, search booruSourceSearch) []booruProvider {
	if chosen == nil {
		chosen = defaultBooruLookupSources
	}
	byID := make(map[string]booru.Source, len(sources))
	for _, source := range sources {
		if !source.Disabled {
			byID[source.ID] = source
		}
	}
	providers := make([]booruProvider, 0, len(chosen))
	for _, id := range chosen {
		source, ok := byID[id]
		if !ok || !booruEngineSupportsMD5(source.Engine) {
			continue
		}
		if source.IsRule34() && (source.APIKey == "" || source.UserID == "") {
			continue
		}
		var provider booruProvider
		if index, legacy := legacyBooruLookupProviders[id]; legacy && source.APIKey == "" {
			provider = booruProviders[index]
		} else {
			provider = clientBooruProvider(source, search)
		}
		if parsed, err := url.Parse(source.URL); err == nil && parsed.Hostname() != "" {
			provider.hosts = []string{booruHostKey(parsed.Hostname())}
		}
		providers = append(providers, provider)
	}
	return providers
}

type booruSourceSearch func(ctx context.Context, source booru.Source, hash string) ([]booru.Post, error)

func searchBooruSourceMD5(ctx context.Context, source booru.Source, hash string) ([]booru.Post, error) {
	result, err := booruLookupClient.Search(ctx, source, "md5:"+hash, 1, 1)
	if err != nil {
		return nil, err
	}
	posts := result.Posts
	// The same typing as the Booru browser: live provider lookups, then the
	// installed tag CSV and the Rule34 tag catalog.
	booruLookupRoutes().categorizeBooruPostsWith(ctx, source, posts, true)
	return posts, nil
}

var booruLookupRoutes = func() booruBrowserRoutes {
	return booruBrowserRoutes{configPath: manager.GetInstance().Config.GetConfigPath(), client: booruLookupClient}
}

func clientBooruProvider(source booru.Source, search booruSourceSearch) booruProvider {
	return booruProvider{
		name: source.Name,
		fetch: func(ctx context.Context, hash string) (*booruPost, error) {
			posts, err := search(ctx, source, hash)
			if err != nil {
				return nil, err
			}
			for _, post := range posts {
				// A source that ignores the md5: term would return an unrelated
				// post; only an exact MD5 is a match.
				if post.MD5 != "" && strings.EqualFold(post.MD5, hash) {
					return booruPostFromBrowser(post), nil
				}
			}
			return nil, errBooruNoMatch
		},
	}
}

func booruPostFromBrowser(post booru.Post) *booruPost {
	fields := map[string][]string{}
	uncategorized := map[string]bool{}
	for _, tag := range post.Tags {
		name := strings.ReplaceAll(strings.TrimSpace(tag.Name), " ", "_")
		if name == "" {
			continue
		}
		category := tag.Category
		switch category {
		case "artist", "copyright", "character", "meta":
		case "", "unknown":
			uncategorized[strings.ToLower(name)] = true
			category = "general"
		default:
			category = "general"
		}
		fields[category] = append(fields[category], name)
	}
	return &booruPost{
		ID:                 booruBrowserRemoteID(post),
		MD5:                strings.ToLower(post.MD5),
		PostURL:            post.PostURL,
		TagStringArtist:    strings.Join(fields["artist"], " "),
		TagStringCopyright: strings.Join(fields["copyright"], " "),
		TagStringCharacter: strings.Join(fields["character"], " "),
		TagStringMeta:      strings.Join(fields["meta"], " "),
		TagStringGeneral:   strings.Join(fields["general"], " "),
		Uncategorized:      uncategorized,
	}
}

func booruBrowserRemoteID(post booru.Post) string {
	if post.RemoteID != "" {
		return post.RemoteID
	}
	return post.ID
}

// booruPostURL prefers the URL the source returned with the post.
func booruPostURL(provider booruProvider, post *booruPost) string {
	if post == nil {
		return ""
	}
	if post.PostURL != "" {
		return post.PostURL
	}
	if post.ID != "" && provider.postURL != nil {
		return provider.postURL(post.ID)
	}
	return ""
}
