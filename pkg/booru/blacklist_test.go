package booru

import (
	"strings"
	"testing"
)

func TestSourceBlacklistValidation(t *testing.T) {
	valid := testSource("danbooru")
	valid.BlacklistTags = []string{"artist_a", "Series Name", "日本語"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid blacklist: %v", err)
	}
	cases := [][]string{
		{"a", "A"},
		{"artist_name", "artist name"},
		{""},
		{" trailing "},
		{"line\nfeed"},
		{"control\x00tag"},
		{strings.Repeat("x", 257)},
	}
	for _, tags := range cases {
		source := valid
		source.BlacklistTags = tags
		if err := source.Validate(); err == nil {
			t.Errorf("accepted invalid blacklist: %#v", tags)
		}
	}
	valid.BlacklistTags = make([]string, 257)
	for i := range valid.BlacklistTags {
		valid.BlacklistTags[i] = "tag"
	}
	if err := valid.Validate(); err == nil {
		t.Fatal("accepted more than 256 blacklist entries")
	}
}

func TestSourceBlacklistFiltersTagsAndAttachments(t *testing.T) {
	s := testSource("danbooru")
	s.BlacklistTags = []string{"bad tag", "hidden_artist"}
	total := int64(100)
	result := Result{
		Posts: []Post{
			{ID: "1", Tags: []Tag{{Name: "GOOD"}}},
			{ID: "2", Tags: []Tag{{Name: "Bad_Tag"}}},
			{ID: "3", Attachments: []Post{{Tags: []Tag{{Name: "HIDDEN artist"}}}}},
			{ID: "4", Tags: []Tag{{Name: "bad_tag_extended"}}},
		},
		Page: 3, Limit: 4, Total: &total, HasMore: true,
	}
	filtered := s.FilterBlacklisted(result)
	if len(filtered.Posts) != 2 || filtered.Posts[0].ID != "1" || filtered.Posts[1].ID != "4" || filtered.BlacklistedCount != 2 {
		t.Fatalf("wrong filtering: %+v", filtered)
	}
	if filtered.Page != 3 || filtered.Limit != 4 || filtered.Total != &total || !filtered.HasMore {
		t.Fatal("filtering altered provider paging")
	}
	other := testSource("gelbooru")
	if got := other.FilterBlacklisted(result); len(got.Posts) != 4 || got.BlacklistedCount != 0 {
		t.Fatal("another source inherited the blacklist")
	}
}
