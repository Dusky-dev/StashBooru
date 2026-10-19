package booru

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Rule34's post DAPI supplies flat tag strings even though its tag DAPI
// publishes category/type metadata. Resolve only exact provider tag names;
// do not guess categories from suffixes or apply categories from other sites.
const (
	rule34CategoryBatchSize   = 100
	rule34CategoryMaxBatches  = 5
	rule34CategoryMaxCached   = 8192
	rule34CategoryCacheExpiry = 24 * time.Hour
)

type rule34CategoryEntry struct {
	category string
	expires  time.Time
}

func parseRule34TagCategories(data []byte, requested map[string]bool) map[string]string {
	result := make(map[string]string)
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
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
		name := text(row["name"])
		key := strings.ToLower(name)
		if !requested[key] || !validTagText(name) || strings.ContainsAny(name, " \t\r\n") {
			continue
		}
		rawCategory := text(row["type"])
		if rawCategory == "" {
			rawCategory = text(row["category"])
		}
		category := suggestionCategory("gelbooru", rawCategory, name)
		if category == "unknown" {
			continue
		}
		result[key] = category
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
	for start := 0; start < len(pending) && ctx.Err() == nil; start += rule34CategoryBatchSize {
		end := min(start+rule34CategoryBatchSize, len(pending))
		batch := pending[start:end]
		names := make(map[string]bool, len(batch))
		for _, name := range batch {
			names[name] = true
		}
		params := url.Values{
			"page": {"dapi"}, "s": {"tag"}, "q": {"index"}, "json": {"1"},
			"names": {strings.Join(batch, " ")}, "limit": {"100"},
			"api_key": {source.APIKey}, "user_id": {source.UserID},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			source.rule34APIBase()+"/index.php?"+params.Encode(), nil)
		if err != nil {
			break
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
		data, err := c.fetchJSON(ctx, source, req, 256<<10)
		if err != nil {
			break // Do not fail the post search on optional category enrichment.
		}
		for name, category := range parseRule34TagCategories(data, names) {
			categories[name] = category
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
