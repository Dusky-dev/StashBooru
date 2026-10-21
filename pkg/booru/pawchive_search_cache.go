package booru

import (
	"context"
	"net/url"
	"time"
)

// Search-only cache: Pawchive can rate limit repeated reads of the same
// public posts page. Never cache errors or account/creator responses, and
// never retry a 429 automatically.
const (
	pawchiveSearchTTL           = 90 * time.Second
	pawchiveSearchMaxEntries    = 32
	pawchiveSearchMaxEntryBytes = 1 << 20
)

type pawchiveSearchCacheEntry struct {
	body    []byte
	expires time.Time
}

func (c *Client) pawchiveCachedSearch(ctx context.Context, source Source, params url.Values) ([]byte, error) {
	// Include the source URL and instance, so custom Pawchive mirrors do
	// not share their result sets even when queries and offsets match.
	key := source.Instance() + "|" + source.URL + "|" + params.Encode()
	for {
		if ctx.Err() != nil {
			return nil, requestError(ctx)
		}
		c.pawchiveSearchMu.Lock()
		if entry, ok := c.pawchiveSearchCache[key]; ok {
			if time.Now().Before(entry.expires) {
				body := append([]byte(nil), entry.body...)
				c.pawchiveSearchMu.Unlock()
				return body, nil
			}
			delete(c.pawchiveSearchCache, key)
		}
		if pending, ok := c.pawchiveSearchBusy[key]; ok {
			c.pawchiveSearchMu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return nil, requestError(ctx)
			}
		}
		if c.pawchiveSearchBusy == nil {
			c.pawchiveSearchBusy = make(map[string]chan struct{})
		}
		pending := make(chan struct{})
		c.pawchiveSearchBusy[key] = pending
		c.pawchiveSearchMu.Unlock()

		data, err := c.extendedRequestUncached(ctx, source, "/api/v1/posts", params)

		c.pawchiveSearchMu.Lock()
		if err == nil && len(data) <= pawchiveSearchMaxEntryBytes {
			if c.pawchiveSearchCache == nil || len(c.pawchiveSearchCache) >= pawchiveSearchMaxEntries {
				c.pawchiveSearchCache = make(map[string]pawchiveSearchCacheEntry)
			}
			c.pawchiveSearchCache[key] = pawchiveSearchCacheEntry{
				body:    append([]byte(nil), data...),
				expires: time.Now().Add(pawchiveSearchTTL),
			}
		}
		delete(c.pawchiveSearchBusy, key)
		close(pending)
		c.pawchiveSearchMu.Unlock()
		return data, err
	}
}
