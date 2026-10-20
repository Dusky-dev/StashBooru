package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

type followingStoredPost struct {
	Post     booru.Post `json:"post"`
	Original string     `json:"original"`
}

// Shared feed storage uses opaque payloads/cursors. This first adapter projects
// booru metadata; future account providers can implement their own projection.
type followingContent struct {
	Title     string      `json:"title"`
	URL       string      `json:"url,omitempty"`
	Thumbnail string      `json:"thumbnail,omitempty"`
	BrowseURL string      `json:"browseURL,omitempty"`
	Rating    string      `json:"rating,omitempty"`
	Tags      []booru.Tag `json:"tags,omitempty"`
}

type followingPostView struct {
	models.FollowingPost
	Content followingContent `json:"content"`
}

func (rs booruBrowserRoutes) FollowingFeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if rs.repository.Following == nil {
		writeBooruError(w, 503, "following", "Following storage is unavailable.")
		return
	}
	q := r.URL.Query()
	for _, key := range []string{"unseen", "history", "ignored"} {
		if value := q.Get(key); value != "" && value != "1" && value != "0" {
			writeBooruError(w, 400, "following", "Invalid feed filter.")
			return
		}
	}
	if value := q.Get("downloaded"); value != "" && value != "hide" {
		writeBooruError(w, 400, "following", "Invalid download-history filter.")
		return
	}
	filter := models.FollowingFilter{Limit: 25, FollowID: q.Get("follow"), Unseen: q.Get("unseen") == "1", History: q.Get("history") == "1", ShowIgnored: q.Get("ignored") == "1", HideDownloaded: q.Get("downloaded") == "hide"}
	var err error
	if q.Get("before") != "" {
		filter.Before, err = strconv.ParseInt(q.Get("before"), 10, 64)
	}
	if err != nil || filter.Before < 0 || (filter.FollowID != "" && !booruFollowID.MatchString(filter.FollowID)) {
		writeBooruError(w, 400, "following", "Invalid feed cursor or follow ID.")
		return
	}
	if q.Get("limit") != "" {
		filter.Limit, err = strconv.Atoi(q.Get("limit"))
	}
	if err != nil || filter.Limit < 1 || filter.Limit > 50 {
		writeBooruError(w, 400, "following", "Feed page size must be 1–50.")
		return
	}
	limit := filter.Limit
	filter.Limit++
	var rows []models.FollowingPost
	err = rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		rows, err = rs.repository.Following.List(ctx, filter)
		return err
	})
	if err != nil {
		writeBooruError(w, 500, "following", "Cannot load the saved feed.")
		return
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	// Source settings failures never make cached decisions/history inaccessible.
	sources, _ := rs.sources()
	items := make([]followingPostView, 0, len(rows))
	for _, row := range rows {
		content := followingContent{Title: "Post " + row.PostID}
		var stored followingStoredPost
		if json.Unmarshal(row.Payload, &stored) == nil {
			content.URL, content.Rating, content.Tags = stored.Post.PostURL, stored.Post.Rating, stored.Post.Tags
			if stored.Post.Title != "" {
				content.Title = stored.Post.Title
			}
			for _, source := range sources {
				if source.Instance() != row.Instance {
					continue
				}
				stored.Post.OriginalURL = stored.Original
				post := publicBooruPost(source, stored.Post)
				content.Thumbnail = post.PreviewURL
				params := url.Values{"source": {source.ID}, "q": {"id:" + row.PostID}, "inspect": {source.ID + ":" + row.PostID}}
				content.BrowseURL = "/booru?" + params.Encode()
				break
			}
		}
		items = append(items, followingPostView{FollowingPost: row, Content: content})
	}
	before := ""
	if more && len(rows) > 0 {
		before = strconv.FormatInt(rows[len(rows)-1].Sequence, 10)
	}
	writeVisualSimilarityJSON(w, map[string]any{"items": items, "next": before})
}

func (rs booruBrowserRoutes) SyncFollow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if !booruFollowID.MatchString(input.ID) || input.Version < 1 {
		writeBooruError(w, 400, "following", "Select a follow and its current version.")
		return
	}
	if rs.repository.Following == nil || rs.repository.BooruFollow == nil {
		writeBooruError(w, 503, "following", "Following storage is unavailable.")
		return
	}
	var follow *models.BooruFollow
	var state models.FollowingCheckpoint
	err := rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		follow, err = rs.repository.BooruFollow.Find(ctx, input.ID)
		if err != nil {
			return err
		}
		if follow == nil || !follow.Active || follow.Version != input.Version {
			return models.ErrBooruFollowConflict
		}
		state, err = rs.repository.Following.Checkpoint(ctx, follow.ID, follow.QueryRevision)
		return err
	})
	if err != nil {
		followingError(w, err)
		return
	}
	fail := func(err error) {
		if r.Context().Err() == nil {
			message := "Feed check failed; retry to resume."
			var provider *booru.Error
			if errors.As(err, &provider) {
				message = provider.Message
			}
			if errors.Is(err, models.ErrFollowingLimit) {
				message = "Snapshot cache is full. Remove seen snapshots and retry to resume."
			}
			if errors.Is(err, errBooruFollowSource) {
				message = "Source changed or is unavailable. Restore it and retry to resume."
			}
			if len(message) > 1024 {
				message = "Feed check failed; retry to resume."
			}
			// Diagnostics are best effort. A stale failure must not overwrite a
			// newer success or replace the primary request error.
			_ = rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
				return rs.repository.Following.RecordError(ctx, *follow, state.Version, message)
			})
		}
		followingError(w, err)
	}
	sources, err := rs.sources()
	if err != nil {
		fail(errBooruFollowSource)
		return
	}
	view := resolveBooruFollow(*follow, sources)
	var source booru.Source
	for _, candidate := range sources {
		if view.Status == "available" && candidate.ID == view.ResolvedSource {
			source = candidate
			break
		}
	}
	if source.ID == "" {
		fail(errBooruFollowSource)
		return
	}
	client, ok := rs.client.(interface {
		FollowPage(context.Context, booru.Source, string, string, string) (booru.FollowPage, error)
	})
	if !ok || source.Engine != "danbooru" {
		writeBooruError(w, 400, "unsupported-source", "Feed sync currently supports Danbooru-compatible sources. This follow remains saved. More sources will be supported later.")
		return
	}
	page, err := client.FollowPage(r.Context(), source, follow.Kind, follow.Target, state.Cursor)
	if err != nil {
		fail(err)
		return
	}
	// Never store ephemeral preview tokens or credentials. Recreate media grants
	// from the current source policy when a cached feed page is read.
	posts := make([]models.FollowingSnapshot, 0, len(page.Posts))
	for i := len(page.Posts) - 1; i >= 0; i-- {
		post := page.Posts[i]
		post.MediaKey, post.DirectPreviewURL, post.DirectSampleURL = "", "", ""
		payload, err := json.Marshal(followingStoredPost{Post: post, Original: post.OriginalURL})
		if err != nil || len(payload) > models.MaxFollowingPayload {
			fail(&booru.Error{Code: "metadata-limit", Message: "A post exceeds the 64 KiB feed metadata limit. No page or checkpoint was saved."})
			return
		}
		posts = append(posts, models.FollowingSnapshot{PostID: post.ID, Payload: payload})
	}
	latest, err := rs.sources()
	unchanged := false
	for _, candidate := range latest {
		if candidate.ID == source.ID && candidate.Instance() == source.Instance() && candidate.APIKey == source.APIKey && candidate.Login == source.Login && candidate.UserID == source.UserID {
			unchanged = true
		}
	}
	if err != nil || !unchanged {
		fail(errBooruFollowSource)
		return
	}
	err = rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		next, err := rs.repository.Following.ApplyPage(ctx, *follow, state.Version, page.Cursor, page.HasMore, posts)
		if err == nil {
			state = next
		}
		return err
	})
	if err != nil {
		fail(err)
		return
	}
	notice := fmt.Sprintf("Saved %d post matches. Check complete.", len(posts))
	if page.HasMore {
		notice = fmt.Sprintf("Saved %d post matches. More new posts remain; use Continue check.", len(posts))
	}
	writeVisualSimilarityJSON(w, map[string]any{"checkpoint": state, "matched": len(posts), "notice": notice})
}

func followingError(w http.ResponseWriter, err error) {
	var provider *booru.Error
	switch {
	case errors.Is(err, models.ErrBooruFollowConflict):
		writeBooruError(w, 409, "conflict", "Follow, checkpoint or seen decision changed. Nothing was saved; reload and retry.")
	case errors.Is(err, errBooruFollowSource):
		writeBooruError(w, 409, "source-changed", "Source changed or is unavailable. No checkpoint was advanced.")
	case errors.Is(err, models.ErrFollowingLimit):
		writeBooruError(w, 409, "feed-limit", "The 5000-post snapshot cache is full. Mark reviewed posts seen and remove seen snapshots, then retry. No checkpoint was advanced.")
	case errors.Is(err, models.ErrFollowingInvalid):
		writeBooruError(w, 400, "following", "Invalid feed page or seen decision.")
	case errors.As(err, &provider):
		status := 502
		if provider.Code == "unsupported-query" || provider.Code == "unsupported-source" {
			status = 400
		}
		if provider.Code == "rate-limited" {
			status = 429
			w.Header().Set("Retry-After", strconv.Itoa(provider.RetryAfter))
		}
		writeBooruError(w, status, provider.Code, provider.Message)
	default:
		writeBooruError(w, 500, "following", "Cannot save Following state. Nothing was changed.")
	}
}

func (rs booruBrowserRoutes) SetFollowingSeen(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Sequence string `json:"sequence"`
		Version  int    `json:"version"`
		Seen     *bool  `json:"seen"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	sequence, err := strconv.ParseInt(input.Sequence, 10, 64)
	if err != nil || sequence < 1 || input.Version < 0 || input.Seen == nil {
		followingError(w, models.ErrFollowingInvalid)
		return
	}
	if rs.repository.Following == nil {
		writeBooruError(w, 503, "following", "Following storage is unavailable.")
		return
	}
	var version int
	var at string
	err = rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		var err error
		version, at, err = rs.repository.Following.SetSeen(ctx, sequence, input.Version, *input.Seen)
		return err
	})
	if err != nil {
		followingError(w, err)
		return
	}
	writeVisualSimilarityJSON(w, map[string]any{"version": version, "seenAt": at})
}

func (rs booruBrowserRoutes) PruneFollowingSeen(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
	}
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if input.Action != "remove-seen-snapshots" {
		followingError(w, models.ErrFollowingInvalid)
		return
	}
	if rs.repository.Following == nil {
		writeBooruError(w, 503, "following", "Following storage is unavailable.")
		return
	}
	var count int64
	err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		var err error
		count, err = rs.repository.Following.PruneSeen(ctx)
		return err
	})
	if err != nil {
		followingError(w, err)
		return
	}
	writeVisualSimilarityJSON(w, map[string]any{"removed": count, "at": time.Now().UTC()})
}
