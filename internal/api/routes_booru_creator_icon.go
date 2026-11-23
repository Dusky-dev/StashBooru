package api

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/booru"
)

// CreatorIcon serves the profile picture of a connected Pawchive or Kemono
// creator account, fetched through the source's media allowlist and size
// limit. The Artist page offers it as the Artist image; nothing is saved here.
func (rs booruBrowserRoutes) CreatorIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	instance, target := r.URL.Query().Get("instance"), r.URL.Query().Get("target")
	sources, err := rs.sources()
	if err != nil {
		writeBooruError(w, 500, "configuration", "Booru sources could not be read.")
		return
	}
	var source booru.Source
	found := false
	for _, candidate := range sources {
		if candidate.Instance() == instance && booru.ArchiveCreatorEngine(candidate.Engine) && !candidate.Disabled {
			source, found = candidate, true
			break
		}
	}
	if !found {
		writeBooruError(w, 404, "source", "That Pawchive or Kemono source is no longer configured or is disabled.")
		return
	}
	icon, err := booru.CreatorIconURL(source, target)
	if err != nil {
		writeBooruError(w, 400, "invalid-query", err.Error())
		return
	}
	var buf bytes.Buffer
	info, err := rs.client.FetchMedia(r.Context(), source, booru.MediaRequest{URL: icon, Limit: 4 << 20, ImagesOnly: true}, &buf)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "HTTP 204") || strings.Contains(err.Error(), "unsupported media type") || strings.Contains(err.Error(), "cannot read media") {
			writeBooruError(w, 404, "no-picture", source.Name+" has no profile picture for this creator.")
			return
		}
		writeBooruError(w, 502, "media", err.Error())
		return
	}
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "icon", time.Time{}, bytes.NewReader(buf.Bytes()))
}
