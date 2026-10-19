package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/stashapp/stash/pkg/booru"
)

func (rs booruBrowserRoutes) Creators(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources before searching artists.")
		return
	}
	page := 1
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	query := r.URL.Query().Get("q")
	if err != nil || !booru.ValidateCreatorQuery(query, page) {
		writeBooruError(w, 400, "invalid-query", "Enter an artist name of 2–128 bytes.")
		return
	}
	client, ok := rs.client.(interface {
		SearchCreators(context.Context, booru.Source, string, int) (booru.CreatorResult, error)
	})
	if !ok {
		writeBooruError(w, 400, "unsupported-query", "Artist directory search is unavailable.")
		return
	}
	result, err := client.SearchCreators(r.Context(), source, query, page)
	if err != nil {
		var failure *booru.Error
		if !errors.As(err, &failure) {
			writeBooruError(w, 502, "unavailable", "Artist directory could not be loaded.")
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
	if _, ok = rs.tagSource(r); !ok {
		writeBooruError(w, 409, "source", "Source changed during artist search. Reload sources.")
		return
	}
	writeVisualSimilarityJSON(w, result)
}
