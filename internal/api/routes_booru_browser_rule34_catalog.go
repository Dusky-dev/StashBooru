package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

// This is a historical, Rule34-only fallback when live HTML and tag DAPI
// do not expose categories. No queries or API credentials reach the mirror.
const (
	rule34AutoCatalogURL   = "https://huggingface.co/datasets/deepghs/site_tags/resolve/main/rule34.xxx/tags.csv"
	rule34AutoCatalogLimit = booru.MaxTagCSVBytes
	rule34AutoCatalogRetry = time.Hour
)

type rule34AutoCatalogEntry struct {
	index *booru.TagIndex
	state string
	tried time.Time
}

var rule34AutoCatalogs = struct {
	sync.Mutex
	entries map[string]*rule34AutoCatalogEntry
}{entries: map[string]*rule34AutoCatalogEntry{}}

func rule34CatalogRedirectAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "huggingface.co", "cdn-lfs.hf.co", "cdn-lfs-us-1.hf.co", "cdn-lfs-eu-1.hf.co", "cas-bridge.xethub.hf.co":
		return true
	}
	return false
}

// The URL is fixed and redirects are restricted to the dataset's known
// download hosts. Validate a complete bounded CSV before atomic publication.
func downloadRule34AutoCatalog(ctx context.Context, path string, client *http.Client) (*booru.TagIndex, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rule34AutoCatalogURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	httpClient := *client
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !rule34CatalogRedirectAllowed(req.URL.String()) {
			return fmt.Errorf("untrusted tag catalog redirect")
		}
		return nil
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Rule34 tag catalog unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > rule34AutoCatalogLimit {
		return nil, fmt.Errorf("Rule34 tag catalog unavailable or too large (HTTP %d)", response.StatusCode)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".rule34-auto-*.csv")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	count, err := io.Copy(file, io.LimitReader(response.Body, rule34AutoCatalogLimit+1))
	if err != nil || count <= 0 || count > rule34AutoCatalogLimit || (response.ContentLength >= 0 && count != response.ContentLength) {
		return nil, fmt.Errorf("Rule34 tag catalog truncated or exceeded 64 MiB")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	index, err := booru.ReadTagCSVForEngine(file, "gelbooru")
	if err != nil {
		return nil, fmt.Errorf("invalid Rule34 catalog: %w", err)
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return nil, err
	}
	return index, nil
}

// Start only one bounded installation per server config and never delay
// provider searches for the 46 MB public catalog. The next search uses it.
func (rs booruBrowserRoutes) rule34Catalog(source booru.Source) (*booru.TagIndex, string) {
	if !source.IsRule34() {
		return nil, ""
	}
	path := filepath.Join(rs.configPath, "booru-tags", "rule34-auto.csv")
	rule34AutoCatalogs.Lock()
	if item, ok := rule34AutoCatalogs.entries[path]; ok {
		index, state := item.index, item.state
		if state != "failed" || time.Since(item.tried) < rule34AutoCatalogRetry {
			rule34AutoCatalogs.Unlock()
			return index, state
		}
	}
	rule34AutoCatalogs.entries[path] = &rule34AutoCatalogEntry{state: "loading", tried: time.Now()}
	rule34AutoCatalogs.Unlock()

	go func() {
		var index *booru.TagIndex
		file, err := os.Open(path)
		if err == nil {
			info, statErr := file.Stat()
			if statErr == nil && info.Mode().IsRegular() && info.Size() <= rule34AutoCatalogLimit {
				index, err = booru.ReadTagCSVForEngine(file, "gelbooru")
			}
			file.Close()
		}
		if index == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			client := &http.Client{Timeout: 2 * time.Minute}
			index, err = downloadRule34AutoCatalog(ctx, path, client)
		}
		rule34AutoCatalogs.Lock()
		defer rule34AutoCatalogs.Unlock()
		item := rule34AutoCatalogs.entries[path]
		if index == nil {
			item.state = "failed"
		} else {
			item.index, item.state = index, "ready"
		}
		item.tried = time.Now()
	}()
	return nil, "loading"
}
