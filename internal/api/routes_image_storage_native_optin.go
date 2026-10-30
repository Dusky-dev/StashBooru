package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// storageShadowNativeRequest is an explicit, per-request read-only trial of
// archived bytes through /image/{id}/image and /thumbnail. It does NOT enable
// archived serving for any ordinary request, update the catalogue or bypass
// the original-backed proof in storageShadowServe.
func storageShadowNativeRequest(r *http.Request, imageID int) (*http.Request, bool, error) {
	parsed := r.URL.Query()
	_, candidatePresent := parsed["storageShadowCandidate"]
	_, stackPresent := parsed["storageShadowStack"]
	if !candidatePresent && !stackPresent {
		return nil, false, nil
	}
	strict, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(strict) != 2 ||
		len(strict["storageShadowCandidate"]) != 1 ||
		len(strict["storageShadowStack"]) != 1 {
		return nil, true, fmt.Errorf("expected exactly storageShadowCandidate and storageShadowStack once")
	}
	candidate := strict.Get("storageShadowCandidate")
	if !storageCandidateIDPattern.MatchString(candidate) {
		return nil, true, fmt.Errorf("invalid saved candidate identity")
	}
	stackString := strict.Get("storageShadowStack")
	stackID, err := strconv.Atoi(stackString)
	if err != nil || stackID <= 0 || strconv.Itoa(stackID) != stackString || imageID <= 0 {
		return nil, true, fmt.Errorf("native packed preview requires canonical numeric image and stack IDs")
	}
	// Work with a request clone only; the incoming URL is never mutated.
	// The existing shadow handler does all trusted catalogue/backup/archive
	// validation and source byte comparisons, without ever replacing an original.
	shadow := r.Clone(r.Context())
	clonedURL := *r.URL
	clonedURL.RawQuery = url.Values{
		"candidateID": {candidate},
		"stackID":     {stackString},
		"imageID":     {strconv.Itoa(imageID)},
	}.Encode()
	shadow.URL = &clonedURL
	return shadow, true, nil
}
