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
	rule34CategoryMaxBatches    = 5
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
		if xml.Unmarshal(data, &response) != nil || len(response.Tags) > rule34CategoryBatchSize {
			return result
		}
		for _, row := range response.Tags {
			add(row.Name, row.Type, row.Category)
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
	pending := make([]string, 0, rule34CategoryBatchSize*rule34CategoryMaxBatches)
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
				if len(pending) < rule34CategoryBatchSize*rule34CategoryMaxBatches {
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
	// Leave a few seconds for the sidebar fallback if the tag API itself
	// stalls. The overall search still retains its eight-second budget.
	tagCtx, tagCancel := context.WithTimeout(ctx, 3*time.Second)
	defer tagCancel()
	for start, batches := 0, 0; start < len(pending) && tagCtx.Err() == nil && batches < rule34CategoryMaxBatches; batches++ {
		end, nameBytes := start, 0
		for end < len(pending) && end-start < rule34CategoryBatchSize {
			length := len(pending[end])
			if end > start && nameBytes+length+1 > rule34CategoryNamesMaxBytes {
				break
			}
			nameBytes += length + 1
			end++
		}
		batch := pending[start:end]
		start = end
		names := make(map[string]bool, len(batch))
		for _, name := range batch {
			names[name] = true
		}
		params := url.Values{
			"page": {"dapi"}, "s": {"tag"}, "q": {"index"},
			"names": {strings.Join(batch, " ")}, "limit": {"100"},
			"api_key": {source.APIKey}, "user_id": {source.UserID},
		}
		req, err := http.NewRequestWithContext(tagCtx, http.MethodGet,
			source.rule34APIBase()+"/index.php?"+params.Encode(), nil)
		if err != nil {
			break
		}
		req.Header.Set("Accept", "application/xml")
		req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
		data, err := c.fetchJSON(tagCtx, source, req, 256<<10)
		if err != nil {
			break // Do not fail the post search on optional category enrichment.
		}
		found := parseRule34TagCategories(data, names)
		for name, category := range found {
			categories[name] = category
		}
		// Rule34's published tag API documents ID lookup, not bulk "names"
		// search. An empty/mismatched first response means more requests with
		// the same unsupported filter are unlikely to help; save time for the
		// public post-sidebar fallback instead.
		if len(found) == 0 {
			break
		}
	}
	missing := make(map[string]bool)
	for _, name := range pending {
		if categories[name] == "" {
			missing[name] = true
		}
	}
	for name, category := range c.rule34SidebarFallback(ctx, source, posts, missing) {
		categories[name] = category
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
