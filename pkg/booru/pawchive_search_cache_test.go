package booru

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPawchiveIdenticalSearchIsCached(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Query().Get("q") != "meili" {
			t.Fatalf("unexpected query %s", r.URL)
		}
		return response(http.StatusOK, `[{"id":"123","user":"42","service":"patreon","title":"meili archive"}]`), nil
	})
	s := testSource("pawchive")
	for range 3 {
		got, err := c.Search(context.Background(), s, "meili", 1, 24)
		if err != nil || len(got.Posts) != 1 || got.Posts[0].RemoteID != "123" {
			t.Fatalf("cached search failed: %+v %v", got, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("repeating the same search hit Pawchive %d times", calls.Load())
	}
	// Different provider page offsets must have different cache identities.
	if _, err := c.Search(context.Background(), s, "meili", 3, 24); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("different page reused the wrong cache entry: %d", calls.Load())
	}
}

func TestPawchiveSearchSingleFlight(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	c := testClient(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		once.Do(func() { close(started) })
		<-release
		return response(http.StatusOK, "[]"), nil
	})
	s := testSource("pawchive")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := c.Search(context.Background(), s, "meili", 1, 24)
		errs <- err
	}()
	<-started
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := c.Search(context.Background(), s, "meili", 1, 24)
		errs <- err
	}()
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("concurrent identical queries sent %d provider calls", calls.Load())
	}
}

func TestPawchive429WithoutRetryAfterHasCooldown(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(http.StatusTooManyRequests, ""), nil
	})
	s := testSource("pawchive")
	for range 2 {
		_, err := c.Search(context.Background(), s, "meili", 1, 24)
		var failure *Error
		if !errors.As(err, &failure) || failure.Code != "rate-limited" ||
			failure.RetryAfter < 59 || !strings.Contains(failure.Message, "retry") {
			t.Fatalf("Pawchive cooldown not respected: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("retry during cooldown contacted provider %d times", calls.Load())
	}
}

func TestPawchiveSearchCacheDoesNotMaskProviderErrors(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(_ *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return response(http.StatusBadGateway, "temporary"), nil
		}
		return response(http.StatusOK, "[]"), nil
	})
	s := testSource("pawchive")
	if _, err := c.Search(context.Background(), s, "meili", 1, 24); err == nil {
		t.Fatal("upstream error was ignored")
	}
	if _, err := c.Search(context.Background(), s, "meili", 1, 24); err != nil {
		t.Fatalf("provider error was accidentally cached: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("provider recovered but was not queried again: %d", calls.Load())
	}
}

func TestPawchiveCacheOnlyAffectsPublicKeywordSearch(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(http.StatusOK, "[]"), nil
	})
	s := testSource("pawchive")
	for range 2 {
		if _, err := c.extendedRequest(context.Background(), s, "/api/v1/posts", url.Values{"o": {"0"}}); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected caching of unfiltered recent browsing: %d", calls.Load())
	}
}
