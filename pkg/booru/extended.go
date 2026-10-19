package booru

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var assetID = regexp.MustCompile(`^(?:[1-9][0-9]{0,19}|[ap][a-f0-9]{32})$`)

// Asset IDs retain existing numeric identities and give archive/gallery files a
// bounded filesystem-safe identity independent of display names and pagination.
func ValidPostID(id string) bool {
	if !assetID.MatchString(id) {
		return false
	}
	if id[0] == 'a' || id[0] == 'p' {
		return true
	}
	_, err := strconv.ParseUint(id, 10, 64)
	return err == nil
}
func stableAssetID(prefix, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return prefix + hex.EncodeToString(sum[:16])
}

func (c *Client) extendedRequest(ctx context.Context, source Source, endpoint string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(source.URL, "/")+endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid provider request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	if source.Engine == "pixiv" {
		req.Header.Set("Authorization", "Bearer "+source.APIKey)
	}
	if source.Engine == "kemono" {
		req.Header.Set("Accept", "text/css")
	}
	if source.Engine == "shimmie" {
		req.Header.Set("Accept", "application/xml")
	}
	return c.fetchJSON(ctx, source, req, responseLimit)
}
func (c *Client) searchExtended(ctx context.Context, s Source, query string, page, limit int, options SearchOptions) (Result, error) {
	result := Result{SourceID: s.ID, Page: page, Limit: limit, Posts: []Post{}, FetchedAt: time.Now().UTC()}
	if !options.Validate() || options.Rating != "" || (options.Sort != "" && options.Sort != "newest") {
		return result, &Error{Code: "unsupported-filter", Message: "This engine supports provider ordering without a rating filter. Clear sort/rating controls."}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if s.Engine == "shimmie" {
		return c.searchShimmie(ctx, s, query, page, limit)
	}
	if s.Engine == "pixiv" && s.APIKey == "" {
		return result, &Error{Code: "authentication", Message: "Pixiv requires an OAuth access token. Add it in this source's API key setting."}
	}
	block := 50
	endpoint := "/api/v1/posts"
	params := url.Values{}
	if s.Engine == "pixiv" {
		block = 30
		endpoint = "/v1/search/illust"
		params = url.Values{"word": {query}, "search_target": {"partial_match_for_tags"}, "sort": {"date_desc"}, "filter": {"for_android"}}
		if strings.TrimSpace(query) == "" {
			endpoint = "/v1/illust/ranking"
			params = url.Values{"mode": {"day"}, "filter": {"for_android"}}
		}
		if strings.HasPrefix(query, "user:") {
			id := strings.TrimPrefix(query, "user:")
			if !postID.MatchString(id) {
				return result, &Error{Code: "invalid-query", Message: "Use user:123 alone to browse a Pixiv artist."}
			}
			endpoint = "/v1/user/illusts"
			params = url.Values{"user_id": {id}, "filter": {"for_android"}}
		}
		if strings.HasPrefix(query, "id:") {
			id := strings.TrimPrefix(query, "id:")
			if !postID.MatchString(id) {
				return result, &Error{Code: "invalid-query", Message: "Use id:123 alone for a Pixiv work."}
			}
			if page > 1 {
				return result, nil
			}
			data, err := c.extendedRequest(ctx, s, "/v1/illust/detail", url.Values{"illust_id": {id}})
			if err != nil {
				return result, err
			}
			var envelope struct {
				Illust object `json:"illust"`
			}
			if json.Unmarshal(data, &envelope) != nil || envelope.Illust == nil {
				return result, extendedParseError()
			}
			post, err := parsePixivPost(s, envelope.Illust)
			if err != nil {
				return result, extendedParseError()
			}
			result.Posts = append(result.Posts, post)
			return result, nil
		}
	} else {
		if strings.HasPrefix(query, "creator:") {
			parts := strings.Split(query, ":")
			if len(parts) != 3 || !safeProviderPart(parts[1]) || !safeProviderPart(parts[2]) {
				return result, &Error{Code: "invalid-query", Message: "Use creator:service:id alone to browse a creator."}
			}
			endpoint = "/api/v1/" + url.PathEscape(parts[1]) + "/user/" + url.PathEscape(parts[2])
			if s.Engine == "kemono" {
				endpoint += "/posts"
			}
		} else if query != "*" && strings.TrimSpace(query) != "" {
			if s.Engine == "pawchive" {
				return result, &Error{Code: "unsupported-query", Message: "Pawchive's API does not filter text searches. Use * for recent posts or creator:service:id to browse a creator."}
			}
			if len(query) < 3 {
				return result, &Error{Code: "invalid-query", Message: "Archive text searches need at least three bytes; use * for recent posts."}
			}
			params.Set("q", query)
		}
	}
	// Both API families have fixed upstream page sizes. Slice offset-aligned
	// blocks, rather than dropping records or pretending the API honors limit.
	start := (page - 1) * limit
	end := start + limit
	for offset := (start / block) * block; offset < end; offset += block {
		key := "o"
		if s.Engine == "pixiv" {
			key = "offset"
		}
		params.Set(key, strconv.Itoa(offset))
		data, err := c.extendedRequest(ctx, s, endpoint, params)
		if err != nil {
			return result, err
		}
		rows, err := extendedRows(s.Engine, data)
		if err != nil || len(rows) > block {
			return result, extendedParseError()
		}
		for i, raw := range rows {
			at := offset + i
			if at < start || at >= end {
				continue
			}
			var post Post
			if s.Engine == "pixiv" {
				post, err = parsePixivPost(s, raw)
			} else {
				post, err = parseArchivePost(s, raw)
			}
			if err != nil {
				return result, extendedParseError()
			}
			result.Posts = append(result.Posts, post)
		}
		result.HasMore = len(rows) == block || offset+len(rows) > end
		if len(rows) < block {
			break
		}
	}
	result.HasMore = result.HasMore && page < MaxPage
	return result, nil
}
func extendedParseError() error {
	return &Error{Code: "invalid-response", Message: "Provider returned unsupported data or exceeded its bounds. Check API access and source settings."}
}
func safeProviderPart(value string) bool {
	return value != "" && len(value) <= 256 && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\r\n\x00 ")
}
func extendedRows(engine string, data []byte) ([]object, error) {
	var rows []object
	if json.Unmarshal(data, &rows) == nil && rows != nil {
		return rows, nil
	}
	var envelope object
	if json.Unmarshal(data, &envelope) != nil || envelope["error"] != nil {
		return nil, fmt.Errorf("invalid envelope")
	}
	key := "posts"
	if engine == "pixiv" {
		key = "illusts"
	}
	if json.Unmarshal(envelope[key], &rows) != nil || rows == nil {
		return nil, fmt.Errorf("missing records")
	}
	return rows, nil
}
func mediaExtension(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
}
func supportedArchiveFile(ext string) bool {
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "apng", "avif", "jxl", "bmp", "tiff", "mp4", "webm":
		return true
	}
	return false
}
func parsePixivPost(s Source, raw object) (Post, error) {
	id := text(raw["id"])
	if !ValidPostID(id) || !postID.MatchString(id) {
		return Post{}, fmt.Errorf("invalid Pixiv ID")
	}
	user := child(raw["user"])
	uid := text(user["id"])
	p := Post{ID: id, RemoteID: id, SourceID: s.ID, PostURL: "https://www.pixiv.net/artworks/" + id, Title: text(raw["title"]), Author: text(user["name"]), AuthorID: uid, Width: number(raw["width"]), Height: number(raw["height"]), CreatedAt: text(raw["create_date"]), Score: number(raw["total_bookmarks"]), Tags: []Tag{}, Position: 1}
	p.Rating = "safe"
	if number(raw["x_restrict"]) > 0 {
		p.Rating = "explicit"
	}
	if postID.MatchString(uid) {
		p.Tags = append(p.Tags, Tag{Name: "user:" + uid, Label: p.Author, Category: "artist"})
	}
	var tags []object
	if json.Unmarshal(raw["tags"], &tags) != nil || len(tags) > 2000 {
		return p, fmt.Errorf("invalid tags")
	}
	for _, tag := range tags {
		name := text(tag["name"])
		if validTagText(name) {
			p.Tags = append(p.Tags, Tag{Name: name, Category: "unknown"})
		}
	}
	images := child(raw["image_urls"])
	original := text(child(raw["meta_single_page"])["original_image_url"])
	var pages []object
	_ = json.Unmarshal(raw["meta_pages"], &pages)
	if len(pages) > 200 {
		return p, fmt.Errorf("too many pages")
	}
	p.PreviewURL = s.postMediaURL(text(images["square_medium"]))
	if p.PreviewURL == "" {
		p.PreviewURL = s.postMediaURL(text(images["medium"]))
	}
	p.SampleURL = s.postMediaURL(text(images["large"]))
	if len(pages) > 0 {
		original = text(child(pages[0]["image_urls"])["original"])
	}
	if text(raw["type"]) == "ugoira" {
		original = ""
		p.MediaNotice = "Ugoira frame ZIPs are not supported for download. Open the remote work."
	} // Frame ZIPs require a separate conversion/import workflow.
	p.OriginalURL = s.postMediaURL(original)
	p.DownloadAvailable = p.OriginalURL != ""
	p.Format = mediaExtension(original)
	p.Filename = path.Base(original)
	for i, row := range pages {
		if i == 0 {
			continue
		}
		asset := p
		asset.Attachments = nil
		asset.Tags = nil
		asset.ID = stableAssetID("p", id+":"+strconv.Itoa(i))
		asset.Position = i + 1
		urls := child(row["image_urls"])
		asset.OriginalURL = s.postMediaURL(text(urls["original"]))
		asset.PreviewURL = s.postMediaURL(text(urls["medium"]))
		asset.SampleURL = s.postMediaURL(text(urls["large"]))
		asset.Format = mediaExtension(asset.OriginalURL)
		asset.DownloadAvailable = asset.OriginalURL != ""
		asset.Filename = path.Base(asset.OriginalURL)
		p.Attachments = append(p.Attachments, asset)
	}
	return p, nil
}
func parseArchivePost(s Source, raw object) (Post, error) {
	service, user, id := text(raw["service"]), text(raw["user"]), text(raw["id"])
	if !safeProviderPart(service) || !safeProviderPart(user) || !safeProviderPart(id) {
		return Post{}, fmt.Errorf("invalid archive identity")
	}
	identity := service + "\x00" + user + "\x00" + id
	p := Post{ID: stableAssetID("a", identity), RemoteID: id, SourceID: s.ID, PostURL: strings.TrimRight(s.URL, "/") + "/" + url.PathEscape(service) + "/user/" + url.PathEscape(user) + "/post/" + url.PathEscape(id), Title: text(raw["title"]), Author: service + "/" + user, AuthorID: user, CreatedAt: text(raw["published"]), Tags: []Tag{{Name: "creator:" + service + ":" + user, Label: service + "/" + user, Category: "artist"}}, Position: 1}
	files := []object{}
	if file := child(raw["file"]); text(file["path"]) != "" {
		files = append(files, file)
	}
	var attachments []object
	_ = json.Unmarshal(raw["attachments"], &attachments)
	if len(attachments) > 200 {
		return p, fmt.Errorf("too many attachments")
	}
	files = append(files, attachments...)
	seen := map[string]bool{}
	omitted := 0
	for _, file := range files {
		filePath := text(file["path"])
		ext := mediaExtension(filePath)
		if filePath == "" || seen[filePath] {
			continue
		}
		if !supportedArchiveFile(ext) {
			omitted++
			continue
		}
		seen[filePath] = true
		asset := p
		asset.Attachments = nil
		asset.Tags = nil
		if len(seen) > 1 {
			asset.ID = stableAssetID("a", identity+"\x00"+filePath)
		}
		base := strings.TrimRight(s.URL, "/")
		u, _ := url.Parse(base)
		if s.Engine == "pawchive" && u.Hostname() == "pawchive.pw" {
			base = "https://file.pawchive.pw"
		}
		original := filePath
		if !strings.HasPrefix(original, "https://") {
			original = base + "/data/" + strings.TrimPrefix(strings.TrimPrefix(original, "/data/"), "/")
		}
		asset.OriginalURL = s.postMediaURL(original)
		asset.DownloadAvailable = asset.OriginalURL != ""
		if string(file["preview_only"]) == "true" || string(file["deferred"]) == "true" || string(raw["has_full"]) == "false" {
			asset.OriginalURL = ""
			asset.DownloadAvailable = false
		}
		asset.Format = ext
		asset.Filename = text(file["name"])
		asset.Bytes = number(file["size"])
		asset.Position = len(seen)
		preview := strings.Replace(original, "/data/", "/thumbnail/data/", 1)
		if s.Engine == "pawchive" {
			preview = strings.Replace(preview, "https://file.pawchive.pw/", "https://img.pawchive.pw/", 1)
		}
		asset.PreviewURL = s.postMediaURL(preview)
		asset.SampleURL = asset.PreviewURL
		if len(seen) == 1 {
			p.OriginalURL, p.DownloadAvailable, p.Format, p.Filename, p.Bytes, p.PreviewURL, p.SampleURL = asset.OriginalURL, asset.DownloadAvailable, asset.Format, asset.Filename, asset.Bytes, asset.PreviewURL, asset.SampleURL
		} else {
			p.Attachments = append(p.Attachments, asset)
		}
	}
	if omitted > 0 {
		p.MediaNotice = fmt.Sprintf("%d non-media attachments omitted (archives/documents are not imported).", omitted)
	}
	if !p.DownloadAvailable {
		p.MediaNotice += " Full media is unavailable; previews are not substituted for originals."
	}
	return p, nil
}
func (c *Client) searchShimmie(ctx context.Context, s Source, query string, page, limit int) (Result, error) {
	result := Result{SourceID: s.ID, Page: page, Limit: limit, Posts: []Post{}, FetchedAt: time.Now().UTC()}
	data, err := c.extendedRequest(ctx, s, "/api/danbooru/find_posts/index.xml", url.Values{"tags": {query}, "page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}})
	if err != nil {
		return result, err
	}
	var envelope struct {
		XMLName xml.Name `xml:"posts"`
		Count   *int64   `xml:"count,attr"`
		Posts   []struct {
			Attrs []xml.Attr `xml:",any,attr"`
		} `xml:"post"`
	}
	if xml.Unmarshal(data, &envelope) != nil || len(envelope.Posts) > limit || (envelope.Count != nil && *envelope.Count < 0) {
		return result, extendedParseError()
	}
	for _, row := range envelope.Posts {
		raw := object{}
		for _, attr := range row.Attrs {
			v, err := json.Marshal(attr.Value)
			if err != nil {
				return result, extendedParseError()
			}
			raw[attr.Name.Local] = v
		}
		post, err := parsePost(s, raw)
		if err != nil {
			return result, extendedParseError()
		}
		post.PostURL = strings.TrimRight(s.URL, "/") + "/post/view/" + post.ID
		post.RemoteID = post.ID
		post.Format = mediaExtension(post.OriginalURL)
		result.Posts = append(result.Posts, post)
	}
	result.Total = envelope.Count
	result.HasMore = page < MaxPage && len(result.Posts) == limit
	if envelope.Count != nil {
		result.HasMore = page < MaxPage && int64(page*limit) < *envelope.Count
	}
	return result, nil
}
