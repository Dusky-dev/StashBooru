package booru

import (
	"testing"
	"time"
)

// Formats follow each site's documented API output; the repository has no
// recorded responses with dates.
func TestPostDate(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for raw, want := range map[string]string{
		"2024-01-05T23:34:56.789-05:00":  "2024-01-05", // Danbooru: the site's own day
		"2024-01-05T12:34:56+09:00":      "2024-01-05", // Pixiv
		"Fri Jan 05 12:34:56 -0500 2024": "2024-01-05", // Gelbooru, Rule34
		"2024-01-05T12:34:56":            "2024-01-05", // Pawchive / Kemono
		"2024-01-05 12:34:56":            "2024-01-05",
		"1704458096":                     "2024-01-05", // Unix seconds
		"2024-01-05":                     "2024-01-05",
	} {
		got, ok := PostDate(raw, now)
		if !ok || got.Format("2006-01-02") != want {
			t.Errorf("PostDate(%q) = %v, %v; want %s", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "yesterday", "0", "1985-01-01", "2027-01-01T00:00:00Z", "2026-11-25"} {
		if got, ok := PostDate(raw, now); ok {
			t.Errorf("PostDate(%q) = %v; want rejected", raw, got)
		}
	}
}
