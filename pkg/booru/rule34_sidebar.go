package booru

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const (
	rule34SidebarMaxPages = 3
	rule34SidebarMaxBytes = 512 << 10
)

// Rule34's public post pages have typed tag rows in #tag-sidebar, even
// when the tag DAPI does not support the requested bulk name lookup.
// Read only known tag links inside this sidebar, never tag-looking text
// elsewhere in the HTML (including scripts, user content, or navigation).
func parseRule34SidebarCategories(data []byte, requested map[string]bool) map[string]string {
	categories := map[string]string{}
	if len(data) == 0 || len(data) > rule34SidebarMaxBytes {
		return categories
	}
	z := html.NewTokenizer(bytes.NewReader(data))
	depth := 0
	category := ""
	for tokens := 0; tokens < 25000; tokens++ {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken && kind != html.EndTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		tag := string(name)
		if kind == html.EndTagToken {
			if depth > 0 {
				switch tag {
				case "ul":
					depth--
				case "li":
					category = ""
				}
			}
			continue
		}
		attrs := map[string]string{}
		for hasAttr {
			k, v, more := z.TagAttr()
			attrs[string(k)] = string(v)
			hasAttr = more
		}
		if tag == "ul" {
			if depth > 0 {
				depth++
			} else if attrs["id"] == "tag-sidebar" {
				depth = 1
			}
			continue
		}
		if depth == 0 {
			continue
		}
		if tag == "li" {
			category = ""
			for _, class := range strings.Fields(attrs["class"]) {
				switch class {
				case "tag-type-artist":
					category = "artist"
				case "tag-type-character":
					category = "character"
				case "tag-type-copyright":
					category = "copyright"
				case "tag-type-meta", "tag-type-metadata":
					category = "meta"
				case "tag-type-general":
					category = "general"
				}
			}
			continue
		}
		if tag != "a" || category == "" {
			continue
		}
		u, err := url.Parse(attrs["href"])
		if err != nil || u.IsAbs() || u.Host != "" ||
			(u.Path != "" && !strings.HasSuffix(u.Path, "/index.php") && u.Path != "index.php") {
			continue
		}
		q := u.Query()
		if q.Get("page") != "post" || q.Get("s") != "list" {
			continue
		}
		tagName := strings.ToLower(q.Get("tags"))
		if requested[tagName] {
			categories[tagName] = category
		}
	}
	return categories
}

// Do not send Rule34 API credentials to the public website. Constrain both
// request identity and host to the provider's own canonical public post URL.
func (c *Client) rule34PostSidebar(ctx context.Context, source Source, id string, requested map[string]bool) (map[string]string, error) {
	if !source.isRule34() || !postID.MatchString(id) {
		return nil, fmt.Errorf("invalid Rule34 post identity")
	}
	params := url.Values{"page": {"post"}, "s": {"view"}, "id": {id}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://rule34.xxx/index.php?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	data, err := c.fetchJSON(ctx, source, req, rule34SidebarMaxBytes)
	if err != nil {
		return nil, err
	}
	return parseRule34SidebarCategories(data, requested), nil
}

// Consult only a few representative posts and reuse matches across the whole
// loaded page. A website challenge, timeout or missing sidebar must never
// prevent the original API search results from rendering.
func (c *Client) rule34SidebarFallback(ctx context.Context, source Source, posts []Post, missing map[string]bool) map[string]string {
	categories := map[string]string{}
	attempted := map[string]bool{}
	for tries := 0; tries < rule34SidebarMaxPages && len(missing) > 0 && ctx.Err() == nil; tries++ {
		bestID, coverage := "", 0
		for _, post := range posts {
			if !postID.MatchString(post.ID) || attempted[post.ID] {
				continue
			}
			count := 0
			for _, tag := range post.Tags {
				if missing[strings.ToLower(tag.Name)] {
					count++
				}
			}
			if count > coverage {
				bestID, coverage = post.ID, count
			}
		}
		if bestID == "" {
			break
		}
		attempted[bestID] = true
		found, err := c.rule34PostSidebar(ctx, source, bestID, missing)
		if err != nil {
			break
		}
		for name, category := range found {
			categories[name] = category
			delete(missing, name)
		}
		// Site may have served a challenge page, a changed layout or no tags.
		// Do not bombard it with additional requests that cannot help.
		if len(found) == 0 {
			break
		}
	}
	return categories
}
