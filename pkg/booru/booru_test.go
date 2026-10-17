package booru

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testClient(fn roundTripFunc) *Client {
	c := NewClient()
	c.http.Transport, c.interval = fn, 0
	return c
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func testSource(engine string) Source {
	return Source{ID: "fixture", Name: "Fixture", Engine: engine, URL: "https://board.example.org", MediaHosts: []string{"cdn.example.org"}}
}

func TestFamiliesAndPagination(t *testing.T) {
	for _, test := range []struct{ engine, body, path, pageKey, pageValue string }{
		{"danbooru", `[{"id":123,"tag_string_artist":"artist_a","tag_string_copyright":"series_a","preview_file_url":"/preview.jpg"}]`, "/posts.json", "page", "2"},
		{"gelbooru", `{"post":[{"id":"123","tags":"landscape","preview_url":"//cdn.example.org/preview.jpg"}]}`, "/index.php", "pid", "1"},
		{"moebooru", `[{"id":123,"tags":"landscape","preview_url":"/preview.jpg"}]`, "/post.json", "page", "2"},
		{"e621", `{"posts":[{"id":123,"file":{"width":400,"height":300,"ext":"png"},"tags":{"artist":["artist_a"],"species":["cat"]},"preview":{"url":"/preview.jpg"}}]}`, "/posts.json", "page", "2"},
		{"philomena", `{"images":[{"id":123,"tags":["artist:example","safe"],"representations":{"thumb":"/preview.jpg"}}],"total":1}`, "/api/v1/json/search/images", "page", "2"},
	} {
		t.Run(test.engine, func(t *testing.T) {
			query := "landscape -bad_tag & % + \"猫\""
			client := testClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != test.path || r.URL.Query().Get(test.pageKey) != test.pageValue {
					t.Errorf("wrong paging request: %s", r.URL)
				}
				queryKey := "tags"
				if test.engine == "philomena" {
					queryKey = "q"
				}
				if r.URL.Query().Get(queryKey) != query {
					t.Error("query semantics changed")
				}
				return response(200, test.body), nil
			})
			got, err := client.Search(context.Background(), testSource(test.engine), query, 2, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Posts) != 1 || got.Posts[0].ID != "123" || got.Posts[0].SourceID != "fixture" || got.Posts[0].PreviewURL == "" {
				t.Fatalf("bad result: %#v", got)
			}
			if got.HasMore != (test.engine != "philomena") {
				t.Fatal("wrong next-page semantics")
			}
			if got.FetchedAt.IsZero() {
				t.Fatal("missing retrieval time")
			}
		})
	}
}

func TestCurrentE621AndMissingMetadata(t *testing.T) {
	got, err := parse(testSource("e621"), []byte(`{"posts":[{"id":9007199254740993,"files":{"original":{"width":640,"height":480},"meta":{"ext":"png","md5":"0123456789ABCDEF0123456789ABCDEF","size":1234},"preview":{"jpg":"/preview.jpg"},"sample":{"jpg":"/sample.jpg"}},"tags":{"artist":["a"],"copyright":["s"]}},{"id":2,"tags":{}}]}`), 1, 24)
	if err != nil {
		t.Fatal(err)
	}
	post := got.Posts[0]
	if post.ID != "9007199254740993" || post.Width != 640 || post.Bytes != 1234 || post.MD5 != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("lost exact identity/metadata: %#v", post)
	}
	if got.Posts[1].PreviewURL != "" || got.Posts[1].Width != 0 || got.HasMore {
		t.Fatal("missing metadata was invented")
	}
}

func TestInvalidResponsesAndUnknownCategories(t *testing.T) {
	unknown, err := parse(testSource("philomena"), []byte(`{"images":[],"total":null}`), 1, 24)
	if err != nil || unknown.Total != nil {
		t.Fatal("null provider total must stay unknown")
	}
	if _, err := parse(testSource("philomena"), []byte(`{"images":[],"total":"invalid"}`), 1, 24); err == nil {
		t.Fatal("invalid provider total accepted")
	}
	for _, body := range []string{`<html>login</html>`, `{}`, `null`, `{"post":null}`, `{"post":[{"id":0}]}`, `{"post":[{"id":1.5}]}`, `{"post":[{"id":"1/../../"}]}`, `{"post":[{"id":1,"tags":12}]}`, `{"success":false,"message":"secret"}`} {
		if _, err := parse(testSource("gelbooru"), []byte(body), 1, 24); err == nil {
			t.Errorf("accepted invalid response %s", body)
		}
	}
	got, err := parse(testSource("gelbooru"), []byte(`[{"id":1,"tags":"a a b"},{"id":1,"tags":"a"}]`), 1, 2)
	if err != nil || len(got.Posts) != 1 || len(got.Posts[0].Tags) != 2 || got.Posts[0].Tags[0].Category != "unknown" || !got.HasMore {
		t.Fatalf("bad duplicate/unknown-category behavior: %#v %v", got, err)
	}
	if _, err := parse(testSource("gelbooru"), []byte(`[{"id":1},{"id":2}]`), 1, 1); err == nil {
		t.Fatal("unbounded response accepted")
	}
}

func TestCredentialsAndErrorRedaction(t *testing.T) {
	for _, engine := range []string{"danbooru", "e621", "gelbooru", "philomena"} {
		t.Run(engine, func(t *testing.T) {
			source := testSource(engine)
			source.APIKey, source.Login, source.UserID = "private-secret", "owner", "42"
			req, err := source.request(context.Background(), "cat", 1, 24)
			if err != nil {
				t.Fatal(err)
			}
			switch engine {
			case "danbooru", "e621":
				login, key, ok := req.BasicAuth()
				if !ok || login != "owner" || key != source.APIKey || strings.Contains(req.URL.String(), source.APIKey) {
					t.Fatal("wrong basic auth")
				}
			case "gelbooru":
				if req.URL.Query().Get("api_key") != source.APIKey || req.URL.Query().Get("user_id") != "42" {
					t.Fatal("wrong DAPI auth")
				}
			case "philomena":
				if req.URL.Query().Get("key") != source.APIKey {
					t.Fatal("wrong Philomena auth")
				}
			}
			public, _ := json.Marshal(source.Public())
			if strings.Contains(string(public), source.APIKey) || strings.Contains(string(public), "owner") {
				t.Fatal("public source leaked credentials")
			}
			client := testClient(func(r *http.Request) (*http.Response, error) { return nil, errors.New("failure " + r.URL.String()) })
			_, err = client.Search(context.Background(), source, "cat", 1, 24)
			if err == nil || strings.Contains(err.Error(), source.APIKey) {
				t.Fatal("transport leaked credentials")
			}
		})
	}
}

func TestThrottlingAndCancellation(t *testing.T) {
	var requests atomic.Int32
	client := testClient(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		r := response(429, "secret")
		r.Header.Set("Retry-After", "60")
		return r, nil
	})
	for i := range 2 {
		source := testSource("danbooru")
		if i == 1 {
			source.URL = "https://BOARD.example.org."
		}
		_, err := client.Search(context.Background(), source, "cat", 1, 24)
		var failure *Error
		if !errors.As(err, &failure) || failure.Code != "rate-limited" || failure.RetryAfter < 59 {
			t.Fatalf("bad throttle: %v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("provider cooldown bypassed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Search(ctx, testSource("gelbooru"), "cat", 1, 24)
	if err == nil {
		t.Fatal("cancelled request succeeded")
	}
	client = testClient(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err = client.Search(ctx, testSource("danbooru"), "cat", 1, 24); err == nil {
		t.Fatal("deadline not enforced")
	}
}

func TestBoundedHTTPFailures(t *testing.T) {
	for _, status := range []int{301, 401, 403, 500} {
		client := testClient(func(_ *http.Request) (*http.Response, error) {
			r := response(status, "private-secret")
			r.Header.Set("Location", "https://127.0.0.1/secret")
			return r, nil
		})
		_, err := client.Search(context.Background(), testSource("danbooru"), "cat", 1, 24)
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("bad HTTP %d error: %v", status, err)
		}
	}
	client := testClient(func(_ *http.Request) (*http.Response, error) {
		return response(200, strings.Repeat("x", responseLimit+1)), nil
	})
	if _, err := client.Search(context.Background(), testSource("danbooru"), "cat", 1, 24); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestPublicNetworkBoundary(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.168.1.1", "::1", "::ffff:127.0.0.1", "fc00::1", "2001:db8::1", "198.18.0.1"} {
		if publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("accepted %s", address)
		}
	}
	called := false
	dial := func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, errors.New("fixture")
	}
	_, err := dialPublicAddresses(context.Background(), "tcp", "443", []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, dial)
	if err == nil || called {
		t.Fatal("mixed DNS result dialed a destination")
	}
	_, _ = dialPublicAddresses(context.Background(), "tcp", "443", []netip.Addr{netip.MustParseAddr("1.1.1.1")}, dial)
	if !called {
		t.Fatal("public IP was rejected")
	}
	source := testSource("danbooru")
	for _, raw := range []string{"http://board.example.org/a", "javascript:alert(1)", "https://evil.example.org/a", "https://cdn.example.org.evil.org/a", "https://cdn.example.org:999/a", "https://owner:secret@cdn.example.org/a", "https://cdn.example.org/a?api_key=secret"} {
		if source.mediaURL(raw) != "" {
			t.Errorf("unsafe preview accepted: %s", raw)
		}
	}
	if source.mediaURL("//img.cdn.example.org/a.jpg") == "" {
		t.Fatal("allowed CDN subdomain rejected")
	}
}

func TestSourceConfiguration(t *testing.T) {
	sources, err := Configure(nil)
	if err != nil || len(sources) != 10 {
		t.Fatalf("bad defaults: %v", err)
	}
	for _, data := range []string{`null`, `[] []`, `[{"id":"x","name":"x","engine":"unknown","url":"https://x.example.org"}]`, `[{"id":"x","disabled":true},{"id":"x","disabled":true}]`, `[{"id":"x","typo":"value"}]`} {
		if _, err := Configure([]byte(data)); err == nil {
			t.Errorf("invalid config accepted: %s", data)
		}
	}
	sources, err = Configure([]byte(`[{"id":"danbooru","disabled":true},{"id":"custom","name":"Custom","engine":"danbooru","url":"https://custom.example.org/sub","login":"me","apiKey":"secret"}]`))
	if err != nil || len(sources) != 10 || sources[len(sources)-1].ID != "custom" {
		t.Fatalf("bad overlay: %v", err)
	}
	for _, raw := range []string{"http://public.example.org", "https://localhost", "https://127.0.0.1", "https://public.example.org/?api_key=secret", "https://owner:secret@public.example.org", "https://public.example.org:8080"} {
		source := testSource("danbooru")
		source.URL = raw
		if source.Validate() == nil {
			t.Errorf("unsafe config accepted: %s", raw)
		}
	}
	for _, query := range []string{"", " ", strings.Repeat("x", 1025), "cat\nfoo"} {
		if ValidateQuery(query, 1, 24) == nil {
			t.Error("invalid query accepted")
		}
	}
	if ValidateQuery("cat", 0, 24) == nil || ValidateQuery("cat", 751, 24) == nil || ValidateQuery("cat", 1, 100) == nil {
		t.Fatal("query limits bypassed")
	}
}
