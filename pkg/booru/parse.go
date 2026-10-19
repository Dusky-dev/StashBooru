package booru

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type object map[string]json.RawMessage

var postID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var md5Hash = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

func text(value json.RawMessage) string {
	var result string
	if json.Unmarshal(value, &result) == nil {
		return result
	}
	var number json.Number
	if json.Unmarshal(value, &number) == nil {
		return number.String()
	}
	return ""
}

func number(value json.RawMessage) int64 {
	ret, err := strconv.ParseInt(text(value), 10, 64)
	if err != nil || ret < 0 {
		return 0
	}
	return ret
}

func child(value json.RawMessage) object {
	var ret object
	_ = json.Unmarshal(value, &ret)
	return ret
}

func parse(source Source, data []byte, page, limit int) (Result, error) {
	result := Result{SourceID: source.ID, Page: page, Limit: limit, Posts: []Post{}}
	var records []object
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return result, fmt.Errorf("empty response")
	}
	if data[0] == '[' {
		if err := json.Unmarshal(data, &records); err != nil {
			return result, err
		}
	} else {
		var envelope object
		if err := json.Unmarshal(data, &envelope); err != nil {
			return result, err
		}
		if success, ok := envelope["success"]; ok && string(success) == "false" {
			return result, fmt.Errorf("provider rejected the query")
		}
		key := "post"
		switch source.Engine {
		case "e621":
			key = "posts"
		case "philomena":
			key = "images"
		}
		raw, exists := envelope[key]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return result, fmt.Errorf("missing post array")
		}
		if err := json.Unmarshal(raw, &records); err != nil {
			return result, err
		}
		if source.Engine == "philomena" && envelope["total"] != nil && !bytes.Equal(bytes.TrimSpace(envelope["total"]), []byte("null")) {
			total, err := strconv.ParseInt(text(envelope["total"]), 10, 64)
			if err != nil || total < 0 {
				return result, fmt.Errorf("invalid provider total")
			}
			result.Total = &total
		}
	}
	if len(records) > limit {
		return result, fmt.Errorf("provider ignored the result limit")
	}
	seen := map[string]bool{}
	for _, raw := range records {
		post, err := parsePost(source, raw)
		if err != nil {
			return result, err
		}
		if !seen[post.ID] {
			result.Posts = append(result.Posts, post)
			seen[post.ID] = true
		}
	}
	result.HasMore = len(records) == limit && page < MaxPage
	if result.Total != nil && int64(page*limit) >= *result.Total {
		result.HasMore = false
	}
	return result, nil
}

func parsePost(source Source, raw object) (Post, error) {
	id := text(raw["id"])
	if _, err := strconv.ParseUint(id, 10, 64); err != nil || !postID.MatchString(id) {
		return Post{}, fmt.Errorf("invalid remote post identity")
	}
	post := Post{ID: id, SourceID: source.ID, Width: number(raw["width"]), Height: number(raw["height"]),
		Bytes: number(raw["file_size"]), MD5: text(raw["md5"]), Rating: text(raw["rating"]),
		Format: text(raw["file_ext"]), Tags: []Tag{}, RemoteID: id, Title: text(raw["title"]), Author: text(raw["author"]), AuthorID: text(raw["creator_id"]), Filename: text(raw["file_name"]), CreatedAt: text(raw["created_at"]), Score: number(raw["score"])}
	preview, sample := text(raw["preview_file_url"]), text(raw["large_file_url"])
	original := text(raw["file_url"])
	if post.Width == 0 {
		post.Width = number(raw["image_width"])
	}
	if post.Height == 0 {
		post.Height = number(raw["image_height"])
	}
	if preview == "" {
		preview = text(raw["preview_url"])
	}
	if sample == "" {
		sample = text(raw["sample_url"])
	}
	postURL := strings.TrimRight(source.URL, "/")
	switch source.Engine {
	case "gelbooru":
		postURL += "/index.php?" + url.Values{"page": {"post"}, "s": {"view"}, "id": {id}}.Encode()
	case "moebooru":
		postURL += "/post/show/" + id
	case "philomena":
		postURL += "/images/" + id
		reps := child(raw["representations"])
		preview, sample = text(reps["thumb"]), text(reps["large"])
		original = text(reps["full"])
		post.Format = text(raw["format"])
		post.Bytes = number(raw["size"])
	case "e621":
		postURL += "/posts/" + id
		// Current v2 and legacy API responses describe different wire layouts.
		if files := child(raw["files"]); files != nil {
			originalFile, meta := child(files["original"]), child(files["meta"])
			post.Width, post.Height = number(originalFile["width"]), number(originalFile["height"])
			post.Bytes, post.MD5, post.Format = number(meta["size"]), text(meta["md5"]), text(meta["ext"])
			original = text(originalFile["url"])
			if original == "" {
				original = text(originalFile[post.Format])
			}
			preview, sample = text(child(files["preview"])["jpg"]), text(child(files["sample"])["jpg"])
		} else {
			file := child(raw["file"])
			post.Width, post.Height = number(file["width"]), number(file["height"])
			post.Bytes, post.MD5, post.Format = number(file["size"]), text(file["md5"]), text(file["ext"])
			original = text(file["url"])
			preview, sample = text(child(raw["preview"])["url"]), text(child(raw["sample"])["url"])
		}
	default:
		postURL += "/posts/" + id
		if asset := child(raw["media_asset"]); asset != nil {
			var variants []object
			_ = json.Unmarshal(asset["variants"], &variants)
			for _, variant := range variants {
				switch text(variant["type"]) {
				case "180x180":
					if preview == "" {
						preview = text(variant["url"])
					}
				case "sample":
					if sample == "" {
						sample = text(variant["url"])
					}
				case "original":
					if original == "" {
						original = text(variant["url"])
					}
				}
			}
		}
	}
	post.OriginalURL = source.postMediaURL(original)
	if post.Format == "" {
		post.Format = mediaExtension(original)
	}
	post.DownloadAvailable = post.OriginalURL != ""
	post.PostURL, post.PreviewURL, post.SampleURL = postURL, source.postMediaURL(preview), source.postMediaURL(sample)
	// Do not substitute a potentially huge original for a missing preview.
	if post.SampleURL == "" {
		post.SampleURL = post.PreviewURL
	}
	if !md5Hash.MatchString(post.MD5) {
		post.MD5 = ""
	} else {
		post.MD5 = strings.ToLower(post.MD5)
	}
	if len(post.Rating) > 80 || len(post.Format) > 32 {
		return Post{}, fmt.Errorf("invalid file metadata")
	}
	if err := parseTags(raw, &post); err != nil {
		return Post{}, err
	}
	return post, nil
}

func parseTags(raw object, post *Post) error {
	seen := map[string]bool{}
	add := func(name, category string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		if len(name) > 1024 || len(category) > 80 || len(post.Tags) >= 2000 {
			return fmt.Errorf("provider tag limits exceeded")
		}
		key := category + "\x00" + name
		if !seen[key] {
			seen[key] = true
			post.Tags = append(post.Tags, Tag{Name: name, Category: category})
		}
		return nil
	}
	if tags := raw["tags"]; len(tags) > 0 && string(tags) != "null" {
		var flat []string
		var categories map[string][]string
		switch {
		case json.Unmarshal(tags, &flat) == nil:
			for _, name := range flat {
				if err := add(name, "unknown"); err != nil {
					return err
				}
			}
		case json.Unmarshal(tags, &categories) == nil:
			keys := make([]string, 0, len(categories))
			for key := range categories {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, category := range keys {
				for _, name := range categories[category] {
					if err := add(name, category); err != nil {
						return err
					}
				}
			}
		default:
			var names string
			if json.Unmarshal(tags, &names) != nil {
				return fmt.Errorf("unsupported tags")
			}
			for _, name := range strings.Fields(names) {
				if err := add(name, "unknown"); err != nil {
					return err
				}
			}
		}
	}
	typed := false
	for _, category := range []string{"artist", "character", "copyright", "general", "meta"} {
		field := raw["tag_string_"+category]
		if field != nil {
			typed = true
		}
		for _, name := range strings.Fields(text(field)) {
			if err := add(name, category); err != nil {
				return err
			}
		}
	}
	if !typed && len(post.Tags) == 0 {
		for _, name := range strings.Fields(text(raw["tag_string"])) {
			if err := add(name, "unknown"); err != nil {
				return err
			}
		}
	}
	return nil
}

// Some compatible APIs still advertise HTTP media URLs. Upgrade them before
// validation; requests and redirects remain HTTPS-only on declared CDN hosts.
func (s Source) postMediaURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Scheme == "http" && (u.Port() == "" || u.Port() == "80") {
		u.Scheme = "https"
		u.Host = u.Hostname()
		raw = u.String()
	}
	return s.mediaURL(raw)
}
