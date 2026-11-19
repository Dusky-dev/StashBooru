package booru

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Archive posts (Pawchive, Kemono) carry only service and creator ID. The
// creator's display name comes from the creator directory, when it is
// already loaded, or from the per-creator profile endpoint. Names are labels
// only: the creator:service:id key stays the identity.

type creatorNameEntry struct {
	name    string
	expires time.Time
}

const (
	creatorNameLimit = 20000
	creatorNameTTL   = 24 * time.Hour
	creatorMissTTL   = time.Hour
)

// ArchiveCreatorEngine reports whether a source's posts name creators only
// by creator:service:id.
func ArchiveCreatorEngine(engine string) bool {
	return engine == "pawchive" || engine == "kemono"
}

func creatorNameKey(source Source, service, id string) string {
	return source.Instance() + "\x00" + service + ":" + id
}

// CachedCreatorName returns a known display name without network access.
// The second result is false when the name is not known yet; a known miss
// returns "", true so callers do not ask again before it expires.
func (c *Client) CachedCreatorName(source Source, service, id string) (string, bool) {
	c.creatorMu.Lock()
	defer c.creatorMu.Unlock()
	now := time.Now()
	if entry, ok := c.creatorNames[creatorNameKey(source, service, id)]; ok && now.Before(entry.expires) {
		return entry.name, true
	}
	if index, ok := c.creatorCache[source.Instance()]; ok && now.Before(index.expires) {
		if name, ok := index.byKey[service+":"+id]; ok {
			return name, true
		}
	}
	return "", false
}

func (c *Client) rememberCreatorName(source Source, service, id, name string, ttl time.Duration) {
	c.creatorMu.Lock()
	defer c.creatorMu.Unlock()
	if c.creatorNames == nil {
		c.creatorNames = map[string]creatorNameEntry{}
	}
	if len(c.creatorNames) >= creatorNameLimit {
		now := time.Now()
		for key, entry := range c.creatorNames {
			if now.After(entry.expires) || len(c.creatorNames) >= creatorNameLimit {
				delete(c.creatorNames, key)
			}
			if len(c.creatorNames) < creatorNameLimit*3/4 {
				break
			}
		}
	}
	c.creatorNames[creatorNameKey(source, service, id)] = creatorNameEntry{name: name, expires: time.Now().Add(ttl)}
}

// LookupCreatorName asks the archive's profile endpoint for a creator's
// display name. An unknown creator is remembered as a miss for an hour.
func (c *Client) LookupCreatorName(ctx context.Context, source Source, service, id string) (string, error) {
	if !ArchiveCreatorEngine(source.Engine) || source.Validate() != nil || !safeProviderPart(service) || !safeProviderPart(id) {
		return "", &Error{Code: "invalid-query", Message: "Creator names are available for Pawchive and Kemono creators."}
	}
	if name, ok := c.CachedCreatorName(source, service, id); ok {
		return name, nil
	}
	endpoint := strings.TrimRight(source.URL, "/") + "/api/v1/" + url.PathEscape(service) + "/user/" + url.PathEscape(id) + "/profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", extendedParseError()
	}
	req.Header.Set("Accept", "application/json")
	if source.Engine == "kemono" {
		req.Header.Set("Accept", "text/css")
	}
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	data, err := c.fetchJSON(ctx, source, req, 64<<10)
	if err != nil {
		var failure *Error
		if errors.As(err, &failure) && failure.Code == "provider-error" && strings.Contains(failure.Message, "HTTP 404") {
			c.rememberCreatorName(source, service, id, "", creatorMissTTL)
		}
		return "", err
	}
	var profile struct {
		ID      json.RawMessage `json:"id"`
		Name    string          `json:"name"`
		Service string          `json:"service"`
	}
	if json.Unmarshal(data, &profile) != nil || text(profile.ID) != id || profile.Service != service {
		return "", extendedParseError()
	}
	name := strings.TrimSpace(profile.Name)
	if !validTagText(name) || len(name) > 200 {
		name = ""
	}
	c.rememberCreatorName(source, service, id, name, creatorNameTTL)
	return name, nil
}
