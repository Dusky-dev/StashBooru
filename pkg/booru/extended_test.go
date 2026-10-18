package booru

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

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
			got, err := c.Search(context.Background(), s, "landscape", 2, 24)
			if err != nil || len(got.Posts) != 24 || got.Posts[0].RemoteID != "25" || got.Posts[23].RemoteID != "48" || !got.HasMore {
				t.Fatalf("wrong slice: %+v %v", got, err)
			}
			for _, p := range got.Posts {
				if len(p.Attachments) != 1 || p.ID == p.Attachments[0].ID || !ValidPostID(p.Attachments[0].ID) {
					t.Fatal("gallery identity lost", p)
				}
			}
			again, err := c.Search(context.Background(), s, "landscape", 4, 24)
			if err != nil || len(again.Posts) != 8 || again.HasMore {
				t.Fatalf("bad last page: %+v %v", again, err)
			}
		})
	}
}
func TestShimmieAndExtendedFailures(t *testing.T) {
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/danbooru/find_posts/index.xml" || r.URL.Query().Get("page") != "2" || r.Header.Get("Accept") != "application/xml" {
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
	if err != nil || post.DownloadAvailable || post.OriginalURL != "" || post.PreviewURL == "" || !strings.Contains(post.MediaNotice, "1 non-media") {
		t.Fatal(post, err)
	}
}
