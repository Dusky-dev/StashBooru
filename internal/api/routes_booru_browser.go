package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

type booruBrowserClient interface {
	Search(context.Context, booru.Source, string, int, int) (booru.Result, error)
	FetchMedia(context.Context, booru.Source, booru.MediaRequest, io.Writer) (booru.MediaInfo, error)
}

// These routes share the server's normal authentication. They never modify the
// catalogue without import review, or accept an arbitrary URL from the browser.
type booruBrowserRoutes struct {
	repository   models.Repository
	configPath   string
	client       booruBrowserClient
	jobs         *job.Manager
	downloads    *manager.DownloadStore
	libraryRoots func() []string
	importFile   func(context.Context, string, booru.Source, booru.Post, bool, *job.Progress) (booruLibraryResult, error)
}

func (rs booruBrowserRoutes) sources() ([]booru.Source, error) {
	file, err := os.Open(filepath.Join(rs.configPath, "booru-browser.json"))
	if errors.Is(err, os.ErrNotExist) {
		return booru.Configure(nil)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("cannot read booru sources configuration")
	}
	return booru.Configure(data)
}

func (rs booruBrowserRoutes) Sources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	sources, err := rs.sources()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "configuration", "Cannot load booru-browser.json. Check the server configuration; credentials are never returned here.")
		return
	}
	public := make([]booru.PublicSource, 0, len(sources))
	for _, source := range sources {
		public = append(public, source.Public())
	}
	writeVisualSimilarityJSON(w, public)
}

func (rs booruBrowserRoutes) Search(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	page, err := strconv.Atoi(query.Get("page"))
	if query.Get("page") == "" {
		page, err = 1, nil
	}
	limit := 24
	if raw := query.Get("limit"); raw != "" {
		var limitErr error
		limit, limitErr = strconv.Atoi(raw)
		if limitErr != nil {
			err = limitErr
		}
	}
	if err != nil || booru.ValidateQuery(query.Get("q"), page, limit) != nil {
		writeBooruError(w, http.StatusBadRequest, "invalid-query", "Enter search tags, a page from 1–750 and page size from 1–50.")
		return
	}
	sources, err := rs.sources()
	if err != nil {
		writeBooruError(w, http.StatusInternalServerError, "configuration", "Cannot load booru-browser.json. Check the server configuration.")
		return
	}
	for _, source := range sources {
		if source.ID != query.Get("source") {
			continue
		}
		options := booru.SearchOptions{Sort: query.Get("sort"), Rating: query.Get("rating")}
		if !options.Validate() {
			writeBooruError(w, 400, "invalid-query", "Invalid sort or rating filter.")
			return
		}
		var result booru.Result
		if filtered, ok := rs.client.(interface {
			SearchFiltered(context.Context, booru.Source, string, int, int, booru.SearchOptions) (booru.Result, error)
		}); ok {
			result, err = filtered.SearchFiltered(r.Context(), source, query.Get("q"), page, limit, options)
		} else if options.Sort != "" || options.Rating != "" {
			writeBooruError(w, 400, "unsupported-filter", "Filters unavailable for this source.")
			return
		} else {
			result, err = rs.client.Search(r.Context(), source, query.Get("q"), page, limit)
		}
		if err != nil {
			failure := &booru.Error{Code: "unavailable", Message: "Provider search failed."}
			var providerError *booru.Error
			if errors.As(err, &providerError) {
				failure = providerError
			}
			status := http.StatusBadGateway
			if failure.Code == "invalid-query" || failure.Code == "unsupported-filter" || failure.Code == "unsupported-query" {
				status = http.StatusBadRequest
			}
			if failure.Code == "rate-limited" {
				status = http.StatusTooManyRequests
			}
			if failure.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(failure.RetryAfter))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			writeVisualSimilarityJSON(w, failure)
			return
		}
		assets := len(result.Posts)
		for _, post := range result.Posts {
			assets += len(post.Attachments)
		}
		if assets > 256 {
			writeBooruError(w, 502, "media", "Provider page contains more than 256 media files. Reduce posts per page.")
			return
		}
		if db, err := rs.tagDatabase(source); err == nil && db.index != nil {
			for i := range result.Posts {
				for j, tag := range result.Posts[i].Tags {
					if tag.Category == "unknown" {
						if category := db.index.Category(tag.Name); category != "" {
							result.Posts[i].Tags[j].Category = category
						}
					}
				}
			}
		}
		for i := range result.Posts {
			result.Posts[i] = publicBooruPost(source, result.Posts[i])
		}

		writeVisualSimilarityJSON(w, result)
		return
	}
	writeBooruError(w, http.StatusBadRequest, "unknown-source", "Choose an enabled source from the server's source list.")
}

func writeBooruError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeVisualSimilarityJSON(w, &booru.Error{Code: code, Message: message})
}
