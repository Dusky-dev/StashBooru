package booru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestArchiveCreatorScopedKeywordSearch(t *testing.T) {
	for _, tc := range []struct {
		engine string
		path   string
	}{
		{"pawchive", "/api/v1/patreon/user/42"},
		{"kemono", "/api/v1/patreon/user/42/posts"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			calls := 0
			client := testClient(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != tc.path || r.URL.Query().Get("q") != "blue sky" ||
					r.URL.Query().Get("o") != "0" {
					t.Fatalf("creator scope, keywords or offset lost: %s", r.URL)
				}
				return response(http.StatusOK, `[{"id":"101","user":"42","service":"patreon","title":"Blue sky story"}]`), nil
			})
			got, err := client.Search(context.Background(), testSource(tc.engine), "creator:patreon:42 blue sky", 1, 24)
			if err != nil || calls != 1 || len(got.Posts) != 1 ||
				got.Posts[0].RemoteID != "101" || got.SearchNotice != "" {
				t.Fatalf("creator keyword search failed: %+v err=%v requests=%d", got, err, calls)
			}
		})
	}
}

func TestArchiveCreatorOnlyKeepsUnfilteredBrowsing(t *testing.T) {
	for _, tc := range []struct {
		engine string
		path   string
	}{
		{"pawchive", "/api/v1/patreon/user/42"},
		{"kemono", "/api/v1/patreon/user/42/posts"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			client := testClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != tc.path || r.URL.Query().Has("q") {
					t.Fatalf("plain creator browsing must not add keywords: %s", r.URL)
				}
				return response(http.StatusOK, `[{"id":"101","user":"42","service":"patreon"}]`), nil
			})
			got, err := client.Search(context.Background(), testSource(tc.engine), "creator:patreon:42", 1, 24)
			if err != nil || len(got.Posts) != 1 {
				t.Fatalf("plain creator browsing regressed: %+v err=%v", got, err)
			}
		})
	}
}

func TestArchiveCreatorScopedKeywordPagination(t *testing.T) {
	for _, engine := range []string{"pawchive", "kemono"} {
		t.Run(engine, func(t *testing.T) {
			offsets := []int{}
			client := testClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("q") != "blue sky" {
					t.Fatalf("lost the creator keyword on another page: %s", r.URL)
				}
				offset, err := strconv.Atoi(r.URL.Query().Get("o"))
				if err != nil || (offset != 0 && offset != 50) {
					t.Fatalf("unexpected upstream offset: %s", r.URL)
				}
				offsets = append(offsets, offset)
				rows := make([]map[string]string, 50)
				for i := range rows {
					rows[i] = map[string]string{
						"id": strconv.Itoa(offset + i + 1), "user": "42",
						"service": "patreon", "title": fmt.Sprintf("Blue sky post %d", offset+i+1),
					}
				}
				data, _ := json.Marshal(rows)
				return response(http.StatusOK, string(data)), nil
			})
			got, err := client.Search(context.Background(), testSource(engine), "creator:patreon:42 blue sky", 3, 24)
			if err != nil || len(got.Posts) != 24 ||
				got.Posts[0].RemoteID != "49" || got.Posts[23].RemoteID != "72" ||
				!got.HasMore || got.SearchNotice != "" || len(offsets) != 2 ||
				offsets[0] != 0 || offsets[1] != 50 {
				t.Fatalf("scoped creator search paging failed: %+v err=%v offsets=%v", got, err, offsets)
			}
		})
	}
}

func TestArchiveCreatorScopedSearchRejectsMalformedQueries(t *testing.T) {
	for _, engine := range []string{"pawchive", "kemono"} {
		t.Run(engine, func(t *testing.T) {
			client := testClient(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("invalid creator keyword query reached the provider: %s", r.URL)
				return nil, nil
			})
			for _, query := range []string{
				"creator:patreon:42 ab",
				"creator:patreon",
				"creator:patreon:../../etc blue sky",
				"creator:patreon:42:extra blue sky",
				"creator:patreon:42\nblue sky",
			} {
				_, err := client.Search(context.Background(), testSource(engine), query, 1, 24)
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != "invalid-query" {
					t.Fatalf("expected invalid query for %q: %v", query, err)
				}
			}
		})
	}
}

func TestPawchiveCreatorSearchNoticeUsesKeywordsOnly(t *testing.T) {
	client := testClient(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `[{"id":"101","user":"42","service":"patreon","title":"Unrelated post title"}]`), nil
	})
	got, err := client.Search(context.Background(), testSource("pawchive"), "creator:patreon:42 blue sky", 1, 24)
	if err != nil || !strings.Contains(got.SearchNotice, "may ignore search filters") {
		t.Fatalf("provider ignoring scoped keywords must still warn: %+v err=%v", got, err)
	}
}
