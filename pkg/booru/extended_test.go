package booru

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestPawchiveRejectsUnfilteredTextSearch(t *testing.T) {
	s := testSource("pawchive")
	calls := 0
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Has("q") {
			t.Fatal("text query sent to an unfiltered endpoint")
		}
		want := "/api/v1/posts"
		if calls == 2 {
			want = "/api/v1/patreon/user/42"
		}
		if r.URL.Path != want {
			t.Fatal("incorrect browse endpoint", r.URL)
		}
		return response(200, "[]"), nil
	})
	for _, query := range []string{"cat", "a", "title words", "creator:patreon:42 extra"} {
		result, err := c.Search(context.Background(), s, query, 1, 24)
		var failure *Error
		if !errors.As(err, &failure) || (failure.Code != "unsupported-query" && failure.Code != "invalid-query") || len(result.Posts) != 0 || calls != 0 {
			t.Fatalf("unfiltered search accepted: %q %+v %v calls=%d", query, result, err, calls)
		}
	}
	for _, query := range []string{"*", "creator:patreon:42"} {
		if _, err := c.Search(context.Background(), s, query, 1, 24); err != nil {
			t.Fatal(query, err)
		}
	}
	if calls != 2 || !strings.Contains(s.Public().QueryHelp, "post search is unsupported") {
		t.Fatal("supported browsing or query guidance lost", calls, s.Public())
	}
}

func TestArchiveSourceKeepsZIPAndCBZ(t *testing.T) {
	for _, extension := range []string{"zip", "cbz"} {
		t.Run(extension, func(t *testing.T) {
			var raw object
			body := fmt.Sprintf(`{"id":"123","service":"patreon","user":"42","file":{"path":"/album.%s","name":"album.%s"},"attachments":[{"path":"/page.png"}]}`, extension, extension)
			if err := json.Unmarshal([]byte(body), &raw); err != nil {
				t.Fatal(err)
			}
			post, err := parseArchivePost(testSource("kemono"), raw)
			if err != nil || post.Format != extension || post.OriginalURL == "" ||
				post.PreviewURL != "" || post.SampleURL != "" || len(post.Attachments) != 1 ||
				post.Attachments[0].Format != "png" {
				t.Fatalf("ZIP/CBZ attachment unavailable: %+v %v", post, err)
			}
		})
	}
}

func TestExtendedFixedPageSlicingAndGalleryIdentity(t *testing.T) {
	for _, engine := range []string{"pixiv", "pawchive", "kemono"} {
		t.Run(engine, func(t *testing.T) {
			s := testSource(engine)
			block := 50
			if engine == "pixiv" {
				block = 30
				s.APIKey = "fixture-token"
			}
			var offsets []int
			query := "landscape"
			if engine == "pawchive" {
				query = "*"
			}
			c := testClient(func(r *http.Request) (*http.Response, error) {
				key := "o"
				if engine == "pixiv" {
					key = "offset"
					if r.Header.Get("Authorization") != "Bearer fixture-token" {
						t.Fatal("missing token")
					}
				}
				offset, _ := strconv.Atoi(r.URL.Query().Get(key))
				offsets = append(offsets, offset)
				if offset%block != 0 {
					t.Fatal("unaligned upstream page")
				}
				rows := []any{}
				for i := offset; i < offset+block && i < 80; i++ {
					if engine == "pixiv" {
						rows = append(rows, map[string]any{"id": i + 1, "tags": []any{}, "user": map[string]any{"id": 42, "name": "Artist"}, "meta_pages": []any{map[string]any{"image_urls": map[string]string{"original": "https://cdn.example.org/one.jpg"}}, map[string]any{"image_urls": map[string]string{"original": "https://cdn.example.org/two.jpg"}}}})
					} else {
						rows = append(rows, map[string]any{"id": fmt.Sprint(i + 1), "service": "patreon", "user": "42", "file": map[string]string{"path": "/one.jpg"}, "attachments": []any{map[string]string{"path": "/two.png"}}})
					}
				}
				var data []byte
				if engine == "pixiv" {
					data, _ = json.Marshal(map[string]any{"illusts": rows})
				} else {
					data, _ = json.Marshal(rows)
				}
				return response(200, string(data)), nil
			})
			got, err := c.Search(context.Background(), s, query, 2, 24)
			if err != nil || len(got.Posts) != 24 || got.Posts[0].RemoteID != "25" || got.Posts[23].RemoteID != "48" || !got.HasMore {
				t.Fatalf("wrong slice: %+v %v", got, err)
			}
			for _, p := range got.Posts {
				if len(p.Attachments) != 1 || p.ID == p.Attachments[0].ID || !ValidPostID(p.Attachments[0].ID) {
					t.Fatal("gallery identity lost", p)
				}
			}
			again, err := c.Search(context.Background(), s, query, 4, 24)
			if err != nil || len(again.Posts) != 8 || again.HasMore {
				t.Fatalf("bad last page: %+v %v", again, err)
			}
		})
	}
}
func TestShimmieAndExtendedFailures(t *testing.T) {
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/danbooru/find_posts" ||
			r.URL.Query().Get("page") != "2" || r.URL.Query().Get("limit") != "2" ||
			r.URL.Query().Get("tags") != "landscape" ||
			r.Header.Get("Accept") != "application/xml" {
			t.Fatal(r.URL)
		}
		return response(200, `<posts count="3"><post id="3" tags="landscape" file_url="https://cdn.example.org/one.jpg" preview_url="https://cdn.example.org/thumb.jpg" /></posts>`), nil
	})
	got, err := c.Search(context.Background(), testSource("shimmie"), "landscape", 2, 2)
	if err != nil || len(got.Posts) != 1 || got.HasMore || got.Posts[0].PostURL != "https://board.example.org/post/view/3" {
		t.Fatal(got, err)
	}
	if _, err = c.Search(context.Background(), testSource("pixiv"), "cat", 1, 24); err == nil {
		t.Fatal("missing token accepted")
	}
	for _, id := range []string{"0", "18446744073709551616", "../../bad", "p" + strings.Repeat("z", 32)} {
		if ValidPostID(id) {
			t.Fatal("unsafe identity", id)
		}
	}
	raw := object{}
	_ = json.Unmarshal([]byte(`{"id":1,"type":"ugoira","tags":[],"user":{},"meta_single_page":{"original_image_url":"https://cdn.example.org/animation.zip"}}`), &raw)
	post, err := parsePixivPost(testSource("pixiv"), raw)
	if err != nil || post.DownloadAvailable {
		t.Fatal("frame ZIP offered as media", err)
	}
}

// Paheal serves posts XML at /api/danbooru/find_posts; the legacy
// /index.xml path can be served as HTML without an HTTP error status.
func TestPahealFindPostsAndInvalidXML(t *testing.T) {
	s := Defaults()[10]
	if s.ID != "paheal" || s.Engine != "shimmie" {
		t.Fatal("Paheal preset was unexpectedly changed", s)
	}
	calls := 0
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "rule34.paheal.net" ||
			r.URL.Path != "/api/danbooru/find_posts" ||
			r.URL.Query().Get("tags") != "landscape" ||
			r.URL.Query().Get("page") != "1" ||
			r.URL.Query().Get("limit") != "2" ||
			r.Header.Get("Accept") != "application/xml" {
			t.Fatalf("wrong Paheal XML API request: %s", r.URL.String())
		}
		return response(200, `<?xml version="1.0" encoding="UTF-8"?>
<posts count="3" offset="0">
  <post id="101" md5="0123456789abcdef0123456789abcdef" file_name="sample.png" file_url="https://rule34.paheal.net/_images/a.png" preview_url="https://rule34.paheal.net/_thumbs/a.png" width="640" height="480" tags="landscape clouds" score="0" />
  <post id="100" file_url="https://rule34.paheal.net/_images/b.jpg" preview_url="https://rule34.paheal.net/_thumbs/b.jpg" tags="landscape" />
</posts>`), nil
	})
	got, err := c.Search(context.Background(), s, "landscape", 1, 2)
	if err != nil || calls != 1 || len(got.Posts) != 2 || !got.HasMore ||
		got.Total == nil || *got.Total != 3 {
		t.Fatalf("Paheal XML was not parsed: %#v %v (calls %d)", got, err, calls)
	}
	first := got.Posts[0]
	if first.PostURL != "https://rule34.paheal.net/post/view/101" ||
		first.OriginalURL != "https://rule34.paheal.net/_images/a.png" ||
		first.PreviewURL != "https://rule34.paheal.net/_thumbs/a.png" ||
		first.Format != "png" || first.Width != 640 || first.Height != 480 ||
		len(first.Tags) != 2 {
		t.Fatal("lost Paheal XML post metadata", first)
	}
	for _, body := range []string{
		`<!DOCTYPE html><html><body>API moved</body></html>`,
		`<posts count="-1"></posts>`,
		`<posts><post id="1"/><post id="2"/><post id="3"/></posts>`,
	} {
		client := testClient(func(*http.Request) (*http.Response, error) {
			return response(200, body), nil
		})
		_, err := client.Search(context.Background(), s, "landscape", 1, 2)
		var failure *Error
		if !errors.As(err, &failure) || failure.Code != "invalid-response" {
			t.Fatalf("invalid Paheal XML accepted: %q, error %v", body, err)
		}
	}
}

func TestExtendedMediaCredentialScope(t *testing.T) {
	for _, engine := range []string{"pawchive", "pixiv"} {
		s := testSource(engine)
		s.APIKey = "fixture-secret"
		s.MediaHosts = append(s.MediaHosts, "file.pawchive.pw")
		calls := 0
		c := testClient(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Authorization") != "" {
				t.Fatal("bearer token leaked to files")
			}
			wantCookie := engine == "pawchive" && calls == 1
			if (r.Header.Get("Cookie") != "") != wantCookie {
				t.Fatal("incorrect cookie scope", r.URL)
			}
			if engine == "pixiv" && r.Header.Get("Referer") != "https://www.pixiv.net/" {
				t.Fatal("Pixiv referer missing")
			}
			if calls == 1 {
				res := response(302, "")
				res.Header.Set("Location", "https://cdn.example.org/final.jpg")
				return res, nil
			}
			res := response(200, "\xff\xd8\xffabcdef")
			res.ContentLength = -1
			return res, nil
		})
		var out bytes.Buffer
		if _, err := c.FetchMedia(context.Background(), s, MediaRequest{URL: "https://file.pawchive.pw/one.jpg", Limit: 1024}, &out); err != nil {
			t.Fatal(err)
		}
	}
}
func TestFilenameTokensAndBoundaries(t *testing.T) {
	for _, format := range []string{"../%id%.%ext%", "/%id%.%ext%", "a//%id%.%ext%", "%unknown%.%ext%", "%artist:separator=x%.%ext%", "%id%.jpg", "C:/%id%.%ext%"} {
		if ValidateFilenameTemplate(format) == nil {
			t.Fatal("invalid format accepted", format)
		}
	}
	p := Post{ID: "123", RemoteID: "42", Tags: []Tag{{Name: "../artist/name", Category: "artist"}, {Name: "series", Category: "copyright"}}, Position: 2}
	got, err := RenderFilename("%artist%/%copyright%/%id%-%position%-%md5%.%ext%", testSource("danbooru"), p, "realhash", "jxl", 0)
	if err != nil || got != "_artist_name/series/42-2-realhash.jxl" {
		t.Fatal(got, err)
	}
	got, err = RenderFilename("%character%/%filename%.%ext%", testSource("danbooru"), Post{Filename: "CON.jpg"}, "hash", "png", 0)
	if err != nil || got != "unknown/_CON.png" {
		t.Fatal(got, err)
	}
}

func TestArchiveUnavailableOriginalAndNonMediaNotice(t *testing.T) {
	var raw object
	if err := json.Unmarshal([]byte(`{"id":"1","user":"2","service":"patreon","has_full":false,"file":{"path":"/one.jpg"},"attachments":[{"path":"/archive.zip"}]}`), &raw); err != nil {
		t.Fatal(err)
	}
	post, err := parseArchivePost(testSource("pawchive"), raw)
	if err != nil || post.DownloadAvailable || post.OriginalURL != "" || post.PreviewURL == "" ||
		len(post.Attachments) != 1 || post.Attachments[0].Format != "zip" ||
		post.Attachments[0].PreviewURL != "" || !strings.Contains(post.MediaNotice, "Full media is unavailable") {
		t.Fatal(post, err)
	}
}
