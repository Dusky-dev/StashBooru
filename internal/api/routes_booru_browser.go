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
	importFile    func(context.Context, string, booru.Source, booru.Post, bool, *job.Progress) (booruLibraryResult, error)
	autoTagImage func(context.Context, int, bool, bool) error
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
	blacklistMode := query["blacklist"]
	if len(blacklistMode) != 0 && (len(blacklistMode) != 1 || blacklistMode[0] != "show") {
		writeBooruError(w, http.StatusBadRequest, "invalid-query", "Invalid blacklist filter.")
		return
	}
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
	fill, filters, valid := booruPageRequestFilters(query)
	if !valid || err != nil || limit < 1 || (fill && limit > booruDisplayMaxLimit) || (!fill && limit > booru.MaxLimit) || booru.ValidateQuery(query.Get("q"), page, min(limit, booru.MaxLimit)) != nil {
		writeBooruError(w, http.StatusBadRequest, "invalid-query", "Use a query of at most 1024 bytes, a page from 1–750 and page size from 1–250 in filled mode (1–50 otherwise).")
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
		fetch := func(ctx context.Context, providerPage, providerLimit int) (booru.Result, error) {
			if filtered, ok := rs.client.(interface {
				SearchFiltered(context.Context, booru.Source, string, int, int, booru.SearchOptions) (booru.Result, error)
			}); ok {
				return filtered.SearchFiltered(ctx, source, query.Get("q"), providerPage, providerLimit, options)
			}
			if options.Sort != "" || options.Rating != "" {
				return booru.Result{}, &booru.Error{Code: "unsupported-filter", Message: "Filters unavailable for this source."}
			}
			return rs.client.Search(ctx, source, query.Get("q"), providerPage, providerLimit)
		}
		var result booru.Result
		if fill {
			filters.hideBlocked = len(blacklistMode) == 0
			if filters.hideIgnored {
				var marked bool
				marked, err = rs.booruHasIgnored(r.Context(), source)
				if !marked {
					filters.hideIgnored = false
				} else {
					filters.excludeIgnored = func(ctx context.Context, posts []booru.Post) ([]booru.Post, error) {
						return rs.booruUnignored(ctx, source, posts)
					}
				}
			}
			if err == nil {
				result, err = fillBooruPage(r.Context(), source, page, limit, filters, fetch)
			}
		} else {
			result, err = fetch(r.Context(), page, limit)
		}
		if err != nil {
			failure := &booru.Error{Code: "unavailable", Message: "Provider search failed."}
			var providerError *booru.Error
			if errors.As(err, &providerError) {
				failure = providerError
			}
			status := http.StatusBadGateway
			if failure.Code == "invalid-query" || failure.Code == "invalid-page" || failure.Code == "page-too-deep" || failure.Code == "unsupported-filter" || failure.Code == "unsupported-query" {
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
		// A single archive post may contain hundreds of supported media
		// attachments. Rejecting the whole search page at 256 items hides
		// otherwise valid results. Provider post/page parsing remains bounded,
		// and the inspector renders attachments incrementally.
		// Exclude blacklisted posts before categorizing or granting preview media keys.
		// An explicit browser toggle can temporarily reveal saved blacklist entries.
		if !fill && len(blacklistMode) == 0 {
			result = source.FilterBlacklisted(result)
		}
		result.CategoryStatus = rs.categorizeBooruPosts(r.Context(), source, result.Posts)
		// Retain full post tags in short-lived download keys only when the
		// server setting is enabled; do not retain them for ordinary browsing.
		prefs, prefsErr := rs.downloadSettings()
		captureDownloadTags := prefsErr == nil && prefs.ImportTags
		rs.captureIdentityPosts(r.Context(), source, result.Posts)
		for i := range result.Posts {
			result.Posts[i] = publicBooruPost(source, result.Posts[i], captureDownloadTags)
		}

		writeVisualSimilarityJSON(w, result)
		return
	}
	writeBooruError(w, http.StatusBadRequest, "unknown-source", "Choose an enabled source from the server's source list.")
}

func (rs booruBrowserRoutes) categorizeBooruPosts(ctx context.Context, source booru.Source, posts []booru.Post) string {
	// Load the CSV for lookup prioritization, but DO NOT apply its types
	// until after live Rule34 classification. Older CSV-known tags must
	// not starve CSV-missing tags in the live 100-name lookup budget.
	db, dbErr := rs.tagDatabase(source)
	if source.IsRule34() {
		if categorizer, ok := rs.client.(interface {
			CategorizeRule34TagsPrioritized(context.Context, booru.Source, []booru.Post, func(string) bool)
		}); ok {
			categorizer.CategorizeRule34TagsPrioritized(ctx, source, posts, func(name string) bool {
				if dbErr != nil || db.index == nil {
					return true
				}
				category := db.index.Category(name)
				return category == "" || category == "unknown"
			})
		} else if categorizer, ok := rs.client.(interface {
			CategorizeRule34Tags(context.Context, booru.Source, []booru.Post)
		}); ok {
			categorizer.CategorizeRule34Tags(ctx, source, posts)
		}
	}

	// Confirmed live categories and provider types win. User-installed
	// CSV fills in only unresolved names, even when live API is blocked.
	if dbErr == nil && db.index != nil {
		for i := range posts {
			for j, tag := range posts[i].Tags {
				if tag.Category == "unknown" {
					if category := db.index.Category(tag.Name); category != "" && category != "unknown" {
						posts[i].Tags[j].Category = category
					}
				}
			}
		}
	}
	if !source.IsRule34() {
		return ""
	}
	unknown := false
	for _, post := range posts {
		for _, tag := range post.Tags {
			if tag.Category == "unknown" {
				unknown = true
				break
			}
		}
	}
	if !unknown || ctx.Err() != nil {
		return ""
	}
	// A historical auto-catalog remains the last resort, never a reason
	// to suppress live Rule34 lookups or invalidate an installed CSV.
	index, status := rs.rule34Catalog(source)
	if index != nil {
		for i := range posts {
			for j, tag := range posts[i].Tags {
				if tag.Category == "unknown" {
					if category := index.Category(tag.Name); category != "" && category != "unknown" {
						posts[i].Tags[j].Category = category
					}
				}
			}
		}
	}
	return status
}
func writeBooruError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeVisualSimilarityJSON(w, &booru.Error{Code: code, Message: message})
}
