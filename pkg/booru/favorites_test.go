package booru

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAccountFavoritesNativeRequests(t *testing.T) {
	for _, engine := range []string{"danbooru", "e621", "gelbooru"} {
		t.Run(engine, func(t *testing.T) {
			s := testSource(engine)
			s.URL += "/custom"
			s.APIKey = "secret-key"
			if engine == "gelbooru" {
				s.UserID = "123"
			} else {
				s.Login = "artist_user"
			}
			calls := 0
			c := testClient(func(r *http.Request) (*http.Response, error) {
				calls++
				q := r.URL.Query()
				if r.Method != http.MethodGet {
					t.Fatal("remote favorite mutation")
				}
				if engine == "gelbooru" {
					if q.Get("api_key") != s.APIKey || q.Get("user_id") != "123" {
						t.Fatal("missing DAPI account")
					}
					if calls == 1 {
						if q.Get("s") != "favorite" || q.Get("id") != "123" || q.Get("pid") != "1" || q.Get("limit") != "2" {
							t.Fatal("wrong favorite page", q)
						}
						return response(200, `{"@attributes":{"count":5},"favorite":[{"id":10,"favorite":42},{"id":11,"favorite":43}]}`), nil
					}
					if q.Get("s") != "post" || q.Get("id") != fmt.Sprint(40+calls) || q.Has("tags") {
						t.Fatal("wrong favorite detail", q)
					}
					return response(200, fmt.Sprintf(`{"post":[{"id":%s,"file_url":"https://cdn.example.org/file.webm?cache=1"}]}`, q.Get("id"))), nil
				}
				login, key, _ := r.BasicAuth()
				if login != s.Login || key != s.APIKey || q.Get("page") != "2" || q.Get("limit") != "2" {
					t.Fatal("lost account/page")
				}
				if engine == "danbooru" {
					if r.URL.Path != "/custom/posts.json" || q.Get("tags") != "ordfav:artist_user" {
						t.Fatal("wrong ordered favorites request")
					}
					return response(200, `[{"id":42}]`), nil
				}
				if r.URL.Path != "/custom/favorites.json" || q.Has("tags") || q.Get("v2") != "true" {
					t.Fatal("wrong e621 request")
				}
				return response(200, `{"posts":[{"id":42}]}`), nil
			})
			got, err := c.AccountFavorites(context.Background(), s, 2, 2)
			if err != nil || len(got.Posts) == 0 || got.Posts[0].ID != "42" || got.FetchedAt.IsZero() {
				t.Fatal(got, err)
			}
			if engine == "gelbooru" && (calls != 3 || got.Posts[0].Format != "webm" || got.Posts[1].ID != "43" || !got.HasMore || *got.Total != 5) {
				t.Fatal("lost favorite order/metadata", got)
			}
		})
	}
}
func TestAccountFavoritesValidationAndFailures(t *testing.T) {
	for _, engine := range []string{"danbooru", "gelbooru", "e621", "moebooru", "pixiv", "kemono"} {
		c := testClient(func(*http.Request) (*http.Response, error) {
			t.Fatal("unconfigured favorites reached network")
			return nil, nil
		})
		if _, err := c.AccountFavorites(context.Background(), testSource(engine), 1, 24); err == nil {
			t.Fatal("unconfigured account accepted", engine)
		}
	}
	s := testSource("gelbooru")
	s.UserID, s.APIKey = "123", "secret-key"
	for _, body := range []string{`{}`, `{"favorite":null}`, `{"favorite":[{"favorite":"invalid"}]}`, `{"favorite":[{"favorite":42},{"favorite":43}]}`, `{"@attributes":{"count":-1},"favorite":[]}`} {
		c := testClient(func(*http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, err := c.AccountFavorites(context.Background(), s, 1, 1); err == nil {
			t.Fatal("invalid favorite accepted", body)
		}
	}
	for _, body := range []string{`{"@attributes":{"count":0}}`, `{"favorite":[]}`} {
		c := testClient(func(*http.Request) (*http.Response, error) { return response(200, body), nil })
		if got, err := c.AccountFavorites(context.Background(), s, 1, 24); err != nil || len(got.Posts) != 0 || got.HasMore {
			t.Fatal(got, err)
		}
	}
	c := testClient(func(*http.Request) (*http.Response, error) { return response(429, "secret-key"), nil })
	if _, err := c.AccountFavorites(context.Background(), s, 1, 24); err == nil || strings.Contains(err.Error(), s.APIKey) {
		t.Fatal("provider error leaked account", err)
	}
}
func TestEmptySearchesAndMissingMediaExtensions(t *testing.T) {
	for _, query := range []string{"", " "} {
		if ValidateQuery(query, 1, 24) != nil {
			t.Fatal("blank query rejected")
		}
	}
	for _, ext := range []string{"PNG", "webm", "gif", "jxl"} {
		got, err := parse(testSource("gelbooru"), []byte(fmt.Sprintf(`{"post":[{"id":1,"file_url":"https://cdn.example.org/media.%s?cache=1"}]}`, ext)), 1, 24)
		if err != nil || got.Posts[0].Format != strings.ToLower(ext) {
			t.Fatal("missing media kind", got, err)
		}
	}
	for _, engine := range []string{"danbooru", "gelbooru", "moebooru", "e621", "philomena", "pawchive", "kemono", "pixiv"} {
		s := testSource(engine)
		if engine == "pixiv" {
			s.APIKey = "test-token"
		}
		c := testClient(func(r *http.Request) (*http.Response, error) {
			switch engine {
			case "pixiv":
				if r.URL.Path != "/v1/illust/ranking" || r.URL.Query().Get("mode") != "day" {
					t.Fatal("blank Pixiv browse is not daily ranking")
				}
				return response(200, `{"illusts":[]}`), nil
			case "pawchive", "kemono":
				return response(200, `[]`), nil
			case "e621":
				return response(200, `{"posts":[]}`), nil
			case "philomena":
				if r.URL.Query().Get("q") != "*" {
					t.Fatal("blank Philomena is not match-all")
				}
				return response(200, `{"images":[]}`), nil
			case "gelbooru":
				return response(200, `{"post":[]}`), nil
			default:
				return response(200, `[]`), nil
			}
		})
		if _, err := c.Search(context.Background(), s, "", 1, 24); err != nil {
			t.Fatal(engine, err)
		}
	}
}
func TestDirectionalProviderSorting(t *testing.T) {
	for _, tt := range []struct{ engine, sort, want string }{
		{"danbooru", "oldest", "cat order:id_asc"}, {"danbooru", "score_desc", "cat order:score"},
		{"e621", "score_asc", "cat order:score_asc"}, {"moebooru", "oldest", "cat order:id_asc"},
		{"gelbooru", "score_asc", "cat sort:score:asc"}, {"gelbooru", "score_desc", "cat sort:score:desc"}, {"gelbooru", "oldest", "cat sort:id:asc"},
	} {
		got, err := testSource(tt.engine).filterQuery("cat", SearchOptions{Sort: tt.sort})
		if err != nil || got != tt.want {
			t.Fatal(tt, got, err)
		}
	}
	for _, sort := range []string{"oldest", "score_asc", "score_desc"} {
		req, err := testSource("philomena").request(context.Background(), "*", 1, 24, SearchOptions{Sort: sort})
		if err != nil {
			t.Fatal(err)
		}
		field, direction := "score", "asc"
		if sort == "oldest" {
			field = "id"
		}
		if sort == "score_desc" {
			direction = "desc"
		}
		if req.URL.Query().Get("sf") != field || req.URL.Query().Get("sd") != direction {
			t.Fatal("wrong Philomena order", req.URL)
		}
	}
}

func TestDeletedFavoritesKeepProviderPaging(t *testing.T) {
	s := testSource("gelbooru")
	s.UserID, s.APIKey = "123", "key"
	c := testClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("s") == "favorite" {
			return response(200, `{"@attributes":{"count":5},"favorite":[{"favorite":42}]}`), nil
		}
		return response(200, `{"@attributes":{"count":0}}`), nil
	})
	got, err := c.AccountFavorites(context.Background(), s, 1, 1)
	if err != nil || len(got.Posts) != 0 || !got.HasMore || *got.Total != 5 {
		t.Fatal(got, err)
	}
}
func TestBlankPixivDoesNotPromiseRecentOrdering(t *testing.T) {
	s := testSource("pixiv")
	s.APIKey = "token"
	c := testClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("unsupported ordering reached network")
		return nil, nil
	})
	if _, err := c.SearchFiltered(context.Background(), s, "", 1, 24, SearchOptions{Sort: "newest"}); err == nil {
		t.Fatal("daily rankings treated as recent ordering")
	}
}

func TestGelbooruFavoritesPagingIgnoresUnderreportedTotal(t *testing.T) {
	s := testSource("gelbooru")
	s.UserID, s.APIKey = "123", "key"
	for _, page := range []int{1, MaxPage} {
		c := testClient(func(r *http.Request) (*http.Response, error) {
			if r.URL.Query().Get("s") == "favorite" {
				return response(200, `{"@attributes":{"count":1},"favorite":[{"favorite":42},{"favorite":43}]}`), nil
			}
			return response(200, fmt.Sprintf(`{"post":[{"id":%s}]}`, r.URL.Query().Get("id"))), nil
		})
		got, err := c.AccountFavorites(context.Background(), s, page, 2)
		if err != nil || len(got.Posts) != 2 || got.HasMore != (page < MaxPage) {
			t.Fatal("reported total truncated full page", page, got, err)
		}
	}
}
