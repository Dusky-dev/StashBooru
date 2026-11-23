package booru

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func twitterSource() Source {
	return Source{ID: "twitter", Name: "Twitter / X", Engine: "twitter", URL: "https://api.x.com", MediaHosts: []string{"twimg.com"}, APIKey: "bearer"}
}

const twitterPage = `{"data":[
 {"id":"1800000000000000001","text":"two  pictures #fanart #oc","author_id":"42","created_at":"2026-11-01T10:00:00.000Z","possibly_sensitive":true,
  "public_metrics":{"like_count":7},"entities":{"hashtags":[{"start":13,"end":20,"tag":"fanart"},{"tag":"oc"}]},
  "attachments":{"media_keys":["3_1","3_2"]}},
 {"id":"1800000000000000002","text":"text only","author_id":"42"},
 {"id":"1800000000000000003","text":"a clip","author_id":"42","attachments":{"media_keys":["7_3"]}}],
 "includes":{"media":[
  {"media_key":"3_1","type":"photo","url":"https://pbs.twimg.com/media/AAA.jpg","width":1200,"height":800},
  {"media_key":"3_2","type":"photo","url":"https://pbs.twimg.com/media/BBB.png"},
  {"media_key":"7_3","type":"video","preview_image_url":"https://pbs.twimg.com/ext_tw_video_thumb/3/pu/img/C.jpg",
   "variants":[{"content_type":"application/x-mpegURL","url":"https://video.twimg.com/x.m3u8"},
    {"bit_rate":256000,"content_type":"video/mp4","url":"https://video.twimg.com/ext_tw_video/3/pu/vid/low.mp4"},
    {"bit_rate":2176000,"content_type":"video/mp4","url":"https://video.twimg.com/ext_tw_video/3/pu/vid/high.mp4?tag=12"}]}],
  "users":[{"id":"42","name":"Artist Name","username":"artist_42"}]},
 "meta":{"result_count":3,"next_token":"NEXT2"}}`

func TestTwitterTimelineAndCursors(t *testing.T) {
	var asked []*http.Request
	client := testClient(func(r *http.Request) (*http.Response, error) {
		asked = append(asked, r)
		switch {
		case r.URL.Path == "/2/users/by/username/artist_42":
			return response(200, `{"data":{"id":"42","name":"Artist Name","username":"artist_42"}}`), nil
		case r.URL.Path == "/2/users/42/tweets" && r.URL.Query().Get("pagination_token") == "":
			return response(200, twitterPage), nil
		case r.URL.Path == "/2/users/42/tweets" && r.URL.Query().Get("pagination_token") == "NEXT2":
			return response(200, `{"data":[],"meta":{"result_count":0}}`), nil
		}
		return response(404, `{}`), nil
	})
	s := twitterSource()
	result, err := client.Search(context.Background(), s, "@artist_42", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	last := asked[len(asked)-1]
	if last.Header.Get("Authorization") != "Bearer bearer" || last.URL.Query().Get("exclude") != "retweets,replies" || last.URL.Query().Get("max_results") != "20" || last.URL.Query().Get("post.fields") == "" {
		t.Fatalf("request %s %v", last.URL, last.Header)
	}
	if len(result.Posts) != 2 || !result.HasMore || !strings.Contains(result.SearchNotice, "user:42") {
		t.Fatalf("text-only posts are skipped; got %+v", result)
	}
	p := result.Posts[0]
	if p.PostURL != "https://x.com/artist_42/status/1800000000000000001" || p.Author != "@artist_42" || p.Rating != "explicit" || p.Score != 7 ||
		p.OriginalURL != "https://pbs.twimg.com/media/AAA?format=jpg&name=orig" || p.Format != "jpg" || !p.DownloadAvailable || p.Width != 1200 {
		t.Fatalf("photo post %+v", p)
	}
	if len(p.Tags) != 3 || p.Tags[0].Name != "user:42" || p.Tags[0].Category != "artist" || p.Tags[1].Name != "fanart" {
		t.Fatalf("tags %+v", p.Tags)
	}
	if len(p.Attachments) != 1 || p.Attachments[0].Format != "png" || p.Attachments[0].Position != 2 || !ValidPostID(p.Attachments[0].ID) {
		t.Fatalf("second photo %+v", p.Attachments)
	}
	v := result.Posts[1]
	if v.OriginalURL != "https://video.twimg.com/ext_tw_video/3/pu/vid/high.mp4?tag=12" || v.Format != "mp4" || v.PreviewURL == "" {
		t.Fatalf("video %+v", v)
	}
	// Page 2 uses the remembered cursor without re-reading page 1.
	before := len(asked)
	if _, err := client.Search(context.Background(), s, "user:42", 2, 20); err != nil {
		t.Fatal(err)
	}
	// user:42 and @artist_42 share the timeline endpoint but not the query
	// key, so page 1 is walked once first.
	if got := asked[len(asked)-1].URL.Query().Get("pagination_token"); got != "NEXT2" || len(asked)-before != 2 {
		t.Fatalf("cursor walk: %d requests, token %q", len(asked)-before, got)
	}
	before = len(asked)
	if _, err := client.Search(context.Background(), s, "user:42", 2, 20); err != nil || len(asked)-before != 1 {
		t.Fatalf("known cursor: %d requests %v", len(asked)-before, err)
	}
}

func TestTwitterSearchAndErrors(t *testing.T) {
	var fieldNames []string
	client := testClient(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if q.Has("post.fields") {
			fieldNames = append(fieldNames, "post.fields")
			return response(400, `{"title":"Invalid Request"}`), nil
		}
		fieldNames = append(fieldNames, "tweet.fields")
		if r.URL.Path == "/2/tweets/search/recent" {
			if q.Get("query") != "cat art has:media -is:retweet" || q.Get("max_results") != "10" {
				t.Errorf("search %v", q)
			}
			return response(200, `{"meta":{"result_count":0}}`), nil
		}
		if r.URL.Path == "/2/tweets/1800000000000000001" {
			return response(200, `{"data":{"id":"1800000000000000001","text":"x","author_id":"42","attachments":{"media_keys":["3_1"]}},"includes":{"media":[{"media_key":"3_1","type":"photo","url":"https://evil.example/a.jpg"}]}}`), nil
		}
		if r.URL.Path == "/2/users/42" {
			return response(200, `{"data":{"id":"42","profile_image_url":"https://pbs.twimg.com/profile_images/1/abc_normal.jpg"}}`), nil
		}
		if r.URL.Path == "/2/users/43" {
			return response(200, `{"data":{"id":"43","profile_image_url":"https://abs.twimg.com/sticky/default_profile_images/default_profile_normal.png"}}`), nil
		}
		return response(402, `{}`), nil
	})
	s := twitterSource()
	ctx := context.Background()
	result, err := client.Search(ctx, s, "cat art", 1, 5)
	if err != nil || len(result.Posts) != 0 || result.Limit != 10 || !strings.Contains(result.SearchNotice, "7 days") {
		t.Fatalf("search %+v %v", result, err)
	}
	if strings.Join(fieldNames, ",") != "post.fields,tweet.fields" {
		t.Fatalf("falls back to the other fields name once: %v", fieldNames)
	}
	fieldNames = nil
	if result, err = client.Search(ctx, s, "id:1800000000000000001", 1, 20); err != nil || len(result.Posts) != 0 || fieldNames[0] != "tweet.fields" {
		t.Fatalf("media off X's hosts is dropped and the accepted name is kept: %+v %v %v", result, err, fieldNames)
	}
	if _, err = client.Search(ctx, s, "user:99", 1, 20); err == nil || !strings.Contains(err.Error(), "paid access") {
		t.Fatalf("402 explains X API access: %v", err)
	}
	for _, query := range []string{"", "*", "user:abc", "id:x", "@bad-handle!"} {
		if _, err := client.Search(ctx, s, query, 1, 20); err == nil {
			t.Fatalf("%q must be refused", query)
		}
	}
	if _, err := client.Search(ctx, s, "user:42", 9, 20); err == nil || !strings.Contains(err.Error(), "earlier pages") {
		t.Fatalf("far pages need earlier cursors: %v", err)
	}
	if got, err := client.CreatorIcon(ctx, s, "user:42"); err != nil || got != "https://pbs.twimg.com/profile_images/1/abc_400x400.jpg" {
		t.Fatalf("icon %q %v", got, err)
	}
	if _, err := client.CreatorIcon(ctx, s, "user:43"); !errors.Is(err, ErrNoCreatorPicture) {
		t.Fatalf("default avatar: %v", err)
	}
	s.APIKey = ""
	if _, err := client.Search(ctx, s, "@artist", 1, 20); err == nil || !strings.Contains(err.Error(), "bearer token") {
		t.Fatalf("token required: %v", err)
	}
	if err := (Source{ID: "t", Name: "T", Engine: "twitter", URL: "https://api.x.com", Login: "me"}).Validate(); err == nil {
		t.Fatal("no login field")
	}
}
