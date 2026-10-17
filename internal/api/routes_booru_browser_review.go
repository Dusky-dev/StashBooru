package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

var booruReviewID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var booruReviewMD5 = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

type booruReviewPost struct {
	ID  string `json:"id"`
	MD5 string `json:"md5"`
}
type booruReviewRead struct {
	Source   string            `json:"source"`
	Instance string            `json:"instance"`
	Posts    []booruReviewPost `json:"posts"`
}
type booruReviewUpdate struct {
	Source   string                     `json:"source"`
	Instance string                     `json:"instance"`
	Changes  []models.BooruReviewChange `json:"changes"`
}
type booruReviewMatch struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	FileID   string `json:"fileID"`
	Evidence string `json:"evidence"`
}
type booruReviewItem struct {
	models.BooruReviewState
	Matches     []booruReviewMatch `json:"matches"`
	MatchStatus string             `json:"matchStatus"`
	CheckedAt   string             `json:"checkedAt"`
}

func readBooruReviewJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	w.Header().Set("Cache-Control", "no-store")
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/json" {
		writeBooruError(w, 415, "content-type", "Review requests require application/json.")
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) {
			writeBooruError(w, 403, "origin", "Use this server's interface to review posts.")
			return false
		}
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeBooruError(w, 403, "origin", "Use this server's interface to review posts.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		writeBooruError(w, 400, "review", "Invalid review request.")
		return false
	}
	return true
}

func validBooruReviewID(id string) bool {
	if !booruReviewID.MatchString(id) {
		return false
	}
	_, err := strconv.ParseUint(id, 10, 64)
	return err == nil
}

func (rs booruBrowserRoutes) reviewSource(w http.ResponseWriter, id, instance string) *booru.Source {
	sources, err := rs.sources()
	if err != nil {
		writeBooruError(w, 500, "configuration", "Cannot load booru source settings.")
		return nil
	}
	for _, source := range sources {
		if source.ID == id {
			if source.Instance() != instance {
				writeBooruError(w, 409, "source-changed", "The source URL or engine changed. Reload sources and search again.")
				return nil
			}
			return &source
		}
	}
	writeBooruError(w, 400, "unknown-source", "Choose an enabled source.")
	return nil
}

func (rs booruBrowserRoutes) ReadReview(w http.ResponseWriter, r *http.Request) {
	var input booruReviewRead
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if len(input.Posts) > 50 {
		writeBooruError(w, 400, "review", "Review pages support at most 50 posts.")
		return
	}
	ids := make([]string, len(input.Posts))
	seen := map[string]bool{}
	for i, post := range input.Posts {
		if !validBooruReviewID(post.ID) || seen[post.ID] || (post.MD5 != "" && !booruReviewMD5.MatchString(post.MD5)) {
			writeBooruError(w, 400, "review", "Invalid post IDs or MD5 values.")
			return
		}
		seen[post.ID] = true
		ids[i] = post.ID
		input.Posts[i].MD5 = strings.ToLower(post.MD5)
	}
	source := rs.reviewSource(w, input.Source, input.Instance)
	if source == nil {
		return
	}
	if rs.repository.BooruReview == nil {
		writeBooruError(w, 503, "review", "Review storage is unavailable.")
		return
	}
	var states []models.BooruReviewState
	err := rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		states, err = rs.repository.BooruReview.Read(ctx, source.Instance(), ids)
		return err
	})
	if err != nil {
		writeBooruError(w, 500, "review", "Cannot load review state.")
		return
	}
	rows := make([]booruReviewItem, len(states))
	for i, state := range states {
		rows[i] = booruReviewItem{BooruReviewState: state, Matches: []booruReviewMatch{}, MatchStatus: "no-remote-md5"}
	}
	rs.matchBooruLocal(r.Context(), input.Posts, rows)
	writeVisualSimilarityJSON(w, rows)
}

func (rs booruBrowserRoutes) UpdateReview(w http.ResponseWriter, r *http.Request) {
	var input booruReviewUpdate
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if len(input.Changes) < 1 || len(input.Changes) > 50 {
		writeBooruError(w, 400, "review", "Select between 1 and 50 posts.")
		return
	}
	seen := map[string]bool{}
	for _, change := range input.Changes {
		if !validBooruReviewID(change.PostID) || seen[change.PostID] || change.Version < 0 || (change.Ignored == nil && change.Downloaded == nil) {
			writeBooruError(w, 400, "review", "Invalid review changes.")
			return
		}
		seen[change.PostID] = true
	}
	source := rs.reviewSource(w, input.Source, input.Instance)
	if source == nil {
		return
	}
	if rs.repository.BooruReview == nil {
		writeBooruError(w, 503, "review", "Review storage is unavailable.")
		return
	}
	var states []models.BooruReviewState
	err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		var err error
		states, err = rs.repository.BooruReview.Update(ctx, models.BooruReviewProvider{Instance: source.Instance(), SourceID: source.ID, Engine: source.Engine, URL: source.URL}, input.Changes)
		return err
	})
	if errors.Is(err, models.ErrBooruReviewConflict) {
		writeBooruError(w, 409, "conflict", "Review state changed in another request. Nothing was changed; reload review status and try again.")
		return
	}
	if err != nil {
		writeBooruError(w, 500, "review", "Cannot save review state. Nothing was changed.")
		return
	}
	writeVisualSimilarityJSON(w, states)
}

func reviewCheckedAt() string { return time.Now().UTC().Format(time.RFC3339Nano) }
