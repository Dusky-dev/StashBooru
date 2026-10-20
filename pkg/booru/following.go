package booru

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type FollowPage struct {
	Posts   []Post
	Cursor  string
	HasMore bool
}

type followCursor struct {
	Initialized bool   `json:"initialized"`
	Checkpoint  uint64 `json:"checkpoint,string"`
	High        uint64 `json:"high,string"`
	Before      uint64 `json:"before,string"`
}

var followQueryControls = regexp.MustCompile(`(?i)(^|[\s{}()])[-~]?(order|limit|random|randseed|page|offset):`)

func FollowQuery(kind, target string) error {
	if strings.TrimSpace(target) == "" || ValidateQuery(target, 1, 50) != nil || strings.ContainsFunc(target, unicode.IsControl) {
		return &Error{Code: "unsupported-query", Message: "Enter a nonempty follow target without control characters."}
	}
	if kind == "query" {
		if followQueryControls.MatchString(target) {
			return &Error{Code: "unsupported-query", Message: "Feed queries cannot contain order, limit, random, randseed, page or offset controls. Keep the tag predicates and exclusions; the feed uses ID order."}
		}
		return nil
	}
	if kind != "artist" && kind != "character" && kind != "copyright" {
		return &Error{Code: "unsupported-query", Message: "Unsupported follow target type."}
	}
	if strings.ContainsFunc(target, unicode.IsSpace) || strings.ContainsAny(target, ":*?{}|\"\\") || strings.HasPrefix(target, "-") || strings.HasPrefix(target, "~") {
		return &Error{Code: "unsupported-query", Message: "Use one exact provider tag for an Artist, Character or Copyright follow. Use Query for a compound search."}
	}
	return nil
}

// FollowPage uses Danbooru's sequential page=bID contract, which forces ID
// descending independently of the tag predicate. No tag or exclusion is added,
// removed or rewritten. See help:common_url_parameters on Danbooru.
func (c *Client) FollowPage(ctx context.Context, source Source, kind, target, rawCursor string) (FollowPage, error) {
	if source.Validate() != nil || source.Engine != "danbooru" {
		return FollowPage{}, &Error{Code: "unsupported-source", Message: "Feed sync currently supports Danbooru-compatible sources. Other sources are not supported yet."}
	}
	if err := FollowQuery(kind, target); err != nil {
		return FollowPage{}, err
	}
	var state followCursor
	if len(rawCursor) > 4096 || (rawCursor != "" && json.Unmarshal([]byte(rawCursor), &state) != nil) || state.Checkpoint >= math.MaxInt64 ||
		(state.Before != 0 && (!state.Initialized || state.High <= state.Checkpoint || state.Before > state.High || state.Before <= state.Checkpoint)) {
		return FollowPage{}, &Error{Code: "cursor", Message: "Stored feed cursor is invalid. No checkpoint was advanced."}
	}
	before := uint64(math.MaxInt64)
	if state.Before != 0 {
		before = state.Before
	}
	req, err := source.request(ctx, target, 1, 50)
	if err != nil {
		return FollowPage{}, &Error{Code: "configuration", Message: "Cannot construct feed request."}
	}
	params := req.URL.Query()
	params.Set("page", "b"+strconv.FormatUint(before, 10))
	req.URL.RawQuery = params.Encode()
	data, err := c.fetchJSON(ctx, source, req, responseLimit)
	if err != nil {
		return FollowPage{}, err
	}
	result, err := parse(source, data, 1, 50)
	if err != nil {
		return FollowPage{}, &Error{Code: "invalid-response", Message: "Invalid feed page. No checkpoint was advanced."}
	}
	posts := []Post{}
	previous := before
	boundary := false
	for _, post := range result.Posts {
		id, err := strconv.ParseUint(post.ID, 10, 63)
		if err != nil || id == 0 || id >= previous || (state.High != 0 && id > state.High) {
			return FollowPage{}, &Error{Code: "invalid-response", Message: "Source did not honor sequential ID pagination. No checkpoint was advanced."}
		}
		previous = id
		if id <= state.Checkpoint {
			boundary = true
			continue
		}
		posts = append(posts, post)
	}
	if state.High == 0 && len(posts) > 0 {
		state.High, _ = strconv.ParseUint(posts[0].ID, 10, 63)
	}
	// Bootstrap is an explicit recent window, not an unbounded historical crawl.
	// Subsequent checks exhaust new IDs, one durable page per request.
	more := state.Initialized && result.HasMore && !boundary
	if more {
		state.Before = previous
	} else {
		state.Checkpoint = max(state.Checkpoint, state.High)
		state.High, state.Before = 0, 0
	}
	state.Initialized = true
	cursor, err := json.Marshal(state)
	if err != nil {
		return FollowPage{}, err
	}
	return FollowPage{Posts: posts, Cursor: string(cursor), HasMore: more}, nil
}
