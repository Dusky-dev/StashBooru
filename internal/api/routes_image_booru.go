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
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

const booruResponseLimit = 8 << 20

var (
	booruMD5Pattern = regexp.MustCompile(`(?i)[0-9a-f]{32}`)
	booruHTTPClient = &http.Client{Timeout: 15 * time.Second}
	errBooruNoMatch = errors.New("booru post not found")
)

type booruMetadataResponse struct {
	Source    string               `json:"source"`
	PostID    string               `json:"postID"`
	PostURL   string               `json:"postURL,omitempty"`
	MD5       string               `json:"md5"`
	MD5Source string               `json:"md5Source"`
	Tags      []camietagger.Tag    `json:"tags"`
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
	name      string
	lookupURL func(md5 string) string
	postURL   func(id string) string
	parse     func([]byte) (*booruPost, error)
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
	},
	{
		name: "Yande.re",
		lookupURL: func(hash string) string {
			return "https://yande.re/post.json?limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string { return "https://yande.re/post/show/" + id },
		parse:   parseBooruArray,
	},
	{
		name: "Konachan",
		lookupURL: func(hash string) string {
			return "https://konachan.com/post.json?limit=1&tags=" + url.QueryEscape("md5:"+hash)
		},
		postURL: func(id string) string { return "https://konachan.com/post/show/" + id },
		parse:   parseBooruArray,
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

func extractBooruMD5(path string) (string, string, error) {
	if match := booruMD5Pattern.FindString(filepath.Base(path)); match != "" {
		return strings.ToLower(match), "filename", nil
	}

	input, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("opening image for MD5: %w", err)
	}
	defer input.Close()

	hash := md5.New() //nolint:gosec // Booru APIs require the file's MD5 identifier.
	if _, err := io.Copy(hash, input); err != nil {
		return "", "", fmt.Errorf("hashing image for booru lookup: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), "file", nil
}

func splitBooruTags(value string) []string {
	return strings.Fields(strings.TrimSpace(value))
}

func booruTags(post *booruPost, source string) []camietagger.Tag {
	var result []camietagger.Tag
	seen := map[string]bool{}
	add := func(category string, names []string) {
		for _, rawName := range names {
			rawName = strings.TrimSpace(rawName)
			if rawName == "" {
				continue
			}
			key := category + "\x00" + strings.ToLower(rawName)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, camietagger.Tag{
				Name:     rawName,
				RawName:  rawName,
				Category: category,
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

func fetchBooruProvider(ctx context.Context, provider booruProvider, hash string) (*booruPost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.lookupURL(hash), nil)
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
	return provider.parse(data)
}

func lookupBooruPost(ctx context.Context, hash string) (booruProvider, *booruPost, error) {
	var providerErrors []string
	atLeastOneLookupSucceeded := false
	for _, provider := range booruProviders {
		post, err := fetchBooruProvider(ctx, provider, hash)
		if err == nil {
			return provider, post, nil
		}
		if errors.Is(err, errBooruNoMatch) {
			atLeastOneLookupSucceeded = true
			continue
		}
		providerErrors = append(providerErrors, provider.name+": "+err.Error())
	}
	if atLeastOneLookupSucceeded {
		return booruProvider{}, nil, errBooruNoMatch
	}
	if len(providerErrors) > 0 {
		return booruProvider{}, nil, fmt.Errorf("all booru lookups failed: %s", strings.Join(providerErrors, "; "))
	}
	return booruProvider{}, nil, errBooruNoMatch
}

func (rs imageRoutes) ImageBooruMetadata(w http.ResponseWriter, r *http.Request) {
	image := r.Context().Value(imageKey).(*models.Image)
	primary := image.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "image has no primary file", http.StatusNotFound)
		return
	}

	hash, hashSource, err := extractBooruMD5(primary.Base().Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	provider, post, err := lookupBooruPost(r.Context(), hash)
	if errors.Is(err, errBooruNoMatch) {
		http.Error(w, "no matching booru post found for MD5 "+hash, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	predictions := booruTags(post, provider.name)
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

func booruPostID(value string) int {
	id, _ := strconv.Atoi(value)
	return id
}
