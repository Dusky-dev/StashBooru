package booru

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Creator struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Service    string `json:"service"`
	searchName string
}
type CreatorResult struct {
	Creators []Creator `json:"creators"`
	Page     int       `json:"page"`
	HasMore  bool      `json:"hasMore"`
}
type creatorEntry struct {
	items   []Creator
	expires time.Time
}

func ValidateCreatorQuery(query string, page int) bool {
	return len(strings.TrimSpace(query)) >= 2 && len(query) <= 128 && validTagText(query) && page >= 1 && page <= 1000
}

// Creator directories are public API indexes, not post-query autocomplete.
// Two bounded indexes are retained for ten minutes; identical concurrent loads
// share one request. The normal DNS, redirect and host-rate limits still apply.
func (c *Client) SearchCreators(ctx context.Context, source Source, query string, page int) (CreatorResult, error) {
	result := CreatorResult{Creators: []Creator{}, Page: page}
	if source.Validate() != nil || !ValidateCreatorQuery(query, page) {
		return result, &Error{Code: "invalid-query", Message: "Enter an artist name of 2–128 bytes and a page from 1–1000."}
	}
	if source.Engine != "pawchive" && source.Engine != "kemono" {
		return result, &Error{Code: "unsupported-query", Message: "Artist directory search is available for Pawchive and Kemono."}
	}
	items, err := c.creatorIndex(ctx, source)
	if err != nil {
		return result, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	skip := (page - 1) * 50
	for _, creator := range items {
		if ctx.Err() != nil {
			return result, requestError(ctx)
		}
		if !strings.Contains(creator.searchName, query) {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		if len(result.Creators) == 50 {
			result.HasMore = page < 1000
			break
		}
		result.Creators = append(result.Creators, creator)
	}
	return result, nil
}

func (c *Client) creatorIndex(ctx context.Context, source Source) ([]Creator, error) {
	key := source.Instance()
	for {
		if ctx.Err() != nil {
			return nil, requestError(ctx)
		}
		c.creatorMu.Lock()
		if entry, ok := c.creatorCache[key]; ok && time.Now().Before(entry.expires) {
			c.creatorMu.Unlock()
			return entry.items, nil
		}
		if pending := c.creatorPending[key]; pending != nil {
			c.creatorMu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return nil, requestError(ctx)
			}
		}
		if c.creatorPending == nil {
			c.creatorPending = map[string]chan struct{}{}
		}
		pending := make(chan struct{})
		c.creatorPending[key] = pending
		c.creatorMu.Unlock()

		items, err := c.fetchCreators(ctx, source)
		c.creatorMu.Lock()
		if err == nil {
			if c.creatorCache == nil {
				c.creatorCache = map[string]creatorEntry{}
			}
			if len(c.creatorCache) >= 2 {
				oldestKey := ""
				var oldest time.Time
				for id, entry := range c.creatorCache {
					if oldestKey == "" || entry.expires.Before(oldest) {
						oldestKey, oldest = id, entry.expires
					}
				}
				delete(c.creatorCache, oldestKey)
			}
			c.creatorCache[key] = creatorEntry{items: items, expires: time.Now().Add(10 * time.Minute)}
		}
		delete(c.creatorPending, key)
		close(pending)
		c.creatorMu.Unlock()
		return items, err
	}
}

func (c *Client) fetchCreators(ctx context.Context, source Source) ([]Creator, error) {
	endpoint := "/api/v1/creators"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(source.URL, "/")+endpoint, nil)
	if err != nil {
		return nil, extendedParseError()
	}
	req.Header.Set("Accept", "application/json")
	if source.Engine == "kemono" {
		req.Header.Set("Accept", "text/css")
	}
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	data, err := c.fetchJSON(ctx, source, req, 64<<20)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('[') {
		return nil, extendedParseError()
	}
	items := []Creator{}
	seen := map[string]bool{}
	for count := 0; decoder.More(); count++ {
		if ctx.Err() != nil {
			return nil, requestError(ctx)
		}
		if count >= 500000 {
			return nil, extendedParseError()
		}
		var row struct {
			ID      json.RawMessage `json:"id"`
			Name    string          `json:"name"`
			Service string          `json:"service"`
		}
		if decoder.Decode(&row) != nil {
			return nil, extendedParseError()
		}
		id := text(row.ID)
		if !safeProviderPart(id) || !safeProviderPart(row.Service) || !validTagText(id) || !validTagText(row.Service) || strings.Contains(id+row.Service, ":") || len(row.Service) > 64 || !validTagText(row.Name) || len(row.Name) > 200 {
			continue
		}
		key := row.Service + ":" + id
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, Creator{ID: id, Name: row.Name, Service: row.Service, searchName: strings.ToLower(row.Name)})
	}
	if _, err = decoder.Token(); err != nil {
		return nil, extendedParseError()
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, extendedParseError()
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].searchName != items[j].searchName {
			return items[i].searchName < items[j].searchName
		}
		if items[i].Service != items[j].Service {
			return items[i].Service < items[j].Service
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}
