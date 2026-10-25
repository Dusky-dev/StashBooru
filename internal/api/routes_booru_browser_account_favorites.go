package api

import (
	"context"
	"errors"
	"github.com/stashapp/stash/pkg/booru"
	"net/http"
	"strconv"
)

func (rs booruBrowserRoutes) AccountFavorites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	blacklistMode := r.URL.Query()["blacklist"]
	if len(blacklistMode) != 0 && (len(blacklistMode) != 1 || blacklistMode[0] != "show") {
		writeBooruError(w, 400, "favorites", "Invalid blacklist filter.")
		return
	}
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources.")
		return
	}
	page, limit := 1, 24
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	if err == nil {
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
	}
	fill, filters, valid := booruPageRequestFilters(r.URL.Query())
	if !valid || err != nil || limit < 1 || (fill && limit > booruDisplayMaxLimit) || (!fill && limit > booru.MaxLimit) || booru.ValidateQuery("", page, min(limit, booru.MaxLimit)) != nil || r.URL.Query().Get("sort") != "" || r.URL.Query().Get("rating") != "" || r.URL.Query().Get("q") != "" {
		writeBooruError(w, 400, "favorites", "Account favorites use provider order, All ratings and an empty search. Invalid page or filters.")
		return
	}
	client, ok := rs.client.(interface {
		AccountFavorites(context.Context, booru.Source, int, int) (booru.Result, error)
	})
	if !ok {
		writeBooruError(w, 400, "favorites", "Account favorites are unavailable.")
		return
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
			result, err = fillBooruPage(r.Context(), source, page, limit, filters, func(ctx context.Context, providerPage, providerLimit int) (booru.Result, error) {
				return client.AccountFavorites(ctx, source, providerPage, providerLimit)
			})
		}
	} else {
		result, err = client.AccountFavorites(r.Context(), source, page, limit)
	}
	if err != nil {
		var failure *booru.Error
		if !errors.As(err, &failure) {
			writeBooruError(w, 502, "favorites", "Account favorites could not be loaded.")
			return
		}
		status := http.StatusBadGateway
		if failure.Code == "unsupported-query" || failure.Code == "invalid-query" || failure.Code == "invalid-page" || failure.Code == "page-too-deep" {
			status = http.StatusBadRequest
		}
		if failure.Code == "rate-limited" {
			status = http.StatusTooManyRequests
			if failure.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(failure.RetryAfter))
			}
		}
		writeBooruError(w, status, failure.Code, failure.Message)
		return
	}
	current, ok := rs.tagSource(r)
	// Instance excludes credentials; reject a changed account on the same site.
	if !ok || current.Login != source.Login || current.UserID != source.UserID || current.APIKey != source.APIKey {
		writeBooruError(w, 409, "source", "Source account changed during favorites search. Reload sources.")
		return
	}
	if len(result.Posts) > limit {
		writeBooruError(w, 502, "favorites", "Provider exceeded the favorites page limit.")
		return
	}
	if !fill && len(blacklistMode) == 0 {
		result = source.FilterBlacklisted(result)
	}
	rs.categorizeBooruPosts(r.Context(), source, result.Posts)
	for i := range result.Posts {
		result.Posts[i] = publicBooruPost(source, result.Posts[i])
	}
	writeVisualSimilarityJSON(w, result)
}
