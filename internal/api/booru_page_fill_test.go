package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func fillFixture(page, limit int) booru.Result {
	posts := make([]booru.Post, 0, limit)
	start := (page - 1) * limit
	for i := start; i < start+limit && i < 400; i++ {
		tags := []booru.Tag{{Name: "safe_tag"}}
		if i%5 == 0 {
			tags = []booru.Tag{{Name: "hidden_tag"}}
		} else if i%7 == 0 {
			tags = []booru.Tag{{Name: "ai_generated"}}
		}
		format := "png"
		if i%3 == 0 {
			format = "webm"
		}
		posts = append(posts, booru.Post{
			ID: strconv.Itoa(i + 1), Format: format, Tags: tags,
		})
	}
	return booru.Result{Page: page, Limit: limit, Posts: posts, HasMore: page*limit < 400}
}

func TestFilledBooruPagesDoNotRepeatBorrowedPosts(t *testing.T) {
	source := booru.Defaults()[0]
	source.BlacklistTags = []string{"hidden_tag"}
	calls := 0
	load := func(_ context.Context, page, limit int) (booru.Result, error) {
		calls++
		if limit != booru.MaxLimit {
			t.Errorf("provider received oversized limit %d", limit)
		}
		return fillFixture(page, limit), nil
	}
	filters := booruPageFilters{hideBlocked: true, media: "images", hideAI: true}
	first, err := fillBooruPage(context.Background(), source, 1, 60, filters, load)
	if err != nil || len(first.Posts) != 60 || !first.HasMore || first.BlacklistedCount == 0 {
		t.Fatalf("first filled page: %+v %v", first, err)
	}
	second, err := fillBooruPage(context.Background(), source, 2, 60, filters, load)
	if err != nil || len(second.Posts) != 60 || !second.HasMore {
		t.Fatalf("second filled page: %+v %v", second, err)
	}
	expected := make([]string, 0, 120)
	for i := 0; i < 400 && len(expected) < 120; i++ {
		if i%5 != 0 && i%7 != 0 && i%3 != 0 {
			expected = append(expected, strconv.Itoa(i+1))
		}
	}
	got := make([]string, 0, 120)
	for _, post := range append(first.Posts, second.Posts...) {
		got = append(got, post.ID)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("filled pages are not consecutive, unique visible posts: got %v, want %v", got, expected)
	}
	if calls < 4 {
		t.Fatal("filtered page was not filled from subsequent provider pages")
	}
}

func TestFilledBooruPageSize250AndDirectJump(t *testing.T) {
	source := booru.Defaults()[0]
	var pages []int
	load := func(_ context.Context, page, limit int) (booru.Result, error) {
		pages = append(pages, page)
		return fillFixture(page, limit), nil
	}
	first, err := fillBooruPage(context.Background(), source, 1, 250, booruPageFilters{}, load)
	if err != nil || len(first.Posts) != 250 || first.Posts[0].ID != "1" || first.Posts[249].ID != "250" {
		t.Fatalf("250-item page: %+v %v", first, err)
	}
	if !reflect.DeepEqual(pages, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("requested incorrect provider batches: %v", pages)
	}
	pages = nil
	next, err := fillBooruPage(context.Background(), source, 2, 120, booruPageFilters{}, load)
	if err != nil || len(next.Posts) != 120 || next.Posts[0].ID != "121" {
		t.Fatalf("unfiltered direct jump: %+v %v", next, err)
	}
	if !reflect.DeepEqual(pages, []int{3, 4, 5}) {
		t.Fatalf("jump did not use direct provider offset: %v", pages)
	}
}

func TestFilledBooruExhaustedAndValidation(t *testing.T) {
	source := booru.Defaults()[0]
	load := func(_ context.Context, page, limit int) (booru.Result, error) {
		return fillFixture(page, limit), nil
	}
	last, err := fillBooruPage(context.Background(), source, 2, 250, booruPageFilters{}, load)
	if err != nil || len(last.Posts) != 150 || last.HasMore {
		t.Fatalf("last page must be partial with no Next: %+v %v", last, err)
	}
	for _, size := range []int{0, 251} {
		if _, err := fillBooruPage(context.Background(), source, 1, size, booruPageFilters{}, load); err == nil {
			t.Fatalf("accepted invalid display size %d", size)
		}
	}
}

type filledRouteClient struct{ booruBrowserClient }

func (filledRouteClient) Search(_ context.Context, source booru.Source, _ string, page, limit int) (booru.Result, error) {
	out := fillFixture(page, limit)
	out.SourceID = source.ID
	return out, nil
}

func TestBooruFilledRouteAndFilters(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: filledRouteClient{}}
	for _, invalid := range []string{"&limit=251", "&media=unknown", "&ai=yes", "&fill=2", "&fill=1&fill=1"} {
		w := httptest.NewRecorder()
		rs.Search(w, httptest.NewRequest("GET", "/?source=danbooru&q=cat&page=1&limit=60&fill=1"+invalid, nil))
		if w.Code != 400 {
			t.Fatalf("invalid filled request accepted (%s): %d", invalid, w.Code)
		}
	}
	w := httptest.NewRecorder()
	rs.Search(w, httptest.NewRequest("GET", "/?source=danbooru&q=cat&page=1&limit=60&fill=1&media=images&ai=hide", nil))
	if w.Code != 200 {
		t.Fatalf("filled route: %d %s", w.Code, w.Body.String())
	}
	var result booru.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Posts) != 60 || !result.HasMore {
		t.Fatalf("route did not fill filtered posts: %+v", result)
	}
	for _, post := range result.Posts {
		if post.Format != "png" || strings.Contains(strings.ToLower(post.Tags[0].Name), "ai_generated") {
			t.Fatalf("hidden media/AI post leaked: %+v", post)
		}
	}
}
