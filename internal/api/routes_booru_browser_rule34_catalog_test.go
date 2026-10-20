package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

type rule34CatalogTransport func(*http.Request) (*http.Response, error)

func (fn rule34CatalogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRule34CatalogRedirectHostRestrictions(t *testing.T) {
	for _, raw := range []string{
		"https://huggingface.co/data",
		"https://cdn-lfs.hf.co/object",
		"https://cas-bridge.xethub.hf.co/object?token=redacted",
	} {
		if !rule34CatalogRedirectAllowed(raw) {
			t.Fatal("trusted catalog host rejected", raw)
		}
	}
	for _, raw := range []string{
		"http://huggingface.co/file",
		"https://user:pass@huggingface.co/file",
		"https://huggingface.co:444/file",
		"https://huggingface.co.evil.example/file",
		"https://untrusted.cas-bridge.xethub.hf.co/file",
		"https://127.0.0.1/file",
	} {
		if rule34CatalogRedirectAllowed(raw) {
			t.Fatal("unsafe catalog redirect approved", raw)
		}
	}
}

func TestRule34CatalogDownloadAndAtomicValidation(t *testing.T) {
	body := "type,count,name,ambiguous,id\n1,42,test_artist,False,1\n4,32,test_character,False,2\n3,22,test_copyright,False,3\n0,100,test_general,False,4\n"
	path := filepath.Join(t.TempDir(), "booru-tags", "rule34-auto.csv")
	calls := 0
	client := &http.Client{Transport: rule34CatalogTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != rule34AutoCatalogURL || req.URL.Query().Get("api_key") != "" {
			t.Errorf("catalog request included a provider query or token: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: http.Header{}}, nil
	})}
	index, err := downloadRule34AutoCatalog(context.Background(), path, client)
	if err != nil || calls != 1 || index == nil {
		t.Fatalf("catalog failed download and parse: %v calls=%d", err, calls)
	}
	for name, want := range map[string]string{
		"test_artist": "artist", "test_character": "character",
		"test_copyright": "copyright", "test_general": "general",
	} {
		if got := index.Category(name); got != want {
			t.Errorf("category %s: %s != %s", name, got, want)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte(body)) {
		t.Fatal("catalog not published atomically", err)
	}
	invalidClient := &http.Client{Transport: rule34CatalogTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("invalid")), ContentLength: 7, Header: http.Header{}}, nil
	})}
	if _, err := downloadRule34AutoCatalog(context.Background(), path, invalidClient); err == nil {
		t.Fatal("invalid replacement catalog accepted")
	}
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte(body)) {
		t.Fatal("invalid download destroyed existing catalog")
	}
}

func TestRule34CatalogRejectsOversizedResponse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tags.csv")
	client := &http.Client{Transport: rule34CatalogTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("x")), ContentLength: rule34AutoCatalogLimit + 1, Header: http.Header{}}, nil
	})}
	if _, err := downloadRule34AutoCatalog(context.Background(), path, client); err == nil {
		t.Fatal("oversized response accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("oversized catalog was published", err)
	}
}

func TestRule34CatalogLoadsPersistedTaxonomyWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "booru-tags", "rule34-auto.csv")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const csv = "type,count,name,ambiguous,id\n1,5,artist_name,False,1\n4,3,character_name,False,2\n0,10,general_name,False,3\n"
	if err := os.WriteFile(path, []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	rs := booruBrowserRoutes{configPath: dir}
	source := booru.Defaults()[4]
	if source.ID != "rule34" {
		t.Fatal("fixture source changed")
	}
	if _, status := rs.rule34Catalog(source); status != "loading" {
		t.Fatalf("expected initial cache load, got %s", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	var index *booru.TagIndex
	var status string
	for time.Now().Before(deadline) {
		index, status = rs.rule34Catalog(source)
		if status == "ready" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != "ready" || index == nil {
		t.Fatal("cached taxonomy was not loaded", status)
	}
	posts := []booru.Post{{Tags: []booru.Tag{
		{Name: "artist_name", Category: "unknown"},
		{Name: "character_name", Category: "unknown"},
		{Name: "general_name", Category: "unknown"},
		{Name: "new_tag", Category: "unknown"},
		{Name: "typed", Category: "copyright"},
	}}}
	state := rs.categorizeBooruPosts(context.Background(), source, posts)
	if state != "ready" || posts[0].Tags[0].Category != "artist" ||
		posts[0].Tags[1].Category != "character" ||
		posts[0].Tags[2].Category != "general" ||
		posts[0].Tags[3].Category != "unknown" ||
		posts[0].Tags[4].Category != "copyright" {
		t.Fatalf("offline categories incorrect: %s %+v", state, posts)
	}
	if _, status := rs.rule34Catalog(booru.Defaults()[0]); status != "" {
		t.Fatal("Rule34 taxonomy leaked into other sources")
	}
}

func TestRule34CatalogFailureDoesNotBreakSearch(t *testing.T) {
	dir := t.TempDir()
	rs := booruBrowserRoutes{configPath: dir}
	source := booru.Defaults()[4]
	key := filepath.Join(dir, "booru-tags", "rule34-auto.csv")
	rule34AutoCatalogs.Lock()
	rule34AutoCatalogs.entries[key] = &rule34AutoCatalogEntry{state: "failed", tried: time.Now()}
	rule34AutoCatalogs.Unlock()
	defer func() {
		rule34AutoCatalogs.Lock()
		delete(rule34AutoCatalogs.entries, key)
		rule34AutoCatalogs.Unlock()
	}()
	posts := []booru.Post{{Tags: []booru.Tag{{Name: "unresolved", Category: "unknown"}}}}
	if got := rs.categorizeBooruPosts(context.Background(), source, posts); got != "failed" ||
		posts[0].Tags[0].Category != "unknown" {
		t.Fatal(fmt.Sprintf("unavailable fallback broke search: %s %+v", got, posts))
	}
}
