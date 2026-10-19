package booru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCreatorDirectoriesSearchPageAndCache(t *testing.T) {
	for _, engine := range []string{"pawchive", "kemono"} {
		t.Run(engine, func(t *testing.T) {
			calls := 0
			c := testClient(func(r *http.Request) (*http.Response, error) {
				calls++
				want := "/api/v1/creators"
				if engine == "kemono" {
					want += ".txt"
				}
				if r.URL.Path != want || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Fatal("wrong directory request", r.URL, r.Header)
				}
				rows := []map[string]string{{"id": "99", "name": "Unrelated", "service": "fanbox"}}
				for i := 0; i < 53; i++ {
					rows = append(rows, map[string]string{"id": fmt.Sprint(i + 1), "name": fmt.Sprintf("Artist %02d", i), "service": "patreon"})
				}
				rows = append(rows, rows[1], map[string]string{"id": "../bad", "name": "Artist bad", "service": "patreon"}, map[string]string{"id": "1", "name": "Artist 00", "service": "fanbox"})
				data, err := json.Marshal(rows)
				if err != nil {
					t.Fatal(err)
				}
				return response(200, string(data)), nil
			})
			s := testSource(engine)
			for page, want := range []int{50, 4, 0} {
				result, err := c.SearchCreators(context.Background(), s, "aRtIsT", page+1)
				if err != nil || len(result.Creators) != want || result.HasMore != (page == 0) {
					t.Fatal(page, result, err)
				}
			}
			result, err := c.SearchCreators(context.Background(), s, "Artist 00", 1)
			if err != nil || len(result.Creators) != 2 || result.Creators[0].Service != "fanbox" || calls != 1 {
				t.Fatal("directory cache or service identity lost", result, err, calls)
			}
		})
	}
}

func TestCreatorIndexSharesLoadsAndHonorsCancellation(t *testing.T) {
	var calls atomic.Int32
	ready := make(chan struct{})
	release := make(chan struct{})
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		close(ready)
		<-release
		return response(200, `[{"id":"1","name":"Artist","service":"patreon"}]`), nil
	})
	s := testSource("pawchive")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := c.SearchCreators(context.Background(), s, "artist", 1)
			if err != nil || len(result.Creators) != 1 {
				t.Error(result, err)
			}
		}()
	}
	<-ready
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.SearchCreators(ctx, s, "artist", 1); !errors.As(err, new(*Error)) {
		t.Error("cancelled search accepted", err)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("directory downloaded repeatedly", calls.Load())
	}
}

func TestCreatorDirectoryRejectsInvalidDataAndDoesNotCacheErrors(t *testing.T) {
	for _, data := range []string{`{}`, `[] {}`, `[{"name":1}]`, `[`} {
		calls := 0
		c := testClient(func(r *http.Request) (*http.Response, error) { calls++; return response(200, data), nil })
		for range 2 {
			if _, err := c.SearchCreators(context.Background(), testSource("kemono"), "artist", 1); err == nil {
				t.Fatal("invalid directory accepted", data)
			}
		}
		if calls != 2 {
			t.Fatal("directory error cached", calls)
		}
	}
	c := testClient(func(r *http.Request) (*http.Response, error) {
		t.Fatal("invalid input issued a request")
		return nil, nil
	})
	for _, query := range []string{"a", "bad\nname", ""} {
		if _, err := c.SearchCreators(context.Background(), testSource("pawchive"), query, 1); err == nil {
			t.Fatal(query)
		}
	}
	if _, err := c.SearchCreators(context.Background(), testSource("danbooru"), "artist", 1); err == nil {
		t.Fatal("unsupported engine accepted")
	}
}

func TestSensitiveRatingIsDistinctAndEngineAware(t *testing.T) {
	for _, engine := range []string{"danbooru", "gelbooru", "e621", "moebooru", "philomena"} {
		s := testSource(engine)
		got, err := s.filterQuery("landscape", SearchOptions{Rating: "sensitive"})
		switch engine {
		case "danbooru":
			if err != nil || got != "landscape rating:s" {
				t.Fatal(got, err)
			}
		case "gelbooru":
			if err != nil || got != "landscape rating:sensitive" {
				t.Fatal(got, err)
			}
		default:
			if err == nil {
				t.Fatal("unsupported rating accepted", engine)
			}
		}
	}
	for _, host := range []string{"https://rule34.xxx", "https://safebooru.org"} {
		s := testSource("gelbooru")
		s.URL = host
		if _, err := s.filterQuery("cat", SearchOptions{Rating: "sensitive"}); err == nil {
			t.Fatal(host)
		}
	}
}
