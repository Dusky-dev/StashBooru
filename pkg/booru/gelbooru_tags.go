package booru

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Gelbooru-family post APIs commonly return one flat tag string. When tag
// import is enabled, resolve unknown categories with one bounded, exact-name
// tag-DAPI request; this must never make searching depend on tag API uptime.
// Rule34 has its own classifier because that site does not support this bulk
// "names" query reliably.
func (c *Client) CategorizeGelbooruTagsPrioritized(ctx context.Context, source Source, posts []Post, missingFromCSV func(string) bool) {
	if source.Engine != "gelbooru" || source.isRule34() || len(posts) == 0 || ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	instance := source.Instance() + ":"
	const batchSize = 100
	var pending, csvCovered []string
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
				continue
			}
			name := strings.ToLower(tag.Name)
			if entry, ok := c.rule34CategoryCache[instance+name]; ok && now.Before(entry.expires) {
				tag.Category = entry.category
				continue
			}
			if seen[name] || !validTagText(tag.Name) || strings.ContainsAny(name, " \t\r\n") {
				continue
			}
			seen[name] = true
			if missingFromCSV != nil && !missingFromCSV(name) {
				if len(csvCovered) < batchSize {
					csvCovered = append(csvCovered, name)
				}
			} else if len(pending) < batchSize {
				pending = append(pending, name)
			}
		}
	}
	c.rule34CategoryMu.Unlock()

	if len(pending) < batchSize {
		pending = append(pending, csvCovered[:min(batchSize-len(pending), len(csvCovered))]...)
	}
	if len(pending) == 0 {
		return
	}
	requested := make(map[string]bool, len(pending))
	for _, name := range pending {
		requested[name] = true
	}
	params := url.Values{
		"page": {"dapi"}, "s": {"tag"}, "q": {"index"},
		"json": {"1"}, "limit": {"100"},
		"names": {strings.Join(pending, " ")},
	}
	if source.APIKey != "" && source.UserID != "" {
		params.Set("api_key", source.APIKey)
		params.Set("user_id", source.UserID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(source.URL, "/")+"/index.php?"+params.Encode(), nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json, application/xml")
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	data, err := c.fetchJSON(ctx, source, req, 256<<10)
	if err != nil {
		return
	}
	categories := parseRule34TagCategories(data, requested)
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
				if category := categories[strings.ToLower(tag.Name)]; category != "" {
					tag.Category = category
				}
			}
		}
	}
}
