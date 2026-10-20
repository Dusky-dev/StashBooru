package booru

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Rule34's post DAPI supplies flat tag strings even though its tag DAPI
// publishes category/type metadata. Resolve only exact provider tag names;
// do not guess categories from suffixes or apply categories from other sites.
const (
	rule34CategoryBatchSize     = 100
	rule34CategoryNamesMaxBytes = 3072
	rule34CategoryMaxExactLookups = 6
	rule34CategoryMaxCached     = 8192
	rule34CategoryCacheExpiry   = 24 * time.Hour
)

type rule34CategoryEntry struct {
	category string
	expires  time.Time
}

// The site's tag DAPI normally returns XML even when the post DAPI supports
// json=1. Keep JSON support for compatible responses, but never infer a tag's
// category from its spelling or from unrelated API results.
type rule34TagXMLRow struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Category string `xml:"category,attr"`
}

type rule34TagXMLResponse struct {
	XMLName xml.Name          `xml:"tags"`
	Tags    []rule34TagXMLRow `xml:"tag"`
}

func parseRule34TagCategories(data []byte, requested map[string]bool) map[string]string {
	result := make(map[string]string)
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return result
	}
	add := func(name, rawType, rawCategory string) {
		key := strings.ToLower(name)
		if !requested[key] || !validTagText(name) || strings.ContainsAny(name, " \t\r\n") {
			return
		}
		if rawType == "" {
			rawType = rawCategory
		}
		if category := suggestionCategory("gelbooru", rawType, name); category != "unknown" {
			result[key] = category
		}
	}
	if data[0] == '<' {
		// Never accept DTDs or declarations in untrusted provider documents.
		if bytes.Contains(bytes.ToLower(data), []byte("<!doctype")) {
			return result
		}
		var response rule34TagXMLResponse
		if xml.Unmarshal(data, &response) == nil && len(response.Tags) <= rule34CategoryBatchSize {
			for _, row := range response.Tags {
				add(row.Name, row.Type, row.Category)
			}
			return result
		}
		// Singular `name` lookups can respond with a single <tag /> root.
		var row rule34TagXMLRow
		if xml.Unmarshal(data, &row) == nil {
			var root struct{ XMLName xml.Name }
			if xml.Unmarshal(data, &root) == nil && root.XMLName.Local == "tag" {
				add(row.Name, row.Type, row.Category)
			}
		}
		return result
	}

	var rows []object
	if data[0] == '[' {
		if json.Unmarshal(data, &rows) != nil {
			return result
		}
	} else {
		var envelope object
		if json.Unmarshal(data, &envelope) != nil ||
			json.Unmarshal(envelope["tag"], &rows) != nil {
			return result
		}
	}
	if len(rows) > rule34CategoryBatchSize {
		return result
	}
	for _, row := range rows {
		add(text(row["name"]), text(row["type"]), text(row["category"]))
	}
	return result
}

// CategorizeRule34Tags best-effort enriches post tags from the documented
// Gelbooru-compatible tag DAPI. Searches and local CSV classification remain
// usable when the provider refuses tag lookups or the network is unavailable.
func (c *Client) CategorizeRule34Tags(ctx context.Context, source Source, posts []Post) {
	if !source.isRule34() || ctx.Err() != nil || len(posts) == 0 ||
		source.APIKey == "" || source.UserID == "" {
		return
	}
	// Category discovery is optional: never hold a post search indefinitely
	// when a tag API becomes slow or rate-limited.
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	instance := source.Instance() + ":"
	pending := make([]string, 0, rule34CategoryBatchSize)
	seen := make(map[string]bool)
	now := time.Now()
	c.rule34CategoryMu.Lock()
	if c.rule34CategoryCache == nil {
		c.rule34CategoryCache = make(map[string]rule34CategoryEntry)
	}
	for i := range posts {
		for j := range posts[i].Tags {
			tag := &posts[i].Tags[j]
			if tag.Category != "unknown" {
				continue // Never override typed provider tags or installed CSV.
			}
			name := strings.ToLower(tag.Name)
			if cached, ok := c.rule34CategoryCache[instance+name]; ok && now.Before(cached.expires) {
				tag.Category = cached.category
			} else if !seen[name] && validTagText(tag.Name) && !strings.ContainsAny(name, " \t\r\n") {
				seen[name] = true
				if len(pending) < rule34CategoryBatchSize {
					pending = append(pending, name)
				}
			}
		}
	}
	c.rule34CategoryMu.Unlock()
	if len(pending) == 0 {
		return
	}

	categories := make(map[string]string)
	missing := make(map[string]bool, len(pending))
	for _, name := range pending {
		missing[name] = true
	}
	// The website's sidebar may provide every category for a post with one
	// request. Try it before the per-name API to minimize provider traffic.
	for name, category := range c.rule34SidebarFallback(ctx, source, posts, missing) {
		categories[name] = category
	}
	// Rule34's tag DAPI does not document Gelbooru's bulk "names" filter.
	// The exact "name" parameter works on the Rule34 tag endpoint; resolve
	// remaining tags individually and cache confirmed categories across pages.
	// One-host throttling and the deadline bound these optional requests.
	attempts := 0
	for _, name := range pending {
		if !missing[name] || ctx.Err() != nil || attempts >= rule34CategoryMaxExactLookups {
			continue
		}
		attempts++
		params := url.Values{
			"page": {"dapi"}, "s": {"tag"}, "q": {"index"},
			"name": {name}, "limit": {"1"},
			"api_key": {source.APIKey}, "user_id": {source.UserID},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			source.rule34APIBase()+"/index.php?"+params.Encode(), nil)
		if err != nil {
			break
		}
		req.Header.Set("Accept", "application/xml")
		req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
		data, err := c.fetchJSON(ctx, source, req, 64<<10)
		if err != nil {
			break // An unavailable provider must never fail the post search.
		}
		if category := parseRule34TagCategories(data, map[string]bool{name: true})[name]; category != "" {
			categories[name] = category
			delete(missing, name)
		}
	}
	if len(categories) == 0 {
		return
	}
	c.rule34CategoryMu.Lock()
	if len(c.rule34CategoryCache)+len(categories) > rule34CategoryMaxCached {
		clear(c.rule34CategoryCache)
	}
	expiry := time.Now().Add(rule34CategoryCacheExpiry)
	for name, category := range categories {
		c.rule34CategoryCache[instance+name] = rule34CategoryEntry{category: category, expires: expiry}
	}
	c.rule34CategoryMu.Unlock()
	for i := range posts {
		for j := range posts[i].Tags {
			tag := &posts[i].Tags[j]
			if tag.Category == "unknown" {
				if category, ok := categories[strings.ToLower(tag.Name)]; ok {
					tag.Category = category
				}
			}
		}
	}
}
