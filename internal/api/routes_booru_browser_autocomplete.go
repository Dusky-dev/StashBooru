package api

import (
	"context"
	"net/http"

	"github.com/stashapp/stash/pkg/booru"
)

func (rs booruBrowserRoutes) Autocomplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	source, ok := rs.tagSource(r)
	if !ok {
		writeBooruError(w, 409, "source", "Source changed. Reload sources before completing tags.")
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode != "" && mode != "live" && mode != "database" {
		writeBooruError(w, 400, "invalid-query", "Invalid autocomplete mode.")
		return
	}
	prefix := r.URL.Query().Get("q")
	if !booru.ValidatePrefix(prefix) {
		writeBooruError(w, 400, "invalid-query", "Enter a tag prefix of 2–80 bytes without query operators.")
		return
	}
	var items []booru.Suggestion
	var liveErr error
	if r.URL.Query().Get("mode") != "database" {
		if client, ok := rs.client.(interface {
			Autocomplete(context.Context, booru.Source, string) ([]booru.Suggestion, error)
		}); ok {
			items, liveErr = client.Autocomplete(r.Context(), source, prefix)
			if liveErr == nil {
				if _, ok := rs.tagSource(r); !ok {
					writeBooruError(w, 409, "source", "Source changed during autocomplete. Reload sources.")
					return
				}
				writeVisualSimilarityJSON(w, map[string]any{"suggestions": items, "mode": "live"})
				return
			}
		}
	}
	if r.Context().Err() != nil {
		return
	}
	// Reuse exactly the installed CSV's validation/cache/revision path. A live
	// provider outage must never remove or replace a user's offline database.
	db, err := rs.tagDatabase(source)
	if err == nil && db.index != nil {
		if _, ok := rs.tagSource(r); !ok {
			writeBooruError(w, 409, "source", "Source changed during autocomplete. Reload sources.")
			return
		}
		notice := "Using the installed CSV database."
		if liveErr != nil {
			notice = "Live suggestions unavailable; using the installed CSV database."
		}
		writeVisualSimilarityJSON(w, map[string]any{"suggestions": db.index.Suggest(r.Context(), prefix), "mode": "database", "notice": notice})
		return
	}
	writeBooruError(w, 502, "autocomplete", "Live suggestions are unavailable and no usable CSV database is installed. Check source access or install an optional CSV in settings.")
}
