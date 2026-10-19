package booru

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Account identity and credentials stay server-side; favorites are read-only.
func (s Source) AccountFavoritesAvailable() bool {
	switch s.Engine {
	case "danbooru", "e621":
		return s.Login != "" && s.APIKey != ""
	case "gelbooru":
		return s.UserID != "" && s.APIKey != ""
	}
	return false
}

func (c *Client) AccountFavorites(ctx context.Context, s Source, page, limit int) (Result, error) {
	result := Result{SourceID: s.ID, Page: page, Limit: limit, Posts: []Post{}, FetchedAt: time.Now().UTC()}
	if s.Validate() != nil || ValidateQuery("", page, limit) != nil {
		return result, &Error{Code: "invalid-query", Message: "Invalid favorites source or page."}
	}
	if !s.AccountFavoritesAvailable() {
		return result, &Error{Code: "unsupported-query", Message: "Account favorites require a supported Danbooru, Gelbooru/DAPI or e621 source with its account credentials saved in Settings."}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if s.Engine == "danbooru" {
		// /favorites.json returns relations, not posts. Keep ordered favorites
		// as a single metatag, with no injected query operators.
		if strings.ContainsAny(s.Login, " \t\r\n:\"") {
			return result, &Error{Code: "configuration", Message: "The saved login cannot be used as a native favorites username."}
		}
		return c.Search(ctx, s, "ordfav:"+s.Login, page, limit)
	}
	req, err := s.request(ctx, "", page, limit)
	if err != nil {
		return result, &Error{Code: "configuration", Message: "Cannot construct favorites request."}
	}
	params := req.URL.Query()
	params.Del("tags")
	if s.Engine == "e621" {
		req.URL.Path = strings.TrimSuffix(req.URL.Path, "/posts.json") + "/favorites.json"
	} else {
		params.Set("s", "favorite")
		params.Set("id", s.UserID)
	}
	req.URL.RawQuery = params.Encode()
	data, err := c.fetchJSON(ctx, s, req, responseLimit)
	if err != nil {
		return result, err
	}
	if s.Engine == "e621" {
		ret, err := parse(s, data, page, limit)
		if err != nil {
			return result, extendedParseError()
		}
		ret.FetchedAt = result.FetchedAt
		return ret, nil
	}
	var envelope object
	if json.Unmarshal(data, &envelope) != nil {
		return result, extendedParseError()
	}
	if raw := child(envelope["@attributes"])["count"]; raw != nil {
		total, err := strconv.ParseInt(text(raw), 10, 64)
		if err != nil || total < 0 {
			return result, extendedParseError()
		}
		result.Total = &total
	}
	if envelope["favorite"] == nil && result.Total != nil && int64((page-1)*limit) >= *result.Total {
		return result, nil
	}
	var rows []object
	if json.Unmarshal(envelope["favorite"], &rows) != nil {
		var row object
		if json.Unmarshal(envelope["favorite"], &row) != nil || row == nil {
			return result, extendedParseError()
		}
		rows = []object{row}
	}
	if rows == nil || len(rows) > limit {
		return result, extendedParseError()
	}
	seen := map[string]bool{}
	for _, row := range rows {
		id := text(row["favorite"])
		if !postID.MatchString(id) {
			return result, extendedParseError()
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		postReq, err := s.request(ctx, "", 1, 1)
		if err != nil {
			return result, extendedParseError()
		}
		values := postReq.URL.Query()
		values.Del("tags")
		values.Del("pid")
		values.Set("id", id)
		postReq.URL.RawQuery = values.Encode()
		body, err := c.fetchJSON(ctx, s, postReq, responseLimit)
		if err != nil {
			return result, err
		}
		posts, err := parse(s, body, 1, 1)
		if err != nil {
			return result, extendedParseError()
		}
		if len(posts.Posts) == 1 {
			if posts.Posts[0].ID != id {
				return result, extendedParseError()
			}
			result.Posts = append(result.Posts, posts.Posts[0])
		}
	}
	result.HasMore = len(rows) == limit && page < MaxPage
	if result.Total != nil && int64(page*limit) >= *result.Total {
		result.HasMore = false
	}
	return result, nil
}
