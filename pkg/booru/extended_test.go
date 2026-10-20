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

func TestPawchiveFullArchiveKeywordSearch(t *testing.T) {
	s := testSource("pawchive")
	offsets := []int{}
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/posts" || r.URL.Query().Get("q") != "meili" ||
			r.Header.Get("Accept") != "application/json" ||
			r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatalf("Pawchive search must use public provider q= endpoint: %s", r.URL)
		}
		offset, err := strconv.Atoi(r.URL.Query().Get("o"))
		if err != nil || offset%50 != 0 {
			t.Fatalf("invalid Pawchive offset: %s", r.URL)
		}
		offsets = append(offsets, offset)
		rows := []map[string]any{}
		for i := offset; i < offset+50 && i < 122; i++ {
			rows = append(rows, map[string]any{
				"id": strconv.Itoa(i + 1), "user": "42", "service": "patreon",
				"title": fmt.Sprintf("Meili archived work %d", i),
			})
		}
		data, _ := json.Marshal(rows)
		return response(http.StatusOK, string(data)), nil
	})
	got, err := c.Search(context.Background(), s, "meili", 3, 24)
	if err != nil || len(got.Posts) != 24 || got.Posts[0].RemoteID != "49" ||
		got.Posts[23].RemoteID != "72" || !got.HasMore || got.SearchNotice != "" {
		t.Fatalf("full-history search returned wrong results: %+v %v", got, err)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 50 {
		t.Fatalf("provider q offset paging lost: %v", offsets)
	}
	got, err = c.Search(context.Background(), s, "meili", 6, 24)
	if err != nil || len(got.Posts) != 2 || got.Posts[0].RemoteID != "121" || got.HasMore {
		t.Fatalf("last archive search page was wrong: %+v %v", got, err)
	}
}

func TestPawchiveSingleRequestWhenTextIsNotIndexedOnPost(t *testing.T) {
	s := testSource("pawchive")
	var urls []string
	c := testClient(func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.RequestURI())
		if r.URL.Path != "/api/v1/posts" || r.URL.Query().Get("o") != "0" ||
			r.URL.Query().Get("q") != "meili" {
			t.Fatalf("wrong Pawchive keyword request: %s", r.URL)
		}
		return response(http.StatusOK, `[
			{"id":"301","user":"42","service":"patreon","title":"Old creator work"},
			{"id":"302","user":"42","service":"patreon","title":"Another archived post"}
		]`), nil
	})
	result, err := c.Search(context.Background(), s, "meili", 1, 24)
	if err != nil || len(result.Posts) != 2 || len(urls) != 1 ||
		!strings.Contains(result.SearchNotice, "may ignore search filters") {
		t.Fatalf("Pawchive added a duplicate provider probe or hid the ambiguity: %+v %v urls=%v", result, err, urls)
	}
}

func TestPawchivePreservesSearchOnOtherIndexedFields(t *testing.T) {
	s := testSource("pawchive")
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Has("q") {
			return response(http.StatusOK, `[{"id":"887","user":"42","service":"patreon","title":"Artist work"}]`), nil
		}
		return response(http.StatusOK, `[{"id":"886","user":"42","service":"patreon","title":"Recent post"}]`), nil
	})
	got, err := c.Search(context.Background(), s, "meili", 1, 24)
	if err != nil || len(got.Posts) != 1 || got.Posts[0].RemoteID != "887" {
		t.Fatalf("matching provider results from non-title index wrongly rejected: %+v %v", got, err)
	}
}

func TestPawchiveRecentAndCreatorModesUnaffected(t *testing.T) {
	s := testSource("pawchive")
	var paths []string
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Has("q") {
			t.Fatalf("recent/creator browsing unexpectedly sent a query: %s", r.URL)
		}
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/api/v1/patreon/user/42" {
			return response(http.StatusOK, `[{"id":"42","user":"42","service":"patreon","title":"Archived creator work"}]`), nil
		}
		return response(http.StatusOK, "[]"), nil
	})
	for _, query := range []string{"*", "creator:patreon:42"} {
		if _, err := c.Search(context.Background(), s, query, 1, 24); err != nil {
			t.Fatal(query, err)
		}
	}
	if len(paths) != 2 || paths[0] != "/api/v1/posts" ||
		paths[1] != "/api/v1/patreon/user/42" ||
		!strings.Contains(s.Public().QueryHelp, "post text") {
		t.Fatalf("recent browsing, creator search or help regressed: %v", paths)
	}
	for _, query := range []string{"creator:patreon:42 extra", "creator:patreon", "creator:patreon:../../etc"} {
		if _, err := c.Search(context.Background(), s, query, 1, 24); err == nil || len(paths) != 2 {
			t.Fatalf("bad creator query reached provider: %q, err %v", query, err)
		}
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
		first.OriginalURL != "https://r34i.paheal-cdn.net/01/23/0123456789abcdef0123456789abcdef" ||
		first.PreviewURL != "https://rule34.paheal.net/_thumbs/a.png" ||
		first.SampleURL != first.OriginalURL || !first.DownloadAvailable ||
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

// The Paheal preset and previously saved overrides must both trust the
// exact media CDN while keeping arbitrary offsite hosts rejected.
func TestPahealOriginalCDNSafetyAndSavedSettings(t *testing.T) {
	s := Defaults()[10]
	if s.ID != "paheal" || s.Validate() != nil {
		t.Fatal("Paheal default is unavailable")
	}
	const cdn = "https://r34i.paheal-cdn.net/01/23/0123456789abcdef0123456789abcdef"
	if s.mediaURL(cdn) != cdn {
		t.Fatal("Paheal CDN original rejected")
	}
	// Settings created before this patch keep a copied MediaHosts list.
	s.MediaHosts = []string{"paheal.net", "r34i.paheal-cdn.net"}
	if s.mediaURL(cdn) != cdn {
		t.Fatal("Paheal CDN rejected by legacy source configuration")
	}
	for _, raw := range []string{
		"https://r34i.paheal-cdn.net.evil.example/01/23/a",
		"https://sub.r34i.paheal-cdn.net/01/23/a",
		"https://different.paheal-cdn.net/01/23/a",
		"http://r34i.paheal-cdn.net/01/23/a",
	} {
		if s.mediaURL(raw) != "" {
			t.Fatalf("unexpected Paheal media approval: %s", raw)
		}
	}
	other := testSource("shimmie")
	if other.mediaURL(cdn) != "" {
		t.Fatal("unrelated Shimmie site implicitly trusted Paheal CDN")
	}
}

func TestPahealOriginalURLRewriteOnlyForOfficialMedia(t *testing.T) {
	s := Defaults()[10]
	const hash = "0123456789abcdef0123456789abcdef"
	const shard = "https://r34i.paheal-cdn.net/01/23/0123456789abcdef0123456789abcdef"
	for _, tc := range []struct{ input, md5, want string }{
		{"https://rule34.paheal.net/_images/a.png", hash, shard},
		{"http://rule34.paheal.net/_images/a.png", hash, shard},
		{"/_images/a.png", hash, shard},
		{"https://rule34.paheal.net/_images/" + hash + ".jpg", "", shard},
		{"https://rule34.paheal.net/_images/" + strings.ToUpper(hash) + ".JPG", "", shard},
		{"https://r34i.paheal-cdn.net/01/23/" + hash, hash, shard},
		{"https://rule34.paheal.net/_images/a.png", "", "https://rule34.paheal.net/_images/a.png"},
		{"https://rule34.paheal.net/_thumbs/a.png", hash, "https://rule34.paheal.net/_thumbs/a.png"},
		{"https://other.example/_images/a.png", hash, "https://other.example/_images/a.png"},
		{"https://rule34.paheal.net/post/view/123", hash, "https://rule34.paheal.net/post/view/123"},
		{"https://evil.example/_images/a.png?api_key=secret", hash, "https://evil.example/_images/a.png?api_key=secret"},
	} {
		if got := pahealOriginalMediaURL(s, tc.input, tc.md5); got != tc.want {
			t.Errorf("rewrite %q (md5 %q): got %q want %q", tc.input, tc.md5, got, tc.want)
		}
	}
	const unrelated = "https://rule34.paheal.net/_images/a.jpg"
	if got := pahealOriginalMediaURL(testSource("shimmie"), unrelated, hash); got != unrelated {
		t.Fatalf("Paheal rewrite applied to unrelated Shimmie source: %q", got)
	}
}

func TestPahealCDNOriginalCanBeDownloaded(t *testing.T) {
	s := Defaults()[10]
	s.MediaHosts = []string{"paheal.net"} // persisted from an older installation
	body := "\xff\xd8\xff" + "fixture-jpeg-data"
	client := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "r34i.paheal-cdn.net" ||
			r.URL.Path != "/01/23/0123456789abcdef0123456789abcdef" ||
			r.Header.Get("Referer") != "https://rule34.paheal.net/" {
			t.Errorf("download request targeted incorrect media host or referer: %s", r.URL)
		}
		res := response(200, body)
		res.ContentLength = -1
		return res, nil
	})
	var dest bytes.Buffer
	info, err := client.FetchMedia(context.Background(), s, MediaRequest{
		URL: "https://r34i.paheal-cdn.net/01/23/0123456789abcdef0123456789abcdef", Limit: 1024,
	}, &dest)
	if err != nil || info.Extension != "jpg" || dest.String() != body {
		t.Fatalf("Paheal original failed full download: %v (%+v)", err, info)
	}
}

func TestPahealSampleRetainsExplicitProviderSample(t *testing.T) {
	s := Defaults()[10]
	c := testClient(func(*http.Request) (*http.Response, error) {
		return response(200, `<posts count="1"><post id="12"
			md5="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			file_name="12.png" file_url="https://rule34.paheal.net/_images/12.png"
			preview_url="https://rule34.paheal.net/_thumbs/12.png"
			sample_url="https://rule34.paheal.net/_samples/12.png" /></posts>`), nil
	})
	result, err := c.Search(context.Background(), s, "", 1, 10)
	if err != nil || len(result.Posts) != 1 {
		t.Fatalf("search failed: %v", err)
	}
	p := result.Posts[0]
	if p.SampleURL != "https://rule34.paheal.net/_samples/12.png" ||
		p.OriginalURL != "https://r34i.paheal-cdn.net/aa/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		!p.DownloadAvailable {
		t.Fatalf("explicit provider sample lost: %+v", p)
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
