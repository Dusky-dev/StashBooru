package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

type largeMediaPageClient struct{ booruBrowserClient }

func (largeMediaPageClient) Search(_ context.Context, source booru.Source, _ string, page, limit int) (booru.Result, error) {
	result := booru.Result{SourceID: source.ID, Page: page, Limit: limit, Posts: make([]booru.Post, 0, 15)}
	for i := 0; i < 15; i++ {
		post := booru.Post{
			ID: fmt.Sprint(i + 1), SourceID: source.ID,
			PreviewURL: fmt.Sprintf("https://img.pawchive.pw/thumbnail/data/%d.png", i+1),
		}
		for j := 0; j < 24; j++ {
			post.Attachments = append(post.Attachments, booru.Post{
				ID: fmt.Sprint(1000+i*24+j), SourceID: source.ID,
				PreviewURL: fmt.Sprintf("https://img.pawchive.pw/thumbnail/data/%d-%d.png", i, j),
			})
		}
		result.Posts = append(result.Posts, post)
	}
	return result, nil
}

func TestBooruSearchKeepsMediaHeavyPages(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: largeMediaPageClient{}}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/booru-browser/search?source=pawchive&q=meili&limit=15", nil))
	if w.Code != 200 {
		t.Fatalf("valid page with 375 media assets rejected: %d %s", w.Code, w.Body.String())
	}
	var result booru.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Posts) != 15 {
		t.Fatalf("expected 15 posts, got %d", len(result.Posts))
	}
	for i, post := range result.Posts {
		if len(post.Attachments) != 24 || len(post.MediaKey) != 48 {
			t.Fatalf("post %d lost attachments or its media key: %d / %q", i, len(post.Attachments), post.MediaKey)
		}
		for j, attachment := range post.Attachments {
			if len(attachment.MediaKey) != 48 || attachment.PreviewURL == "" {
				t.Fatalf("post %d attachment %d missing preview capability", i, j)
			}
		}
	}
}

func TestBooruLargePageMediaSnapshotRetention(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	var source booru.Source
	for _, preset := range booru.Defaults() {
		if preset.ID == "pawchive" {
			source = preset
			break
		}
	}
	if source.ID == "" {
		t.Fatal("missing Pawchive preset")
	}
	first := ""
	for i := 0; i < 4300; i++ {
		key := rememberBooruPost(source, booru.Post{
			ID: fmt.Sprint(i+1), SourceID: source.ID,
			PreviewURL: "https://img.pawchive.pw/thumbnail/data/example.png",
		})
		if key == "" {
			t.Fatal("media key generation failed")
		}
		if i == 0 {
			first = key
		}
	}
	_, post, ok := rs.mediaSnapshot(first)
	if !ok || post.ID != "1" {
		t.Fatal("first asset was prematurely evicted from a single large page")
	}
}
