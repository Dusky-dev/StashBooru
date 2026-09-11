package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Booru APIs use MD5 as a content identifier, not for security.
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

const (
	booruResponseLimit      = 8 << 20
	booruLookupTimeout      = 12 * time.Second
	booruRequestTimeout     = 8 * time.Second
	booruTagLookupWorkers   = 6
	booruGelbooruTagBatch   = 100
	booruMaxSingleTagLookup = 120
)

var (
	booruMD5Pattern = regexp.MustCompile(`(?i)[0-9a-f]{32}`)
	booruHTTPClient = &http.Client{Timeout: booruRequestTimeout}
	errBooruNoMatch = errors.New("booru post not found")
)

type booruMetadataResponse struct {
	Source    string            `json:"source"`
	PostID    string            `json:"postID"`
	PostURL   string            `json:"postURL,omitempty"`
	MD5       string            `json:"md5"`
	MD5Source string            `json:"md5Source"`
	Tags      []camietagger.Tag `json:"tags"`
}

type booruPost struct {
	ID                 string
	MD5                string
	Tags               string
	TagString          string
	TagStringGeneral   string
	TagStringCharacter string
	TagStringCopyright string
	TagStringArtist    string
	TagStringMeta      string
}

type booruProvider struct {
	name               string
	lookupURL          func(md5 string) string
	postURL            func(id string) string
	parse              func([]byte) (*booruPost, error)
	batchTagInfoURL    func([]string) string
	singleTagInfoURL   func(string) string
	parseTagCategories func([]byte) (map[string]string, error)
}

type booruJSONPost struct {
	ID                 json.RawMessage `json:"id"`
	MD5                string          `json:"md5"`
	Tags               string          `json:"tags"`
	TagString          string          `json:"tag_string"`
	TagStringGeneral   string          `json:"tag_string_general"`
	TagStringCharacter string          `json:"tag_string_character"`
	TagStringCopyright string          `json:"tag_string_copyright"`
	TagStringArtist    string          `json:"tag_string_artist"`
	TagStringMeta      string          `json:"tag_string_meta"`
}

type gelbooruJSONResponse struct {
	Post []booruJSONPost `json:"post"`
}

type booruTagInfo struct {
	Name    string          `json:"name"`
	Type    json.RawMessage `json:"type"`
	TagType json.RawMessage `json:"tag_type"`
}

type gelbooruTagInfoResponse struct {
	Tag []booruTagInfo `json:"tag"`
}

var booruProviders = []booruProvider{
	{
		name: "Danbooru",
		lookupURL: func(hash string) string {
			return "https://danbooru.donmai.us/posts.json?limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string { return "https://danbooru.donmai.us/posts/" + id },
		parse:   parseBooruArray,
	},
	{
		name: "Gelbooru",
		lookupURL: func(hash string) string {
			return "https://gelbooru.com/index.php?page=dapi&s=post&q=index&json=1&limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string {
			return "https://gelbooru.com/index.php?page=post&s=view&id=" + url.QueryEscape(id)
		},
		parse: parseGelbooruResponse,
		batchTagInfoURL: func(names []string) string {
			return "https://gelbooru.com/index.php?page=dapi&s=tag&q=index&json=1&limit=100&names=" + url.QueryEscape(strings.Join(names, " "))
		},
		parseTagCategories: parseGelbooruTagCategories,
	},
	{
		name: "Yande.re",
		lookupURL: func(hash string) string {
			return "https://yande.re/post.json?limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string { return "https://yande.re/post/show/" + id },
		parse:   parseBooruArray,
		singleTagInfoURL: func(name string) string {
			return "https://yande.re/tag.json?name=" + url.QueryEscape(name)
		},
		parseTagCategories: parseMoebooruTagCategories,
	},
	{
		name: "Konachan",
		lookupURL: func(hash string) string {
			return "https://konachan.com/post.json?limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string { return "https://konachan.com/post/show/" + id },
		parse:   parseBooruArray,
		singleTagInfoURL: func(name string) string {
			return "https://konachan.com/tag.json?name=" + url.QueryEscape(name)
		},
		parseTagCategories: parseMoebooruTagCategories,
	},
	{
		name: "Safebooru",
		lookupURL: func(hash string) string {
			return "https://safebooru.org/index.php?page=dapi&s=post&q=index&json=1&limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string {
			return "https://safebooru.org/index.php?page=post&s=view&id=" + url.QueryEscape(id)
		},
		parse: parseGelbooruResponse,
		batchTagInfoURL: func(names []string) string {
			return "https://safebooru.org/index.php?page=dapi&s=tag&q=index&json=1&limit=100&names=" + url.QueryEscape(strings.Join(names, " "))
		},
		parseTagCategories: parseGelbooruTagCategories,
	},
}

func rawBooruID(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if err := json.Unmarshal(value, &number); err == nil {
		return number.String()
	}
	return strings.Trim(strings.TrimSpace(string(value)), `"`)
}

func convertBooruPost(post booruJSONPost) *booruPost {
	return &booruPost{
		ID:                 rawBooruID(post.ID),
		MD5:                strings.ToLower(strings.TrimSpace(post.MD5)),
		Tags:               post.Tags,
		TagString:          post.TagString,
		TagStringGeneral:   post.TagStringGeneral,
		TagStringCharacter: post.TagStringCharacter,
		TagStringCopyright: post.TagStringCopyright,
		TagStringArtist:    post.TagStringArtist,
		TagStringMeta:      post.TagStringMeta,
	}
}

func parseBooruArray(data []byte) (*booruPost, error) {
	var posts []booruJSONPost
	if err := json.Unmarshal(data, &posts); err != nil {
		return nil, fmt.Errorf("decoding booru response: %w", err)
	}
	if len(posts) == 0 {
		return nil, errBooruNoMatch
	}
	return convertBooruPost(posts[0]), nil
}

func parseGelbooruResponse(data []byte) (*booruPost, error) {
	var wrapped gelbooruJSONResponse
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Post) > 0 {
		return convertBooruPost(wrapped.Post[0]), nil
	}

	var posts []booruJSONPost
	if err := json.Unmarshal(data, &posts); err == nil {
		if len(posts) == 0 {
			return nil, errBooruNoMatch
		}
		return convertBooruPost(posts[0]), nil
	}
	return nil, errBooruNoMatch
}

func filenameBooruMD5(path string) string {
	match := booruMD5Pattern.FindString(filepath.Base(path))
	return strings.ToLower(match)
}

func calculateBooruMD5(path string) (string, error) {
	input, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening image for MD5: %w", err)
	}
	defer input.Close()

	hash := md5.New() //nolint:gosec // Booru APIs require the file's MD5 identifier.
	if _, err := io.Copy(hash, input); err != nil {
		return "", fmt.Errorf("hashing image for booru lookup: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func splitBooruTags(value string) []string {
	return strings.Fields(strings.TrimSpace(value))
}

func tagCategoryFromType(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var number int
	if err := json.Unmarshal(raw, &number); err != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return ""
		}
		text = strings.ToLower(strings.TrimSpace(text))
		switch text {
		case "0", "general":
			return "general"
		case "1", "artist":
			return "artist"
		case "3", "copyright":
			return "copyright"
		case "4", "character":
			return "character"
		case "5", "meta":
			return "meta"
		default:
			return ""
		}
	}

	switch number {
	case 0:
		return "general"
	case 1:
		return "artist"
	case 3:
		return "copyright"
	case 4:
		return "character"
	case 5:
		return "meta"
	default:
		return ""
	}
}

func tagCategoriesFromInfo(items []booruTagInfo) map[string]string {
	result := make(map[string]string, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		rawType := item.TagType
		if len(rawType) == 0 {
			rawType = item.Type
		}
		if category := tagCategoryFromType(rawType); category != "" {
			result[strings.ToLower(name)] = category
		}
	}
	return result
}

func parseGelbooruTagCategories(data []byte) (map[string]string, error) {
	var wrapped gelbooruTagInfoResponse
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Tag != nil {
		return tagCategoriesFromInfo(wrapped.Tag), nil
	}
	var items []booruTagInfo
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decoding tag categories: %w", err)
	}
	return tagCategoriesFromInfo(items), nil
}

func parseMoebooruTagCategories(data []byte) (map[string]string, error) {
	var items []booruTagInfo
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decoding tag categories: %w", err)
	}
	return tagCategoriesFromInfo(items), nil
}

func booruTags(post *booruPost, source string, categories map[string]string) []camietagger.Tag {
	var result []camietagger.Tag
	seen := map[string]bool{}
	add := func(category string, names []string) {
		for _, rawName := range names {
			rawName = strings.TrimSpace(rawName)
			if rawName == "" {
				continue
			}
			itemCategory := category
			if resolved := categories[strings.ToLower(rawName)]; resolved != "" {
				itemCategory = resolved
			}
			key := itemCategory + "\x00" + strings.ToLower(rawName)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, camietagger.Tag{
				Name:     rawName,
				RawName:  rawName,
				Category: itemCategory,
				Score:    1,
				Source:   "booru:" + strings.ToLower(source),
			})
		}
	}

	categorized := strings.TrimSpace(post.TagStringGeneral) != "" ||
		strings.TrimSpace(post.TagStringCharacter) != "" ||
		strings.TrimSpace(post.TagStringCopyright) != "" ||
		strings.TrimSpace(post.TagStringArtist) != "" ||
		strings.TrimSpace(post.TagStringMeta) != ""
	if categorized {
		add("artist", splitBooruTags(post.TagStringArtist))
		add("copyright", splitBooruTags(post.TagStringCopyright))
		add("character", splitBooruTags(post.TagStringCharacter))
		add("general", splitBooruTags(post.TagStringGeneral))
		add("meta", splitBooruTags(post.TagStringMeta))
		return result
	}

	tags := post.TagString
	if strings.TrimSpace(tags) == "" {
		tags = post.Tags
	}
	add("general", splitBooruTags(tags))
	return result
}

func fetchBooruURL(ctx context.Context, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StashBooru/1.0 (+https://github.com/Dusky-dev/StashBooru)")

	response, err := booruHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, errBooruNoMatch
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, booruResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > booruResponseLimit {
		return nil, fmt.Errorf("response exceeds %d bytes", booruResponseLimit)
	}
	return data, nil
}

func fetchBooruProvider(ctx context.Context, provider booruProvider, hash string) (*booruPost, error) {
	data, err := fetchBooruURL(ctx, provider.lookupURL(hash))
	if err != nil {
		return nil, err
	}
	post, err := provider.parse(data)
	if err != nil {
		return nil, err
	}
	if post.MD5 != "" && !strings.EqualFold(post.MD5, hash) {
		return nil, errBooruNoMatch
	}
	return post, nil
}

type booruLookupResult struct {
	index    int
	provider booruProvider
	post     *booruPost
	err      error
}

func lookupBooruPost(ctx context.Context, hash string) (booruProvider, *booruPost, error) {
	ctx, cancel := context.WithTimeout(ctx, booruLookupTimeout)
	defer cancel()

	results := make(chan booruLookupResult, len(booruProviders))
	for index, provider := range booruProviders {
		go func(index int, provider booruProvider) {
			post, err := fetchBooruProvider(ctx, provider, hash)
			results <- booruLookupResult{index: index, provider: provider, post: post, err: err}
		}(index, provider)
	}

	completed := make([]bool, len(booruProviders))
	successes := make([]*booruLookupResult, len(booruProviders))
	var providerErrors []string
	noMatchCount := 0

	for received := 0; received < len(booruProviders); received++ {
		select {
		case result := <-results:
			completed[result.index] = true
			switch {
			case result.err == nil:
				copyResult := result
				successes[result.index] = &copyResult
			case errors.Is(result.err, errBooruNoMatch):
				noMatchCount++
			default:
				providerErrors = append(providerErrors, result.provider.name+": "+result.err.Error())
			}

			for index, success := range successes {
				if success == nil {
					continue
				}
				higherPriorityDone := true
				for higher := 0; higher < index; higher++ {
					if !completed[higher] {
						higherPriorityDone = false
						break
					}
				}
				if higherPriorityDone {
					return success.provider, success.post, nil
				}
			}
		case <-ctx.Done():
			for _, success := range successes {
				if success != nil {
					return success.provider, success.post, nil
				}
			}
			if len(providerErrors) > 0 {
				return booruProvider{}, nil, fmt.Errorf("booru lookup timed out: %s", strings.Join(providerErrors, "; "))
			}
			return booruProvider{}, nil, fmt.Errorf("booru lookup timed out: %w", ctx.Err())
		}
	}

	if noMatchCount == len(booruProviders) {
		return booruProvider{}, nil, errBooruNoMatch
	}
	if len(providerErrors) > 0 {
		return booruProvider{}, nil, fmt.Errorf("booru lookups failed: %s", strings.Join(providerErrors, "; "))
	}
	return booruProvider{}, nil, errBooruNoMatch
}

func flatBooruTagNames(post *booruPost) []string {
	if post == nil {
		return nil
	}
	if strings.TrimSpace(post.TagStringGeneral) != "" ||
		strings.TrimSpace(post.TagStringCharacter) != "" ||
		strings.TrimSpace(post.TagStringCopyright) != "" ||
		strings.TrimSpace(post.TagStringArtist) != "" ||
		strings.TrimSpace(post.TagStringMeta) != "" {
		return nil
	}
	value := post.TagString
	if strings.TrimSpace(value) == "" {
		value = post.Tags
	}
	return splitBooruTags(value)
}

func resolveBooruTagCategories(ctx context.Context, provider booruProvider, post *booruPost) map[string]string {
	names := flatBooruTagNames(post)
	if len(names) == 0 || provider.parseTagCategories == nil {
		return nil
	}

	result := map[string]string{}
	if provider.batchTagInfoURL != nil {
		for start := 0; start < len(names); start += booruGelbooruTagBatch {
			end := min(start+booruGelbooruTagBatch, len(names))
			data, err := fetchBooruURL(ctx, provider.batchTagInfoURL(names[start:end]))
			if err != nil {
				continue
			}
			categories, err := provider.parseTagCategories(data)
			if err != nil {
				continue
			}
			for name, category := range categories {
				result[name] = category
			}
		}
		return result
	}

	if provider.singleTagInfoURL == nil {
		return nil
	}
	if len(names) > booruMaxSingleTagLookup {
		names = names[:booruMaxSingleTagLookup]
	}

	type tagResult struct {
		categories map[string]string
	}
	jobs := make(chan string)
	results := make(chan tagResult, len(names))
	workerCount := min(booruTagLookupWorkers, len(names))
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for range workerCount {
		go func() {
			defer wg.Done()
			for name := range jobs {
				data, err := fetchBooruURL(ctx, provider.singleTagInfoURL(name))
				if err != nil {
					continue
				}
				categories, err := provider.parseTagCategories(data)
				if err == nil {
					results <- tagResult{categories: categories}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, name := range names {
			select {
			case jobs <- name:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	for item := range results {
		for name, category := range item.categories {
			result[name] = category
		}
	}
	return result
}

type booruLookupFunc func(context.Context, string) (booruProvider, *booruPost, error)

func lookupImageBooruMetadata(ctx context.Context, path string, lookup booruLookupFunc) (booruProvider, *booruPost, string, string, error) {
	filenameHash := filenameBooruMD5(path)
	if filenameHash != "" {
		provider, post, err := lookup(ctx, filenameHash)
		if err == nil {
			return provider, post, filenameHash, "filename", nil
		}
		if !errors.Is(err, errBooruNoMatch) {
			return booruProvider{}, nil, "", "", err
		}
	}

	fileHash, err := calculateBooruMD5(path)
	if err != nil {
		return booruProvider{}, nil, "", "", err
	}
	if filenameHash != "" && strings.EqualFold(filenameHash, fileHash) {
		return booruProvider{}, nil, fileHash, "file", errBooruNoMatch
	}

	provider, post, err := lookup(ctx, fileHash)
	if err != nil {
		return booruProvider{}, nil, fileHash, "file", err
	}
	return provider, post, fileHash, "file", nil
}

func (rs imageRoutes) ImageBooruMetadata(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	provider, post, hash, hashSource, err := lookupImageBooruMetadata(r.Context(), primary.Base().Path, lookupBooruPost)
	if errors.Is(err, errBooruNoMatch) {
		http.Error(w, "no matching booru post found for image MD5", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	categoryCtx, cancel := context.WithTimeout(r.Context(), booruRequestTimeout)
	categories := resolveBooruTagCategories(categoryCtx, provider, post)
	cancel()

	predictions := booruTags(post, provider.name, categories)
	for index := range predictions {
		predictions[index] = normalizeCamiePrediction(predictions[index])
	}
	predictions = enrichCamiePredictionTargets(r.Context(), predictions)

	postURL := ""
	if post.ID != "" && provider.postURL != nil {
		postURL = provider.postURL(post.ID)
	}
	writeVisualSimilarityJSON(w, booruMetadataResponse{
		Source:    provider.name,
		PostID:    post.ID,
		PostURL:   postURL,
		MD5:       hash,
		MD5Source: hashSource,
		Tags:      predictions,
	})
}
