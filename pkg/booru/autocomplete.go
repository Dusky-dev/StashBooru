package booru

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type autocompleteEntry struct {
	items   []Suggestion
	expires time.Time
}

func ValidatePrefix(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	return len(prefix) >= 2 && len(prefix) <= 80 && validTagText(prefix) && !strings.ContainsAny(prefix, "*?\\\"")
}

// Autocomplete queries only a configured source. Results and credentials are
// isolated in a bounded, short-lived cache; errors are never cached as success.
func (c *Client) Autocomplete(ctx context.Context, source Source, prefix string) ([]Suggestion, error) {
	if source.Validate() != nil || !ValidatePrefix(prefix) {
		return nil, &Error{Code: "invalid-query", Message: "Enter a tag prefix of 2–80 bytes without query operators."}
	}
	if err := ctx.Err(); err != nil {
		return nil, requestError(ctx)
	}
	if source.Engine == "pixiv" || source.Engine == "pawchive" || source.Engine == "kemono" || source.Engine == "shimmie" {
		return nil, &Error{Code: "autocomplete", Message: "Live completion unavailable for this engine; install an optional CSV database."}
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if source.Engine == "philomena" && len(prefix) < 3 {
		return []Suggestion{}, nil
	}
	sum := sha256.Sum256([]byte(source.Instance() + "\x00" + source.APIKey + "\x00" + source.Login + "\x00" + source.UserID + "\x00" + prefix))
	key := hex.EncodeToString(sum[:])
	c.autocompleteMu.Lock()
	entry, found := c.autocompleteCache[key]
	c.autocompleteMu.Unlock()
	if found && time.Now().Before(entry.expires) {
		return append([]Suggestion{}, entry.items...), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := source.autocompleteRequest(ctx, prefix)
	if err != nil {
		return nil, err
	}
	data, err := c.fetchJSON(ctx, source, req, 512<<10)
	if err != nil {
		return nil, err
	}
	items, err := parseAutocomplete(source.Engine, data)
	if err != nil {
		return nil, &Error{Code: "invalid-response", Message: "Provider returned unsupported autocomplete data."}
	}
	c.autocompleteMu.Lock()
	if c.autocompleteCache == nil {
		c.autocompleteCache = map[string]autocompleteEntry{}
	}
	if len(c.autocompleteCache) >= 512 {
		for old := range c.autocompleteCache {
			delete(c.autocompleteCache, old)
			break
		}
	}
	c.autocompleteCache[key] = autocompleteEntry{append([]Suggestion{}, items...), time.Now().Add(5 * time.Minute)}
	c.autocompleteMu.Unlock()
	return items, nil
}

func (s Source) autocompleteRequest(ctx context.Context, prefix string) (*http.Request, error) {
	base := strings.TrimRight(s.URL, "/")
	path := "/tags.json"
	values := url.Values{"search[name_matches]": {prefix + "*"}, "search[order]": {"count"}, "limit": {"20"}}
	switch s.Engine {
	case "danbooru":
		path = "/autocomplete.json"
		values = url.Values{"search[type]": {"tag_query"}, "search[query]": {prefix}, "limit": {"20"}}
	case "gelbooru":
		path = "/index.php"
		values = url.Values{"page": {"autocomplete2"}, "term": {prefix}, "type": {"tag_query"}, "limit": {"20"}}
		u, _ := url.Parse(s.URL)
		if u.Hostname() == "rule34.xxx" || u.Hostname() == "api.rule34.xxx" {
			base, path = "https://api.rule34.xxx", "/autocomplete.php"
			values = url.Values{"q": {prefix}}
		} else if s.APIKey != "" {
			values.Set("api_key", s.APIKey)
			values.Set("user_id", s.UserID)
		}
	case "moebooru":
		path = "/tag.json"
		values = url.Values{"name": {prefix + "*"}, "order": {"count"}, "limit": {"20"}}
	case "philomena":
		path = "/autocomplete/tags"
		values = url.Values{"term": {prefix}}
		if s.APIKey != "" {
			values.Set("key", s.APIKey)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid autocomplete request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	req.Header.Set("Referer", strings.TrimRight(s.URL, "/")+"/")
	if (s.Engine == "danbooru" || s.Engine == "e621") && s.APIKey != "" {
		req.SetBasicAuth(s.Login, s.APIKey)
	}
	return req, nil
}

func parseAutocomplete(engine string, data []byte) ([]Suggestion, error) {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(data, &rows) != nil || rows == nil {
		return nil, fmt.Errorf("invalid tags")
	}
	if len(rows) > 200 {
		return nil, fmt.Errorf("too many suggestions")
	}
	items := []Suggestion{}
	seen := map[string]bool{}
	for _, row := range rows {
		name := text(row["value"])
		if name == "" {
			name = text(row["name"])
		}
		if !validTagText(name) || (engine != "philomena" && strings.ContainsFunc(name, unicode.IsSpace)) || seen[name] {
			continue
		}
		category := text(row["category"])
		if category == "" {
			category = text(row["type"])
		}
		category = suggestionCategory(engine, category, name)
		count := number(row["post_count"])
		if count == 0 {
			count = number(row["count"])
		}
		if count == 0 {
			count = number(row["images"])
		}
		if (engine == "gelbooru" || engine == "philomena") && count == 0 {
			label := text(row["label"])
			if at := strings.LastIndex(label, " ("); at >= 0 {
				count, _ = strconv.ParseInt(strings.TrimSuffix(label[at+2:], ")"), 10, 64)
			}
		}
		if count < 0 {
			count = 0
		}
		seen[name] = true
		items = append(items, Suggestion{Name: name, Category: category, Count: count})
	}
	if len(rows) > 0 && len(items) == 0 {
		return nil, fmt.Errorf("no valid suggestions")
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Count > items[j].Count })
	if len(items) > 20 {
		items = items[:20]
	}
	return items, nil
}

func suggestionCategory(engine, category, name string) string {
	if engine == "philomena" {
		if strings.HasPrefix(name, "artist:") {
			return "artist"
		}
		if strings.HasPrefix(name, "series:") {
			return "copyright"
		}
		if category == "oc" {
			return "character"
		}
	}
	numeric := map[string]string{"0": "general", "1": "artist", "3": "copyright", "4": "character", "5": "meta"}
	if engine == "moebooru" {
		numeric["5"], numeric["6"] = "circle", "faults"
	}
	if engine == "e621" {
		numeric["2"], numeric["5"], numeric["6"], numeric["7"], numeric["8"] = "contributor", "species", "invalid", "meta", "lore"
	}
	if value, ok := numeric[category]; ok {
		return value
	}
	switch category {
	case "artist", "copyright", "character", "general", "meta", "species", "lore", "circle", "faults", "contributor", "invalid":
		return category
	}
	return "unknown"
}
