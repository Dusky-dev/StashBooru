package api

import (
	"net/http"

	"github.com/stashapp/stash/pkg/booru"
)

type sharedLinkRequest struct {
	URL string `json:"url"`
}

// SharedLink is POST-only: Dropbox share keys and MEGA decryption keys must
// never be copied into a query URL, saved filter, browser history or server log.
func (rs booruBrowserRoutes) SharedLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input sharedLinkRequest
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	post, err := booru.SharedLinkPost(input.URL)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "invalid-link", err.Error())
		return
	}
	source := booru.SharedLinkSource()
	post.MediaKey = rememberBooruPost(source, post)
	if post.MediaKey == "" {
		writeBooruError(w, http.StatusInternalServerError, "link", "Cannot create a temporary shared-file reference.")
		return
	}
	// These are server-generated URLs. Never expose original download URLs,
	// Dropbox sharing keys or MEGA decryption keys in the JSON response.
	if post.PreviewURL != "" {
		post.PreviewURL = "booru-browser/media?key=" + post.MediaKey + "&kind=preview"
		post.SampleURL = "booru-browser/media?key=" + post.MediaKey + "&kind=sample"
	}
	writeVisualSimilarityJSON(w, struct {
		Source booru.PublicSource `json:"source"`
		Post   booru.Post         `json:"post"`
	}{Source: source.Public(), Post: post})
}
