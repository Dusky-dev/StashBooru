package booru

import (
	"fmt"
	"strings"
	"unicode"
)

// blacklistKey normalizes case and booru-style spaces/underscores.
func blacklistKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(name, "_", " ")), "_"))
}

func validateBlacklistTags(tags []string) error {
	if len(tags) > 256 {
		return fmt.Errorf("tag blacklist supports at most 256 tags")
	}
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag == "" || len(tag) > 256 || strings.TrimSpace(tag) != tag ||
			strings.IndexFunc(tag, unicode.IsControl) >= 0 {
			return fmt.Errorf("blacklisted tags must be 1–256 UTF-8 bytes, trimmed, and without control characters")
		}
		key := blacklistKey(tag)
		if key == "" || seen[key] {
			return fmt.Errorf("blacklisted tags must be nonempty and unique (ignoring case and spaces/underscores)")
		}
		seen[key] = true
	}
	return nil
}

// AddBlacklistTag appends one tag without disturbing the source's existing
// settings. A repeat (including different case or spaces vs underscores) is
// harmless. The caller persists the updated source only when changed.
func (s *Source) AddBlacklistTag(tag string) (bool, error) {
	if err := validateBlacklistTags([]string{tag}); err != nil {
		return false, err
	}
	key := blacklistKey(tag)
	for _, current := range s.BlacklistTags {
		if blacklistKey(current) == key {
			return false, nil
		}
	}
	next := append(append([]string{}, s.BlacklistTags...), strings.ReplaceAll(tag, " ", "_"))
	if err := validateBlacklistTags(next); err != nil {
		return false, err
	}
	s.BlacklistTags = next
	return true, nil
}

// FilterBlacklisted omits posts with blacklisted tags, including their attachments.
// Provider pagination and totals stay unchanged even when all posts are hidden.
func (s Source) FilterBlacklisted(result Result) Result {
	if len(s.BlacklistTags) == 0 || len(result.Posts) == 0 {
		return result
	}
	blocked := make(map[string]bool, len(s.BlacklistTags))
	for _, tag := range s.BlacklistTags {
		blocked[blacklistKey(tag)] = true
	}
	kept := make([]Post, 0, len(result.Posts))
	for _, post := range result.Posts {
		if postHasBlacklistedTag(post, blocked) {
			result.BlacklistedCount++
		} else {
			kept = append(kept, post)
		}
	}
	result.Posts = kept
	return result
}

func postHasBlacklistedTag(post Post, blocked map[string]bool) bool {
	for _, tag := range post.Tags {
		if blocked[blacklistKey(tag.Name)] {
			return true
		}
	}
	for _, attachment := range post.Attachments {
		if postHasBlacklistedTag(attachment, blocked) {
			return true
		}
	}
	return false
}
