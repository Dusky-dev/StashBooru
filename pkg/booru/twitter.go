package booru

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Twitter / X uses the official X API v2 with the user's own bearer token
// (API key setting). Queries:
//
//	@handle      that account's posts with media (resolved to user:<id>)
//	user:<id>    the same, by the stable numeric account ID
//	id:<id>      one post
//	other text   recent search (X limits it to the last 7 days)
//
// X pages with cursors; the browser's page numbers map to cursors remembered
// per query. Retweets and replies are left out of account timelines.

var twitterHandle = regexp.MustCompile(`^@?([A-Za-z0-9_]{1,15})$`)

const (
	twitterMaxWalk   = 5
	twitterMaxCursor = 2048
)

type twitterCursors struct {
	mu     sync.Mutex
	tokens map[string]string
	// fields is the post-fields parameter name the API accepted: the current
	// docs name it post.fields, older deployments tweet.fields.
	fields string
}

func (t *twitterCursors) get(key string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	token, ok := t.tokens[key]
	return token, ok
}

func (t *twitterCursors) set(key, token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tokens == nil || len(t.tokens) >= twitterMaxCursor {
		t.tokens = map[string]string{}
	}
	t.tokens[key] = token
}

func (t *twitterCursors) fieldsName() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.fields == "" {
		return "post.fields"
	}
	return t.fields
}

func (t *twitterCursors) swapFieldsName(failed string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fields = "tweet.fields"
	if failed == "tweet.fields" {
		t.fields = "post.fields"
	}
	return t.fields
}

func twitterCursorKey(s Source, endpoint, query string, limit, page int) string {
	sum := sha256.Sum256([]byte(s.Instance() + "\n" + endpoint + "\n" + query + "\n" + strconv.Itoa(limit) + "\n" + strconv.Itoa(page)))
	return hex.EncodeToString(sum[:])
}

func (c *Client) twitterRequest(ctx context.Context, s Source, endpoint string, params url.Values, postFields bool) ([]byte, error) {
	do := func(params url.Values) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.URL, "/")+endpoint+"?"+params.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("invalid provider request")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
		return c.fetchJSON(ctx, s, req, responseLimit)
	}
	if !postFields {
		return twitterError(do(params))
	}
	name := c.twitter.fieldsName()
	with := cloneValues(params)
	with.Set(name, "created_at,possibly_sensitive,entities,public_metrics,attachments,author_id")
	data, err := do(with)
	if err != nil && strings.Contains(err.Error(), "HTTP 400") {
		retry := cloneValues(params)
		retry.Set(c.twitter.swapFieldsName(name), with.Get(name))
		data, err = do(retry)
	}
	return twitterError(data, err)
}

func cloneValues(values url.Values) url.Values {
	out := url.Values{}
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

func twitterError(data []byte, err error) ([]byte, error) {
	var providerErr *Error
	if err == nil {
		return data, nil
	}
	if errors.As(err, &providerErr) {
		switch {
		case providerErr.Code == "authentication":
			return nil, &Error{Code: "authentication", Message: "X rejected the bearer token, or your X API access does not include this endpoint. Check the token in this source's API key setting."}
		case strings.Contains(providerErr.Message, "HTTP 402"):
			return nil, &Error{Code: "authentication", Message: "X API requires paid access or credits for this request. Check your X developer account."}
		}
	}
	return nil, err
}

func (c *Client) searchTwitter(ctx context.Context, s Source, query string, page, limit int, options SearchOptions) (Result, error) {
	result := Result{SourceID: s.ID, Page: page, Limit: limit, Posts: []Post{}, FetchedAt: time.Now().UTC()}
	if !options.Validate() || options.Rating != "" || (options.Sort != "" && options.Sort != "newest") {
		return result, &Error{Code: "unsupported-filter", Message: "X lists newest posts first and has no rating filter. Clear sort/rating controls."}
	}
	if s.APIKey == "" {
		return result, &Error{Code: "authentication", Message: "X requires your own API bearer token. Add it in this source's API key setting."}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	query = strings.TrimSpace(query)
	params := url.Values{"expansions": {"attachments.media_keys,author_id"}, "media.fields": {"media_key,type,url,preview_image_url,variants,width,height"}, "user.fields": {"username,name"}}
	if strings.HasPrefix(query, "id:") {
		id := strings.TrimPrefix(query, "id:")
		if !postID.MatchString(id) {
			return result, &Error{Code: "invalid-query", Message: "Use id:123 alone for one X post."}
		}
		if page > 1 {
			return result, nil
		}
		data, err := c.twitterRequest(ctx, s, "/2/tweets/"+id, params, true)
		if err != nil {
			return result, err
		}
		var single struct {
			Data     object `json:"data"`
			Includes object `json:"includes"`
		}
		if json.Unmarshal(data, &single) != nil || single.Data == nil {
			return result, extendedParseError()
		}
		posts, err := parseTwitterPosts(s, []object{single.Data}, single.Includes)
		if err != nil {
			return result, extendedParseError()
		}
		result.Posts = posts
		if len(posts) == 0 {
			result.SearchNotice = "This X post has no images or videos."
		}
		return result, nil
	}
	endpoint := "/2/tweets/search/recent"
	size := min(max(limit, 10), 100)
	switch {
	case query == "" || query == "*":
		return result, &Error{Code: "invalid-query", Message: "Enter @handle, user:123, id:123 or search words. X has no general recent feed."}
	case strings.HasPrefix(query, "user:"):
		id := strings.TrimPrefix(query, "user:")
		if !postID.MatchString(id) {
			return result, &Error{Code: "invalid-query", Message: "Use user:123 alone to browse an X account by its numeric ID, or @handle."}
		}
		endpoint = "/2/users/" + id + "/tweets"
		size = min(max(limit, 5), 100)
		params.Set("exclude", "retweets,replies")
	case strings.HasPrefix(query, "@") && !strings.Contains(query, " "):
		match := twitterHandle.FindStringSubmatch(query)
		if match == nil {
			return result, &Error{Code: "invalid-query", Message: "X handles have 1-15 letters, digits or underscores."}
		}
		id, err := c.twitterUserID(ctx, s, match[1])
		if err != nil {
			return result, err
		}
		endpoint = "/2/users/" + id + "/tweets"
		size = min(max(limit, 5), 100)
		params.Set("exclude", "retweets,replies")
		result.SearchNotice = "Showing @" + match[1] + " (user:" + id + ")."
	default:
		params.Set("query", query+" has:media -is:retweet")
		result.SearchNotice = "X searches only cover the last 7 days. Use @handle for an account's older posts."
	}
	params.Set("max_results", strconv.Itoa(size))
	result.Limit = size
	// Find the cursor for this page, walking forward from the nearest known
	// page within a small bound.
	token, from := "", 1
	for p := page; p > 1; p-- {
		if cursor, ok := c.twitter.get(twitterCursorKey(s, endpoint, query, size, p)); ok {
			token, from = cursor, p
			break
		}
	}
	if page-from > twitterMaxWalk {
		return result, &Error{Code: "invalid-page", Message: "X pages by cursor. Open the earlier pages first."}
	}
	for p := from; ; p++ {
		pageParams := cloneValues(params)
		if token != "" {
			pageParams.Set("pagination_token", token)
		}
		data, err := c.twitterRequest(ctx, s, endpoint, pageParams, true)
		if err != nil {
			return result, err
		}
		var envelope struct {
			Data     []object `json:"data"`
			Includes object   `json:"includes"`
			Meta     struct {
				NextToken string `json:"next_token"`
			} `json:"meta"`
		}
		if json.Unmarshal(data, &envelope) != nil || len(envelope.Data) > 100 || len(envelope.Meta.NextToken) > 512 {
			return result, extendedParseError()
		}
		next := envelope.Meta.NextToken
		if next != "" {
			c.twitter.set(twitterCursorKey(s, endpoint, query, size, p+1), next)
		}
		if p == page {
			posts, err := parseTwitterPosts(s, envelope.Data, envelope.Includes)
			if err != nil {
				return result, extendedParseError()
			}
			result.Posts = posts
			result.HasMore = next != "" && page < MaxPage
			return result, nil
		}
		if next == "" {
			return result, nil
		}
		token = next
	}
}

func (c *Client) twitterUserID(ctx context.Context, s Source, handle string) (string, error) {
	data, err := c.twitterRequest(ctx, s, "/2/users/by/username/"+url.PathEscape(handle), url.Values{}, false)
	if err != nil {
		return "", err
	}
	var user struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &user) != nil || !postID.MatchString(user.Data.ID) {
		return "", &Error{Code: "invalid-query", Message: "X has no account @" + handle + "."}
	}
	return user.Data.ID, nil
}

// twitterIcon is the account's profile picture at 400x400, or
// ErrNoCreatorPicture for X's default avatar.
func (c *Client) twitterIcon(ctx context.Context, s Source, target string) (string, error) {
	id := strings.TrimPrefix(target, "user:")
	if !strings.HasPrefix(target, "user:") || !postID.MatchString(id) {
		return "", fmt.Errorf("profile pictures are available for X accounts (user:123)")
	}
	if s.APIKey == "" {
		return "", &Error{Code: "authentication", Message: "X requires your own API bearer token. Add it in this source's API key setting."}
	}
	data, err := c.twitterRequest(ctx, s, "/2/users/"+id, url.Values{"user.fields": {"profile_image_url"}}, false)
	if err != nil {
		return "", err
	}
	var user struct {
		Data struct {
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &user) != nil {
		return "", extendedParseError()
	}
	raw := user.Data.ProfileImageURL
	if raw == "" || strings.Contains(raw, "default_profile_images") {
		return "", ErrNoCreatorPicture
	}
	raw = strings.Replace(raw, "_normal.", "_400x400.", 1)
	approved := s.mediaURL(raw)
	if approved == "" {
		return "", fmt.Errorf("the profile picture address is not on the source's media hosts")
	}
	return approved, nil
}

func parseTwitterPosts(s Source, rows []object, includes object) ([]Post, error) {
	var media, users []object
	if len(includes["media"]) > 0 && json.Unmarshal(includes["media"], &media) != nil {
		return nil, fmt.Errorf("invalid media")
	}
	if len(includes["users"]) > 0 && json.Unmarshal(includes["users"], &users) != nil {
		return nil, fmt.Errorf("invalid users")
	}
	mediaByKey := map[string]object{}
	for _, item := range media {
		mediaByKey[text(item["media_key"])] = item
	}
	usersByID := map[string]object{}
	for _, user := range users {
		usersByID[text(user["id"])] = user
	}
	posts := []Post{}
	for _, raw := range rows {
		post, ok, err := parseTwitterPost(s, raw, mediaByKey, usersByID)
		if err != nil {
			return nil, err
		}
		if ok {
			posts = append(posts, post)
		}
	}
	return posts, nil
}

func parseTwitterPost(s Source, raw object, media map[string]object, users map[string]object) (Post, bool, error) {
	id := text(raw["id"])
	if !postID.MatchString(id) || !ValidPostID(id) {
		return Post{}, false, fmt.Errorf("invalid X post ID")
	}
	uid := text(raw["author_id"])
	user := users[uid]
	handle := text(user["username"])
	postURL := "https://x.com/i/web/status/" + id
	if twitterHandle.MatchString(handle) {
		postURL = "https://x.com/" + handle + "/status/" + id
	}
	title := strings.Join(strings.Fields(text(raw["text"])), " ")
	if r := []rune(title); len(r) > 200 {
		title = string(r[:200])
	}
	p := Post{ID: id, RemoteID: id, SourceID: s.ID, PostURL: postURL, Title: title, Author: text(user["name"]), AuthorID: uid, CreatedAt: text(raw["created_at"]), Tags: []Tag{}, Position: 1, Rating: "safe"}
	if handle != "" {
		p.Author = "@" + handle
	}
	if string(raw["possibly_sensitive"]) == "true" {
		p.Rating = "explicit"
	}
	p.Score = number(child(raw["public_metrics"])["like_count"])
	if postID.MatchString(uid) {
		p.Tags = append(p.Tags, Tag{Name: "user:" + uid, Label: p.Author, Category: "artist"})
	}
	var hashtags []object
	_ = json.Unmarshal(child(raw["entities"])["hashtags"], &hashtags)
	for _, tag := range hashtags {
		if name := text(tag["tag"]); validTagText(name) && len(p.Tags) < 200 {
			p.Tags = append(p.Tags, Tag{Name: name, Category: "unknown"})
		}
	}
	var keys []string
	_ = json.Unmarshal(child(raw["attachments"])["media_keys"], &keys)
	if len(keys) > 20 {
		return Post{}, false, fmt.Errorf("too many media")
	}
	var assets []Post
	for _, key := range keys {
		item, found := media[key]
		if !found {
			continue
		}
		asset := twitterAsset(s, item)
		if asset.OriginalURL == "" && asset.PreviewURL == "" {
			continue
		}
		assets = append(assets, asset)
	}
	if len(assets) == 0 {
		return Post{}, false, nil
	}
	first := assets[0]
	p.OriginalURL, p.PreviewURL, p.SampleURL, p.Format, p.Filename, p.Width, p.Height, p.MediaNotice = first.OriginalURL, first.PreviewURL, first.SampleURL, first.Format, first.Filename, first.Width, first.Height, first.MediaNotice
	p.DownloadAvailable = p.OriginalURL != ""
	for i, asset := range assets[1:] {
		file := p
		file.Attachments, file.Tags = nil, nil
		file.ID = stableAssetID("p", "x:"+id+":"+strconv.Itoa(i+1))
		file.Position = i + 2
		file.OriginalURL, file.PreviewURL, file.SampleURL, file.Format, file.Filename, file.Width, file.Height, file.MediaNotice = asset.OriginalURL, asset.PreviewURL, asset.SampleURL, asset.Format, asset.Filename, asset.Width, asset.Height, asset.MediaNotice
		file.DownloadAvailable = file.OriginalURL != ""
		p.Attachments = append(p.Attachments, file)
	}
	return p, true, nil
}

// twitterAsset maps one X media item: photos at original size, videos and
// GIFs as their highest-bitrate MP4.
func twitterAsset(s Source, item object) Post {
	asset := Post{Width: number(item["width"]), Height: number(item["height"])}
	switch text(item["type"]) {
	case "photo":
		raw := text(item["url"])
		u, err := url.Parse(raw)
		if err != nil || raw == "" {
			return asset
		}
		ext := strings.TrimPrefix(path.Ext(u.Path), ".")
		base := strings.TrimSuffix(raw, "."+ext)
		if ext == "" {
			return asset
		}
		asset.OriginalURL = s.postMediaURL(base + "?format=" + url.QueryEscape(ext) + "&name=orig")
		asset.SampleURL = s.postMediaURL(base + "?format=" + url.QueryEscape(ext) + "&name=large")
		asset.PreviewURL = s.postMediaURL(base + "?format=" + url.QueryEscape(ext) + "&name=small")
		asset.Format = strings.ToLower(ext)
		asset.Filename = path.Base(u.Path)
	case "video", "animated_gif":
		var variants []object
		_ = json.Unmarshal(item["variants"], &variants)
		best, rate := "", int64(-1)
		for _, variant := range variants {
			if text(variant["content_type"]) == "video/mp4" && number(variant["bit_rate"]) > rate {
				best, rate = text(variant["url"]), number(variant["bit_rate"])
			}
		}
		asset.PreviewURL = s.postMediaURL(text(item["preview_image_url"]))
		asset.SampleURL = asset.PreviewURL
		asset.OriginalURL = s.postMediaURL(best)
		if asset.OriginalURL != "" {
			asset.Format = "mp4"
			u, _ := url.Parse(asset.OriginalURL)
			asset.Filename = path.Base(u.Path)
		} else {
			asset.MediaNotice = "X did not offer an MP4 for this video. Open the post on X."
		}
	}
	return asset
}
