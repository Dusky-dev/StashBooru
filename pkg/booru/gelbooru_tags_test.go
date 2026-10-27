package booru

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type booruCategoryTransport struct {
	requests int
	names    string
}

func (r *booruCategoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.requests++
	r.names = req.URL.Query().Get("names")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"tag":[{"name":"saber","type":4},{"name":"artist_a","type":1},{"name":"fate_stay_night","type":3},{"name":"blue_eyes","type":0}]}`)),
	}, nil
}

func TestGelbooruDownloadCategoriesLookupExactAndCached(t *testing.T) {
	client := NewClient()
	transport := &booruCategoryTransport{}
	client.http.Transport = transport
	client.interval = 0
	source := Source{ID: "gelbooru", Name: "Gelbooru", Engine: "gelbooru", URL: "https://gelbooru.com"}
	makePosts := func() []Post {
		return []Post{{Tags: []Tag{
			{Name: "saber", Category: "unknown"},
			{Name: "artist_a", Category: "unknown"},
			{Name: "fate_stay_night", Category: "unknown"},
			{Name: "blue_eyes", Category: "unknown"},
		}}}
	}
	posts := makePosts()
	client.CategorizeGelbooruTagsPrioritized(context.Background(), source, posts, nil)
	want := []string{"character", "artist", "copyright", "general"}
	for i, expected := range want {
		if posts[0].Tags[i].Category != expected {
			t.Fatalf("tag %q: got category %q; want %q", posts[0].Tags[i].Name, posts[0].Tags[i].Category, expected)
		}
	}
	if transport.requests != 1 || !strings.Contains(transport.names, "fate_stay_night") {
		t.Fatalf("expected one exact names lookup, got requests=%d names=%q", transport.requests, transport.names)
	}

	// Repeat search with new post objects: saved validated categories must
	// be reused without generating another provider API request.
	posts = makePosts()
	client.CategorizeGelbooruTagsPrioritized(context.Background(), source, posts, nil)
	if transport.requests != 1 || posts[0].Tags[0].Category != "character" {
		t.Fatalf("failed to use cached categories: requests=%d tag=%+v", transport.requests, posts[0].Tags[0])
	}
}
