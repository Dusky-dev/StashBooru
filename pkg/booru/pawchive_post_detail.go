package booru

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// PawchivePostIdentity only accepts the canonical URL of a post already
// supplied by the configured Pawchive source. No client-supplied URL is fetched.
func pawchivePostIdentity(source Source, post Post) (string, string, string, bool) {
	if source.Engine != "pawchive" || post.SourceID != source.ID {
		return "", "", "", false
	}
	u, err := url.Parse(post.PostURL)
	base, baseErr := url.Parse(source.URL)
	if err != nil || baseErr != nil || u.Scheme != "https" ||
		u.Host != base.Host || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 5 || parts[1] != "user" || parts[3] != "post" ||
		!safeProviderPart(parts[0]) || !safeProviderPart(parts[2]) || !safeProviderPart(parts[4]) {
		return "", "", "", false
	}
	identity := parts[0] + "\x00" + parts[2] + "\x00" + parts[4]
	if post.ID != "" && stableAssetID("a", identity) != post.ID {
		return "", "", "", false
	}
	return parts[0], parts[2], parts[4], true
}

// PawchivePostDetail fetches exactly one canonical post on demand, when its
// inspector is opened. Archive lists can omit content or embeds; the detail
// endpoint is authoritative for links in that post.
func (c *Client) PawchivePostDetail(ctx context.Context, source Source, post Post) (Post, error) {
	service, user, id, ok := pawchivePostIdentity(source, post)
	if !ok {
		return Post{}, fmt.Errorf("invalid Pawchive post identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	endpoint := "/api/v1/" + url.PathEscape(service) + "/user/" +
		url.PathEscape(user) + "/post/" + url.PathEscape(id)
	data, err := c.extendedRequest(ctx, source, endpoint, nil)
	if err != nil {
		return Post{}, err
	}
	var raw object
	if json.Unmarshal(data, &raw) != nil {
		return Post{}, fmt.Errorf("pawchive post details are invalid")
	}
	if nested := child(raw["post"]); nested != nil {
		// Pawchive/Kemono versions differ: some return the post directly,
		// others return an object with post and attachments.
		if len(nested["attachments"]) == 0 && len(raw["attachments"]) > 0 {
			nested["attachments"] = raw["attachments"]
		}
		raw = nested
	}
	detailed, err := parseArchivePost(source, raw)
	if err != nil || detailed.ID != post.ID {
		return Post{}, fmt.Errorf("pawchive returned a different or invalid post")
	}
	return detailed, nil
}
