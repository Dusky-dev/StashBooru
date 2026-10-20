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
	if err != nil || booru.ValidateQuery("", page, limit) != nil || r.URL.Query().Get("sort") != "" || r.URL.Query().Get("rating") != "" || r.URL.Query().Get("q") != "" {
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
	result, err := client.AccountFavorites(r.Context(), source, page, limit)
	if err != nil {
		var failure *booru.Error
		if !errors.As(err, &failure) {
			writeBooruError(w, 502, "favorites", "Account favorites could not be loaded.")
			return
		}
		status := http.StatusBadGateway
		if failure.Code == "unsupported-query" || failure.Code == "invalid-query" {
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
	rs.categorizeBooruPosts(r.Context(), source, result.Posts)
	for i := range result.Posts {
		result.Posts[i] = publicBooruPost(source, result.Posts[i])
	}
	writeVisualSimilarityJSON(w, result)
}
