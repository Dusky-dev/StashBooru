package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

const (
	siteIconTTL        = 7 * 24 * time.Hour
	siteIconFailureTTL = 24 * time.Hour
)

var siteIconMu sync.Mutex

// SiteIcon serves a configured source's /favicon.ico. The browser names a
// source ID only; the URL comes from server configuration. Icons and misses
// are cached on disk so each site is asked at most once a day.
func (rs booruBrowserRoutes) SiteIcon(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("source")
	if !booruSavedSourceID.MatchString(id) {
		http.Error(w, "unknown source", http.StatusNotFound)
		return
	}
	sources, err := rs.sources()
	if err != nil {
		http.Error(w, "cannot load sources", http.StatusInternalServerError)
		return
	}
	var source *booru.Source
	for i := range sources {
		if sources[i].ID == id {
			source = &sources[i]
			break
		}
	}
	if source == nil || rs.iconDir == "" {
		http.Error(w, "unknown source", http.StatusNotFound)
		return
	}
	data, contentType, ok := rs.siteIcon(r.Context(), *source)
	if !ok {
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.Error(w, "no icon", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data)
}

func (rs booruBrowserRoutes) siteIcon(ctx context.Context, source booru.Source) ([]byte, string, bool) {
	siteIconMu.Lock()
	defer siteIconMu.Unlock()
	iconPath := filepath.Join(rs.iconDir, source.ID+".icon")
	missPath := filepath.Join(rs.iconDir, source.ID+".miss")
	// A changed source URL invalidates the cache.
	urlPath := filepath.Join(rs.iconDir, source.ID+".url")
	if cachedURL, err := os.ReadFile(urlPath); err == nil && string(cachedURL) == source.URL {
		if stat, err := os.Stat(iconPath); err == nil && time.Since(stat.ModTime()) < siteIconTTL {
			if data, err := os.ReadFile(iconPath); err == nil {
				if contentType := http.DetectContentType(data); strings.HasPrefix(contentType, "image/") {
					return data, contentType, true
				}
			}
		}
		if stat, err := os.Stat(missPath); err == nil && time.Since(stat.ModTime()) < siteIconFailureTTL {
			return nil, "", false
		}
	}
	fetch := rs.fetchIcon
	if fetch == nil {
		fetch = booru.FetchIcon
	}
	data, contentType, err := fetch(ctx, source)
	if mkErr := os.MkdirAll(rs.iconDir, 0o755); mkErr != nil {
		return data, contentType, err == nil
	}
	_ = os.WriteFile(urlPath, []byte(source.URL), 0o644)
	if err != nil {
		// Only a site that answered without an icon is remembered as a miss;
		// a cancelled request is retried next time.
		if errors.Is(err, booru.ErrNoIcon) {
			_ = os.Remove(iconPath)
			_ = os.WriteFile(missPath, nil, 0o644)
		}
		return nil, "", false
	}
	_ = os.Remove(missPath)
	_ = os.WriteFile(iconPath, data, 0o644)
	return data, contentType, true
}
