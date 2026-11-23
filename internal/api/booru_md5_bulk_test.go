package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/camietagger"
)

func countingProvider(name string, calls *[]string, result func() (*booruPost, error)) booruProvider {
	return booruProvider{name: name, fetch: func(context.Context, string) (*booruPost, error) {
		*calls = append(*calls, name)
		return result()
	}}
}

func TestBulkBooruLookupAsksSitesInOrderAndStopsAtMatch(t *testing.T) {
	var calls []string
	noMatch := func() (*booruPost, error) { return nil, errBooruNoMatch }
	lookup := newBulkBooruLookup([]booruProvider{
		countingProvider("first", &calls, noMatch),
		countingProvider("second", &calls, func() (*booruPost, error) { return &booruPost{ID: "7"}, nil }),
		countingProvider("third", &calls, noMatch),
	})
	provider, post, err := lookup.lookup(context.Background(), "ABC")
	if err != nil || provider.name != "second" || post.ID != "7" {
		t.Fatalf("got %q %+v %v", provider.name, post, err)
	}
	if strings.Join(calls, ",") != "first,second" {
		t.Fatalf("sites asked: %v", calls)
	}
	// A repeated hash in the same job is not asked again.
	if _, _, err := lookup.lookup(context.Background(), "abc"); err != nil || len(calls) != 2 {
		t.Fatalf("repeat lookup asked again: %v %v", calls, err)
	}
}

func TestBulkBooruLookupDropsRateLimitedSite(t *testing.T) {
	var calls []string
	noMatch := func() (*booruPost, error) { return nil, errBooruNoMatch }
	lookup := newBulkBooruLookup([]booruProvider{
		countingProvider("limited", &calls, func() (*booruPost, error) {
			return nil, &booru.Error{Code: "rate-limited", Message: "slow down"}
		}),
		countingProvider("other", &calls, noMatch),
	})
	if _, _, err := lookup.lookup(context.Background(), "a"); !errors.Is(err, errBooruNotChecked) {
		t.Fatalf("a rate-limited site must not count as no match, got %v", err)
	}
	if _, _, err := lookup.lookup(context.Background(), "b"); !errors.Is(err, errBooruNotChecked) {
		t.Fatalf("got %v", err)
	}
	if strings.Join(calls, ",") != "limited,other,other" {
		t.Fatalf("a rate-limited site must be skipped for the rest of the job: %v", calls)
	}

	calls = nil
	allMissing := newBulkBooruLookup([]booruProvider{countingProvider("only", &calls, noMatch)})
	if _, _, err := allMissing.lookup(context.Background(), "c"); !errors.Is(err, errBooruNoMatch) {
		t.Fatalf("every site answering no match is a no match, got %v", err)
	}
}

func TestBulkBooruLookupDropsFailingSite(t *testing.T) {
	var calls []string
	lookup := newBulkBooruLookup([]booruProvider{
		countingProvider("down", &calls, func() (*booruPost, error) { return nil, errors.New("HTTP 503") }),
		countingProvider("locked", &calls, func() (*booruPost, error) {
			return nil, &booru.Error{Code: "authentication", Message: "bad key"}
		}),
	})
	for _, hash := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := lookup.lookup(context.Background(), hash); !errors.Is(err, errBooruNotChecked) {
			t.Fatalf("a failing site must leave the Image not checked, got %v", err)
		}
	}
	if strings.Join(calls, ",") != "down,locked,down,down" {
		t.Fatalf("a site must be dropped after rejecting credentials or failing three times in a row: %v", calls)
	}
}

func TestBulkBooruPredictionsSkipUncategorizedTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The site answers for "smile" only; "hero" gets no answer.
		if r.URL.Query().Get("name") != "smile" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "smile", "type": 0}})
	}))
	defer server.Close()
	provider := booruProviders[legacyBooruLookupProviders["yandere"]]
	provider.singleTagInfoURL = func(name string) string { return server.URL + "/?name=" + name }
	lookup := newBulkBooruLookup([]booruProvider{provider})
	lookup.gates = newBooruHostGates(0)

	predictions := lookup.bulkBooruPredictions(context.Background(), provider, &booruPost{ID: "1", Tags: "hero smile"})
	if len(predictions) != 1 || predictions[0].Category != "general" || predictions[0].RawName != "smile" {
		t.Fatalf("an uncategorized name must not be applied as a Tag: %+v", predictions)
	}

	browser := booruPostFromBrowser(booru.Post{MD5: "x", Tags: []booru.Tag{
		{Name: "hero", Category: "unknown"},
		{Name: "furry", Category: "species"},
	}})
	predictions = lookup.bulkBooruPredictions(context.Background(), booruProvider{name: "e621"}, browser)
	if len(predictions) != 1 || predictions[0].RawName != "furry" {
		t.Fatalf("a browser tag without a category must not be applied: %+v", predictions)
	}
}

func TestBooruHostGatesSpaceRequestsAndBlockRateLimitedHosts(t *testing.T) {
	gates := newBooruHostGates(40 * time.Millisecond)
	ctx := context.Background()
	start := time.Now()
	for range 3 {
		if err := gates.wait(ctx, "example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
		t.Fatalf("three requests to one host took only %v", elapsed)
	}
	if err := gates.wait(ctx, "other.example"); err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	gated := withBooruHostGates(ctx, newBooruHostGates(0))
	if _, err := fetchBooruURL(gated, server.URL); !errors.Is(err, errBooruRateLimited) {
		t.Fatalf("HTTP 429 must report a rate limit, got %v", err)
	}
	if _, err := fetchBooruURL(gated, server.URL); !errors.Is(err, errBooruRateLimited) || requests.Load() != 1 {
		t.Fatalf("a rate-limited host must not be asked again in the job: %v, %d requests", err, requests.Load())
	}
}

func TestBulkBooruLookupRemembersTagCategories(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		mu.Lock()
		asked = append(asked, name)
		mu.Unlock()
		category := "0"
		if name == "hero" {
			category = "4"
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"name": name, "type": category}})
	}))
	defer server.Close()
	provider := booruProviders[legacyBooruLookupProviders["yandere"]]
	provider.singleTagInfoURL = func(name string) string { return server.URL + "/?name=" + name }

	lookup := newBulkBooruLookup([]booruProvider{provider})
	lookup.gates = newBooruHostGates(0)
	first := lookup.tagCategories(context.Background(), provider, &booruPost{Tags: "hero smile"})
	second := lookup.tagCategories(context.Background(), provider, &booruPost{Tags: "hero blush"})
	if first["hero"] != "character" || second["hero"] != "character" {
		t.Fatalf("categories: %v %v", first, second)
	}
	mu.Lock()
	sort.Strings(asked)
	got := strings.Join(asked, ",")
	mu.Unlock()
	if got != "blush,hero,smile" {
		t.Fatalf("a tag seen earlier in the job must not be asked again: %v", asked)
	}
}

func TestBulkBooruLookupRecognisesChosenSitePostLinks(t *testing.T) {
	lookup := newBulkBooruLookup([]booruProvider{{name: "Danbooru", hosts: []string{"danbooru.donmai.us"}}})
	if !lookup.hasPostLink([]string{"https://danbooru.donmai.us/posts/1"}) {
		t.Fatal("a chosen site's post link must be recognised")
	}
	if lookup.hasPostLink([]string{"https://example.com/a", "not a url"}) {
		t.Fatal("other links must not skip the lookup")
	}
}

func TestBulkRequestMD5AndPixaiGates(t *testing.T) {
	config := defaultCamieConfig()
	request := func() bulkImageTagRequest {
		var request bulkImageTagRequest
		request.Confirmed = true
		request.ImageIDs = []int{1, 2}
		request.ApplyTags = true
		return request
	}
	md5Only := request()
	md5Only.UseBooru = true
	if status, err := validateBulkImageTagRequest(&md5Only, config, 0); err == nil || status != http.StatusConflict {
		t.Fatalf("MD5 lookup without usable sites must be rejected, got %d %v", status, err)
	}
	if _, err := validateBulkImageTagRequest(&md5Only, config, 2); err != nil {
		t.Fatalf("MD5 lookup must not need the model opt-in: %v", err)
	}

	pixai := request()
	pixai.UsePixai = true
	if status, err := validateBulkImageTagRequest(&pixai, config, 0); err == nil || status != http.StatusConflict {
		t.Fatalf("PixAI must stay behind the model opt-in, got %d %v", status, err)
	}
	config.BulkTaggingEnabled = true
	if _, err := validateBulkImageTagRequest(&pixai, config, 0); err != nil {
		t.Fatal(err)
	}
	if pixai.PixaiThreshold != defaultPixaiThreshold {
		t.Fatalf("PixAI threshold default not filled: %v", pixai.PixaiThreshold)
	}
}

func TestMergeBulkImageTagsMD5MatchOutranksModels(t *testing.T) {
	booruTags := []camietagger.Tag{
		{Name: "posted_hero", Category: "character", Score: 1, Source: "booru"},
		{Name: "smile", Category: "general", Score: 1, Source: "booru"},
	}
	pixai := []camietagger.Tag{
		{Name: "model_hero", Category: "character", Score: .9, Source: "pixai"},
		{Name: "smile", Category: "general", Score: .5, Source: "pixai"},
		{Name: "blush", Category: "general", Score: .5, Source: "pixai"},
	}
	merged := mergeBulkImageTags(nil, booruTags, pixai)
	var got []string
	for _, prediction := range merged {
		got = append(got, prediction.Category+":"+prediction.Source)
	}
	if strings.Join(got, ",") != "character:booru,general:booru,general:pixai" {
		t.Fatalf("merged: %v", got)
	}

	filename := []camietagger.Tag{{Name: "file_hero", Category: "character", Score: 1, Source: "filename"}}
	merged = mergeBulkImageTags(filename, booruTags)
	for _, prediction := range merged {
		if prediction.Category == "character" && prediction.Source != "filename" {
			t.Fatalf("filename identity must outrank the MD5 match: %+v", merged)
		}
	}
}
