package booru

import (
	"strconv"
	"strings"
	"time"
)

// Post date formats seen in site APIs: Danbooru/e621 (RFC 3339 with
// fraction), Gelbooru/Rule34 (Ruby date), Pawchive/Kemono (no zone),
// Pixiv (RFC 3339) and Moebooru-style Unix seconds.
var postDateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	time.RubyDate,
	time.UnixDate,
	time.RFC1123Z,
	time.RFC1123,
	"2006-01-02",
}

// PostDate reads a post's upload date as the site wrote it (its own
// calendar day; no time-zone conversion). It returns false for empty,
// unparseable, pre-1990 or future values.
func PostDate(raw string, now time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 64 {
		return time.Time{}, false
	}
	var at time.Time
	parsed := false
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		at, parsed = time.Unix(seconds, 0).UTC(), true
	} else {
		for _, layout := range postDateLayouts {
			if value, err := time.Parse(layout, raw); err == nil {
				at, parsed = value, true
				break
			}
		}
	}
	if !parsed || at.Year() < 1990 || at.After(now.Add(24*time.Hour)) {
		return time.Time{}, false
	}
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), true
}
