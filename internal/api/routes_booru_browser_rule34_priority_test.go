package api

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

// A CSV entry used to mark the tag as known before this live categorizer
// ran, so a newly retyped tag was never checked against the live source.
type rule34CategorySpyClient struct {
	calls            int
	sawUnknownBefore bool
	sawPriority      bool
	liveCategories   map[string]string
}

func (c *rule34CategorySpyClient) Search(context.Context, booru.Source, string, int, int) (booru.Result, error) {
	return booru.Result{}, nil
}

func (c *rule34CategorySpyClient) FetchMedia(context.Context, booru.Source, booru.MediaRequest, io.Writer) (booru.MediaInfo, error) {
	return booru.MediaInfo{}, nil
}

func (c *rule34CategorySpyClient) CategorizeRule34Tags(_ context.Context, source booru.Source, posts []booru.Post) {
	c.calls++
	if !source.IsRule34() {
		return
	}
	for i := range posts {
		for j := range posts[i].Tags {
			tag := &posts[i].Tags[j]
			if tag.Category != "unknown" {
				continue
			}
			c.sawUnknownBefore = true
			if category := c.liveCategories[tag.Name]; category != "" {
				tag.Category = category
			}
		}
	}
}

func (c *rule34CategorySpyClient) CategorizeRule34TagsPrioritized(ctx context.Context, source booru.Source, posts []booru.Post, missingFromCSV func(string) bool) {
	c.sawPriority = missingFromCSV("live_only") &&
		!missingFromCSV("changed_role") && !missingFromCSV("csv_only_character")
	c.CategorizeRule34Tags(ctx, source, posts)
}

func TestRule34LiveCategoriesTakePrecedenceOverInstalledCSV(t *testing.T) {
	dir := t.TempDir()
	var source booru.Source
	for _, candidate := range booru.Defaults() {
		if candidate.ID == "rule34" {
			source = candidate
			break
		}
	}
	if source.ID == "" {
		t.Fatal("Rule34 default source not found")
	}
	tagDir := filepath.Join(dir, "booru-tags")
	if err := os.MkdirAll(tagDir, 0700); err != nil {
		t.Fatal(err)
	}
	csv := "type,count,name,ambiguous,id\n" +
		"1,40,changed_role,False,1\n" +
		"4,30,csv_only_character,False,2\n" +
		"0,20,provider_typed,False,3\n"
	if err := os.WriteFile(filepath.Join(tagDir, source.Instance()+".csv"), []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	client := &rule34CategorySpyClient{liveCategories: map[string]string{
		"changed_role": "copyright",
		"live_only":    "artist",
	}}
	rs := booruBrowserRoutes{configPath: dir, client: client}
	posts := []booru.Post{{Tags: []booru.Tag{
		{Name: "changed_role", Category: "unknown"},
		{Name: "csv_only_character", Category: "unknown"},
		{Name: "live_only", Category: "unknown"},
		{Name: "provider_typed", Category: "meta"},
	}}}
	if status := rs.categorizeBooruPosts(context.Background(), source, posts); status != "" {
		t.Fatalf("fully categorized posts should not install automatic catalog: %q", status)
	}
	if client.calls != 1 || !client.sawUnknownBefore || !client.sawPriority {
		t.Fatalf("CSV suppressed or incorrectly prioritized live categorizer: calls=%d sawUnknown=%t priority=%t",
			client.calls, client.sawUnknownBefore, client.sawPriority)
	}
	for i, want := range []string{"copyright", "character", "artist", "meta"} {
		if got := posts[0].Tags[i].Category; got != want {
			t.Errorf("tag %s: category=%q, want %q", posts[0].Tags[i].Name, got, want)
		}
	}

	// An already installed database must not prevent a fresh live attempt
	// on a later search, including when the same CSV covers every tag.
	again := []booru.Post{{Tags: []booru.Tag{
		{Name: "changed_role", Category: "unknown"},
		{Name: "csv_only_character", Category: "unknown"},
	}}}
	rs.categorizeBooruPosts(context.Background(), source, again)
	if client.calls != 2 || again[0].Tags[0].Category != "copyright" {
		t.Fatalf("live refresh did not run after the initial cached CSV: calls=%d posts=%+v", client.calls, again)
	}
}

func TestOtherBoorusDoNotInvokeRule34LiveCategorizer(t *testing.T) {
	client := &rule34CategorySpyClient{liveCategories: map[string]string{"hero": "artist"}}
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: client}
	source := booru.Defaults()[0]
	posts := []booru.Post{{Tags: []booru.Tag{{Name: "hero", Category: "unknown"}}}}
	rs.categorizeBooruPosts(context.Background(), source, posts)
	if client.calls != 0 || posts[0].Tags[0].Category != "unknown" {
		t.Fatalf("Rule34 lookup leaked into %s: calls=%d posts=%+v", source.ID, client.calls, posts)
	}
}
