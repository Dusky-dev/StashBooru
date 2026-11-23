package booru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrNoCreatorPicture means the account exists but uses the site's default
// avatar, so there is nothing worth setting as an Artist image.
var ErrNoCreatorPicture = errors.New("no profile picture")

// CreatorPictureEngine reports whether connected accounts on the engine can
// supply a profile picture.
func CreatorPictureEngine(engine string) bool {
	return engine == "pixiv" || engine == "twitter" || ArchiveCreatorEngine(engine)
}

// CreatorIconURL is the profile picture of a Pawchive or Kemono creator
// account ("creator:<service>:<id>"): <site>/icons/<service>/<id>. Kemono
// redirects it to its img. host, which is within the source's media hosts.
// A creator without a picture answers 404 (Kemono) or 204 (Pawchive).
func CreatorIconURL(source Source, target string) (string, error) {
	parts := strings.Split(target, ":")
	if !ArchiveCreatorEngine(source.Engine) || len(parts) != 3 || parts[0] != "creator" || !safeProviderPart(parts[1]) || !safeProviderPart(parts[2]) {
		return "", fmt.Errorf("profile pictures are available for Pixiv, Pawchive and Kemono creator accounts")
	}
	raw := strings.TrimRight(source.URL, "/") + "/icons/" + url.PathEscape(parts[1]) + "/" + url.PathEscape(parts[2])
	approved := source.mediaURL(raw)
	if approved == "" {
		return "", fmt.Errorf("the profile picture address is not on the source's media hosts")
	}
	return approved, nil
}

// CreatorIcon resolves the profile picture address of a connected creator
// account. Pixiv ("user:<id>") needs the source's OAuth token for a user
// detail lookup; Pawchive and Kemono use a fixed address.
func (c *Client) CreatorIcon(ctx context.Context, source Source, target string) (string, error) {
	if source.Engine == "twitter" {
		return c.twitterIcon(ctx, source, target)
	}
	if source.Engine != "pixiv" {
		return CreatorIconURL(source, target)
	}
	id := strings.TrimPrefix(target, "user:")
	if !strings.HasPrefix(target, "user:") || !postID.MatchString(id) {
		return "", fmt.Errorf("profile pictures are available for Pixiv user accounts (user:123)")
	}
	if source.APIKey == "" {
		return "", &Error{Code: "authentication", Message: "Pixiv requires an OAuth access token. Add it in this source's API key setting."}
	}
	data, err := c.extendedRequest(ctx, source, "/v1/user/detail", url.Values{"user_id": {id}, "filter": {"for_android"}})
	if err != nil {
		return "", err
	}
	var detail struct {
		User struct {
			ProfileImageURLs struct {
				Medium string `json:"medium"`
			} `json:"profile_image_urls"`
		} `json:"user"`
	}
	if json.Unmarshal(data, &detail) != nil {
		return "", extendedParseError()
	}
	raw := detail.User.ProfileImageURLs.Medium
	// Accounts without an upload show Pixiv's shared no_profile image.
	if raw == "" || strings.Contains(raw, "/no_profile") {
		return "", ErrNoCreatorPicture
	}
	approved := source.mediaURL(raw)
	if approved == "" {
		return "", fmt.Errorf("the profile picture address is not on the source's media hosts")
	}
	return approved, nil
}
