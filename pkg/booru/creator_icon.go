package booru

import (
	"fmt"
	"net/url"
	"strings"
)

// CreatorIconURL is the profile picture of a Pawchive or Kemono creator
// account ("creator:<service>:<id>"): <site>/icons/<service>/<id>. Kemono
// redirects it to its img. host, which is within the source's media hosts.
// A creator without a picture answers 404 (Kemono) or 204 (Pawchive).
func CreatorIconURL(source Source, target string) (string, error) {
	parts := strings.Split(target, ":")
	if !ArchiveCreatorEngine(source.Engine) || len(parts) != 3 || parts[0] != "creator" || !safeProviderPart(parts[1]) || !safeProviderPart(parts[2]) {
		return "", fmt.Errorf("profile pictures are available for Pawchive and Kemono creator accounts")
	}
	raw := strings.TrimRight(source.URL, "/") + "/icons/" + url.PathEscape(parts[1]) + "/" + url.PathEscape(parts[2])
	approved := source.mediaURL(raw)
	if approved == "" {
		return "", fmt.Errorf("the profile picture address is not on the source's media hosts")
	}
	return approved, nil
}
