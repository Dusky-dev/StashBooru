package booru

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Fixture content identity.
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMediaBoundaryAndChecksums(t *testing.T) {
	data := "\xff\xd8\xff" + strings.Repeat("x", 700)
	checksum := fmt.Sprintf("%x", md5.Sum([]byte(data))) //nolint:gosec // Test checksum.
	source := testSource("danbooru")
	source.Login, source.APIKey = "owner", "secret"
	var gotAuth, gotRef string
	c := testClient(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotRef = r.Header.Get("Referer")
		res := response(200, data)
		res.ContentLength = int64(len(data))
		return res, nil
	})
	for _, tc := range []struct {
		url  string
		auth bool
	}{{"https://cdn.example.org/a.jpg", false}, {"https://board.example.org/a.jpg", true}} {
		var out bytes.Buffer
		info, err := c.FetchMedia(context.Background(), source, MediaRequest{URL: tc.url, Limit: 1024, ExpectedMD5: checksum, ExpectedBytes: int64(len(data))}, &out)
		if err != nil || out.String() != data || info.Extension != "jpg" || info.MD5 != checksum || (gotAuth != "") != tc.auth || gotRef != "https://board.example.org/" {
			t.Fatalf("media headers/identity: %+v %v", info, err)
		}
	}
	for _, input := range []MediaRequest{
		{URL: "https://evil.example.org/a", Limit: 1024},
		{URL: "http://cdn.example.org/a", Limit: 1024},
		{URL: "https://cdn.example.org/a?api_key=secret", Limit: 1024},
		{URL: "https://cdn.example.org/a", Limit: 700},
		{URL: "https://cdn.example.org/a", Limit: 1024, ExpectedBytes: 700},
		{URL: "https://cdn.example.org/a", Limit: 1024, ExpectedMD5: strings.Repeat("0", 32)},
	} {
		var out bytes.Buffer
		if _, err := c.FetchMedia(context.Background(), source, input, &out); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	for _, body := range []string{"<html>login secret</html>", "<svg xmlns='http://www.w3.org/2000/svg'/>", "not a media file"} {
		c := testClient(func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
		var out bytes.Buffer
		if _, err := c.FetchMedia(context.Background(), source, MediaRequest{URL: "https://cdn.example.org/a", Limit: 1024}, &out); err == nil || out.Len() != 0 {
			t.Fatal("accepted active/invalid response")
		}
	}
}
func TestMediaRedirectsAndCancellation(t *testing.T) {
	source := testSource("danbooru")
	source.Login, source.APIKey = "owner", "secret"
	var calls int
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			res := response(302, "")
			res.Header.Set("Location", "https://cdn.example.org/final.jpg")
			return res, nil
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("credentials reached CDN")
		}
		res := response(200, "\xff\xd8\xffabcdef")
		res.ContentLength = -1
		return res, nil
	})
	var out bytes.Buffer
	if _, err := c.FetchMedia(context.Background(), source, MediaRequest{URL: "https://board.example.org/a", Limit: 1024}, &out); err != nil || calls != 2 {
		t.Fatal(err, calls)
	}
	calls = 0
	c = testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		res := response(302, "")
		res.Header.Set("Location", "http://127.0.0.1/private?secret=1")
		return res, nil
	})
	if _, err := c.FetchMedia(context.Background(), source, MediaRequest{URL: "https://board.example.org/a", Limit: 1024}, &out); err == nil || calls != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatal("unsafe redirect or error disclosure", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.FetchMedia(ctx, source, MediaRequest{URL: "https://board.example.org/a", Limit: 1024}, &out); err == nil {
		t.Fatal("cancelled media succeeded")
	}
}
func TestOriginalURLsAndModernDanbooruVariants(t *testing.T) {
	source := testSource("danbooru")
	result, err := parse(source, []byte(`[{"id":1,"image_width":900,"image_height":1200,"media_asset":{"variants":[{"type":"180x180","url":"https://cdn.example.org/preview.jpg"},{"type":"sample","url":"https://cdn.example.org/sample.jpg"},{"type":"original","url":"https://cdn.example.org/original.jpg"}]}}]`), 1, 24)
	if err != nil || result.Posts[0].Width != 900 || !result.Posts[0].DownloadAvailable || !strings.HasSuffix(result.Posts[0].PreviewURL, "preview.jpg") {
		t.Fatalf("variants %+v %v", result, err)
	}
	result, err = parse(source, []byte(`[{"id":1,"file_url":"https://cdn.example.org/original.jpg"}]`), 1, 24)
	if err != nil || result.Posts[0].PreviewURL != "" || result.Posts[0].OriginalURL == "" {
		t.Fatal("original must not become thumbnail", err)
	}
}
