package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stashapp/stash/pkg/booru"
)

const booruDisplayMaxLimit = 250
const booruFillRequestBudget = 32

type booruPageFilters struct {
	media       string
	hideAI      bool
	hideBlocked bool
	hideIgnored bool
	excludeIgnored func(context.Context, []booru.Post) ([]booru.Post, error)
}

func (f booruPageFilters) keep(post booru.Post) bool {
	if f.media != "" && booruPostMedia(post) != f.media {
		return false
	}
	if f.hideAI {
		for _, tag := range post.Tags {
			if strings.ToLower(strings.NewReplacer("-", "_", " ", "_").Replace(tag.Name)) == "ai_generated" {
				return false
			}
		}
	}
	return true
}

func booruPostMedia(post booru.Post) string {
	switch strings.ToLower(post.Format) {
	case "mp4", "webm", "mkv", "mov", "avi", "m4v":
		return "videos"
	case "jpg", "jpeg", "png", "gif", "apng", "webp", "avif", "jxl", "bmp", "tiff":
		return "images"
	}
	return "unknown"
}

// Fill a virtual page after filtering, never repeating borrowed posts on the
// next page. Providers retain their original 50-post request limit.
func fillBooruPage(ctx context.Context, source booru.Source, page, limit int, filters booruPageFilters, load func(context.Context, int, int) (booru.Result, error)) (booru.Result, error) {
	if page < 1 || page > booru.MaxPage || limit < 1 || limit > booruDisplayMaxLimit {
		return booru.Result{}, &booru.Error{Code: "invalid-page", Message: "Page must be 1–750 and displayed page size 1–250."}
	}
	firstIndex := (page - 1) * limit
	upstreamPage, eligible := 1, 0
	if (!filters.hideBlocked || len(source.BlacklistTags) == 0) && filters.media == "" && !filters.hideAI && !filters.hideIgnored {
		upstreamPage = firstIndex/booru.MaxLimit + 1
		eligible = (upstreamPage - 1) * booru.MaxLimit
	}
	if upstreamPage > booru.MaxPage {
		return booru.Result{SourceID: source.ID, Page: page, Limit: limit, Posts: []booru.Post{}}, nil
	}
	out := booru.Result{SourceID: source.ID, Page: page, Limit: limit, Posts: make([]booru.Post, 0, limit)}
	for attempts := 0; attempts < booruFillRequestBudget && upstreamPage <= booru.MaxPage; attempts++ {
		if err := ctx.Err(); err != nil {
			return booru.Result{}, err
		}
		batch, err := load(ctx, upstreamPage, booru.MaxLimit)
		if err != nil {
			return booru.Result{}, err
		}
		out.FetchedAt = batch.FetchedAt
		if out.SearchNotice == "" {
			out.SearchNotice = batch.SearchNotice
		}
		if filters.hideBlocked {
			batch = source.FilterBlacklisted(batch)
			out.BlacklistedCount += batch.BlacklistedCount
		}
		if filters.hideIgnored && filters.excludeIgnored != nil && len(batch.Posts) > 0 {
			batch.Posts, err = filters.excludeIgnored(ctx, batch.Posts)
			if err != nil {
				return booru.Result{}, err
			}
		}
		for i, post := range batch.Posts {
			if !filters.keep(post) {
				continue
			}
			if eligible >= firstIndex && len(out.Posts) < limit {
				out.Posts = append(out.Posts, post)
			}
			eligible++
			if len(out.Posts) == limit {
				out.HasMore = batch.HasMore
				for _, remaining := range batch.Posts[i+1:] {
					if filters.keep(remaining) {
						out.HasMore = true
						break
					}
				}
				return out, nil
			}
		}
		if !batch.HasMore {
			return out, nil
		}
		upstreamPage++
	}
	return booru.Result{}, &booru.Error{Code: "page-too-deep", Message: fmt.Sprintf("Page %d requires scanning more than %d provider pages. Choose a smaller page or narrow the filters.", page, booruFillRequestBudget)}
}

// Filter saved ignored review records before counting visible page slots.
func (rs booruBrowserRoutes) booruUnignored(ctx context.Context, source booru.Source, posts []booru.Post) ([]booru.Post, error) {
	if rs.repository.BooruReview == nil {
		return posts, nil
	}
	ids := make([]string, len(posts))
	for i := range posts {
		ids[i] = posts[i].ID
	}
	var ignored map[string]bool
	err := rs.repository.WithReadTxn(ctx, func(tx context.Context) error {
		states, err := rs.repository.BooruReview.Read(tx, source.Instance(), ids)
		if err != nil {
			return err
		}
		ignored = make(map[string]bool, len(states))
		for _, state := range states {
			if state.IgnoredAt != "" {
				ignored[state.PostID] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	kept := make([]booru.Post, 0, len(posts))
	for _, post := range posts {
		if !ignored[post.ID] {
			kept = append(kept, post)
		}
	}
	return kept, nil
}

// Filled search params are kept separate from regular provider pagination, so
// legacy API consumers continue to use the 1–50 upstream page size.
func booruPageRequestFilters(q url.Values) (bool, booruPageFilters, bool) {
	fill := q.Get("fill") == "1"
	if (q.Has("fill") && (!fill || len(q["fill"]) != 1)) || len(q["media"]) > 1 || len(q["ai"]) > 1 || len(q["ignored"]) > 1 || len(q["limit"]) > 1 || len(q["page"]) > 1 {
		return false, booruPageFilters{}, false
	}
	f := booruPageFilters{media: q.Get("media"), hideAI: q.Get("ai") == "hide", hideIgnored: q.Get("ignored") == "hide"}
	if f.media != "" && f.media != "images" && f.media != "videos" {
		return false, f, false
	}
	if (q.Has("ai") && !f.hideAI) || (q.Has("ignored") && !f.hideIgnored) {
		return false, f, false
	}
	if !fill && (f.media != "" || f.hideAI || f.hideIgnored) {
		return false, f, false
	}
	return fill, f, true
}
