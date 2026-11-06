package api

import (
	"context"
	"net/http"

	"github.com/stashapp/stash/pkg/booru"
)

// PostDetails refreshes one server-owned Pawchive search snapshot on demand.
// The provider identity and post URL are never accepted from the browser.
func (rs booruBrowserRoutes) PostDetails(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	source, original, ok := rs.mediaSnapshot(r.URL.Query().Get("key"))
	if !ok {
		writeBooruError(w, http.StatusGone, "expired", "Post preview expired. Refresh the search.")
		return
	}
	detailer, supported := rs.client.(interface {
		PawchivePostDetail(context.Context, booru.Source, booru.Post) (booru.Post, error)
	})
	if !supported || source.Engine != "pawchive" || original.SourceID != source.ID {
		writeBooruError(w, http.StatusBadRequest, "unsupported", "Post details are only available for Pawchive posts.")
		return
	}
	detailed, err := detailer.PawchivePostDetail(r.Context(), source, original)
	if err != nil || detailed.ID != original.ID {
		writeBooruError(w, http.StatusBadGateway, "provider-error", "Could not load complete Pawchive post details. Use the original post link.")
		return
	}
	// Mirror the search handler's native identity projection: detailed results
	// must retain canonical Artist/Character/Copyright display identities.
	posts := []booru.Post{detailed}
	rs.captureIdentityPosts(r.Context(), source, posts)
	prefs, prefsErr := rs.downloadSettings()
	captureTags := prefsErr == nil && prefs.ImportTags
	// A new, bounded opaque snapshot is issued for each discovered linked item.
	writeVisualSimilarityJSON(w, struct {
		Post booru.Post `json:"post"`
	}{Post: publicBooruPost(source, posts[0], captureTags)})
}
