package booru

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"slices"
	"strconv"
)

const unsupportedFollowSource = "Feed sync supports Danbooru, Gelbooru, Rule34, Paheal and Pawchive creators. This follow stays saved."

// followPageCap bounds one check. A follow that has more new posts than this
// is caught up to the newest posts; older new posts are skipped and said so.
const followPageCap = 20

// FollowSyncEngine reports whether feed checks can fetch new posts.
func FollowSyncEngine(engine string) bool {
	switch engine {
	case "danbooru", "gelbooru", "shimmie", "pawchive":
		return true
	}
	return false
}

var archiveCreatorTarget = regexp.MustCompile(`^creator:[A-Za-z0-9_-]{1,64}:[A-Za-z0-9_.-]{1,128}$`)

// FollowQueryFor validates a follow target for one engine. Pawchive follows
// one creator, written creator:service:id; other engines use tags.
func FollowQueryFor(engine, kind, target string) error {
	if engine == "pawchive" {
		if kind != "artist" || !archiveCreatorTarget.MatchString(target) {
			return &Error{Code: "unsupported-query", Message: "Pawchive follows one creator: choose Artist and use creator:service:id."}
		}
		return nil
	}
	return FollowQuery(kind, target)
}

type pagedFollowCursor struct {
	followCursor
	Page int `json:"page,omitempty"`
}

func followPageNotice(capped bool) string {
	if capped {
		return "Stopped after " + strconv.Itoa(followPageCap) + " pages of new posts; older new posts were skipped."
	}
	return ""
}

// followNumericPages pages a newest-first search by page number. IDs are
// numeric and descending, so the checkpoint is the highest ID seen; posts
// pushed down onto a later page by newer uploads are skipped as repeats.
func (c *Client) followNumericPages(ctx context.Context, source Source, target, rawCursor string) (FollowPage, error) {
	var state pagedFollowCursor
	if len(rawCursor) > 4096 || (rawCursor != "" && json.Unmarshal([]byte(rawCursor), &state) != nil) || state.Checkpoint >= math.MaxInt64 ||
		state.Page < 0 || state.Page > followPageCap ||
		(state.Before != 0 && (!state.Initialized || state.Page < 2 || state.High <= state.Checkpoint || state.Before > state.High || state.Before <= state.Checkpoint)) {
		return FollowPage{}, &Error{Code: "cursor", Message: "Stored feed cursor is invalid. No checkpoint was advanced."}
	}
	page := max(state.Page, 1)
	result, err := c.Search(ctx, source, target, page, 50)
	if err != nil {
		return FollowPage{}, err
	}
	posts := []Post{}
	previous := uint64(math.MaxInt64)
	boundary := false
	for _, post := range result.Posts {
		id, err := strconv.ParseUint(post.ID, 10, 63)
		if err != nil || id == 0 || id >= previous {
			return FollowPage{}, &Error{Code: "invalid-response", Message: "Source did not return newest posts first. Remove any sort: or order: tag from the follow. No checkpoint was advanced."}
		}
		previous = id
		if id <= state.Checkpoint {
			boundary = true
			break
		}
		if (state.Before != 0 && id >= state.Before) || (state.High != 0 && id > state.High) {
			continue // repeat pushed down by newer posts, or newer than this run
		}
		posts = append(posts, post)
	}
	if state.High == 0 && len(posts) > 0 {
		state.High, _ = strconv.ParseUint(posts[0].ID, 10, 63)
	}
	more := state.Initialized && result.HasMore && !boundary && len(result.Posts) > 0
	capped := more && page >= followPageCap
	if more && !capped {
		state.Before, state.Page = previous, page+1
	} else {
		more = false
		state.Checkpoint = max(state.Checkpoint, state.High)
		state.High, state.Before, state.Page = 0, 0, 0
	}
	state.Initialized = true
	cursor, err := json.Marshal(state)
	if err != nil {
		return FollowPage{}, err
	}
	return FollowPage{Posts: posts, Cursor: string(cursor), HasMore: more, Notice: followPageNotice(capped)}, nil
}

// Archive creator posts have no ordered numeric IDs. The cursor remembers
// the newest post IDs of the last finished check and stops at the first one
// it sees again. Posts the archive imports later with an older date are not
// picked up.
type archiveFollowCursor struct {
	Initialized bool     `json:"initialized"`
	Recent      []string `json:"recent,omitempty"`
	Pending     []string `json:"pending,omitempty"`
	Page        int      `json:"page,omitempty"`
}

const (
	archiveRecentIDs  = 40
	archivePendingIDs = 20
)

func (c *Client) followArchiveCreator(ctx context.Context, source Source, target, rawCursor string) (FollowPage, error) {
	var state archiveFollowCursor
	if len(rawCursor) > 4096 || (rawCursor != "" && json.Unmarshal([]byte(rawCursor), &state) != nil) ||
		len(state.Recent) > archiveRecentIDs || len(state.Pending) > archivePendingIDs || state.Page < 0 || state.Page > followPageCap ||
		(state.Page > 1 && (!state.Initialized || len(state.Pending) == 0)) {
		return FollowPage{}, &Error{Code: "cursor", Message: "Stored feed cursor is invalid. No checkpoint was advanced."}
	}
	page := max(state.Page, 1)
	result, err := c.Search(ctx, source, target, page, 50)
	if err != nil {
		return FollowPage{}, err
	}
	posts := []Post{}
	ids := []string{}
	boundary := false
	for _, post := range result.Posts {
		if post.ID == "" || len(post.ID) > 64 {
			return FollowPage{}, &Error{Code: "invalid-response", Message: "Invalid feed page. No checkpoint was advanced."}
		}
		if slices.Contains(state.Recent, post.ID) {
			boundary = true
			break
		}
		ids = append(ids, post.ID)
		if slices.Contains(state.Pending, post.ID) {
			continue // repeat pushed down by newer posts
		}
		posts = append(posts, post)
	}
	if page == 1 {
		state.Pending = ids[:min(len(ids), archivePendingIDs)]
	}
	more := state.Initialized && result.HasMore && !boundary && len(result.Posts) > 0
	capped := more && page >= followPageCap
	if more && !capped {
		state.Page = page + 1
	} else {
		more = false
		recent := slices.Clone(state.Pending)
		for _, id := range state.Recent {
			if len(recent) >= archiveRecentIDs {
				break
			}
			if !slices.Contains(recent, id) {
				recent = append(recent, id)
			}
		}
		state.Recent, state.Pending, state.Page = recent, nil, 0
	}
	state.Initialized = true
	cursor, err := json.Marshal(state)
	if err != nil {
		return FollowPage{}, err
	}
	return FollowPage{Posts: posts, Cursor: string(cursor), HasMore: more, Notice: followPageNotice(capped)}, nil
}
