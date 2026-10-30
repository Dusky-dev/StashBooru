package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const nativeOptInCandidate = "stack-51-delta-0123456789abcdef01234567"

func TestStorageShadowNativeOptInPreservesOrdinaryImageAndThumbnailQueries(t *testing.T) {
	for _, route := range []string{
		"/image/511/image",
		"/image/511/thumbnail?quality=85",
		"/image/checksum/image?quality=85",
		"/image/511/image?storageShadow=not-a-real-flag",
	} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		rewritten, selected, err := storageShadowNativeRequest(request, 511)
		if err != nil || selected || rewritten != nil {
			t.Fatalf("ordinary native image query intercepted: %s (%v)", route, err)
		}
	}
}

func TestStorageShadowNativeOptInUsesOnlyTrustedImageIdentity(t *testing.T) {
	raw := "/image/511/image?storageShadowCandidate=" + nativeOptInCandidate +
		"&storageShadowStack=51"
	req := httptest.NewRequest(http.MethodGet, raw, nil)
	originalRaw := req.URL.RawQuery
	rewritten, selected, err := storageShadowNativeRequest(req, 511)
	if err != nil || !selected || rewritten == nil {
		t.Fatalf("valid opt-in request rejected: %v", err)
	}
	if req.URL.RawQuery != originalRaw {
		t.Fatal("original native image request mutated")
	}
	if rewritten.URL == req.URL {
		t.Fatal("shadow request reused incoming URL pointer")
	}
	parsed, err := storageShadowParseRequest(rewritten)
	if err != nil || parsed.StackID != 51 || parsed.ImageID != 511 ||
		parsed.CandidateID != nativeOptInCandidate {
		t.Fatalf("native opt-in did not delegate exact IDs to trusted shadow verifier: %+v %v", parsed, err)
	}
	if len(rewritten.URL.Query()) != 3 {
		t.Fatal("native opt-in leaked untrusted query arguments to the archive reader")
	}

	// The same parser drives the native thumbnail route. It does not write
	// the normal thumbnail cache or change any original-file path.
	req = httptest.NewRequest(http.MethodGet,
		"/image/511/thumbnail?storageShadowStack=51&storageShadowCandidate="+nativeOptInCandidate, nil)
	rewritten, selected, err = storageShadowNativeRequest(req, 511)
	if err != nil || !selected || rewritten == nil {
		t.Fatalf("native thumbnail opt-in rejected: %v", err)
	}
	if _, err := storageShadowParseRequest(rewritten); err != nil {
		t.Fatalf("thumbnail did not retain existing shadow validation: %v", err)
	}
}

func TestStorageShadowNativeOptInRejectsMalformedOrAmbiguousRequests(t *testing.T) {
	valid := "storageShadowCandidate=" + nativeOptInCandidate + "&storageShadowStack=51"
	cases := []string{
		"storageShadowCandidate=" + nativeOptInCandidate,
		"storageShadowStack=51",
		valid + "&storageShadowStack=51",
		valid + "&storageShadowCandidate=" + nativeOptInCandidate,
		valid + "&archive=/etc/shadow",
		"storageShadowCandidate=../../private&storageShadowStack=51",
		"storageShadowCandidate=&storageShadowStack=51",
		"storageShadowCandidate=" + nativeOptInCandidate + "&storageShadowStack=0",
		"storageShadowCandidate=" + nativeOptInCandidate + "&storageShadowStack=051",
		"storageShadowCandidate=" + nativeOptInCandidate + "&storageShadowStack=-1",
		"storageShadowCandidate=" + nativeOptInCandidate + "&storageShadowStack=bad",
		valid + "&%ZZ=malformed",
	}
	for _, raw := range cases {
		req := httptest.NewRequest(http.MethodGet, "/image/511/image?"+raw, nil)
		rewritten, selected, err := storageShadowNativeRequest(req, 511)
		if err == nil || !selected || rewritten != nil {
			t.Fatalf("accepted unsafe native opt-in query %q: selected=%t error=%v", raw, selected, err)
		}
	}
	for _, id := range []int{-1, 0} {
		req := httptest.NewRequest(http.MethodGet, "/image/checksum/image?"+valid, nil)
		_, selected, err := storageShadowNativeRequest(req, id)
		if err == nil || !selected {
			t.Fatal("accepted candidate override without canonical numeric image ID")
		}
	}
}

func TestStorageShadowNativeOptInDoesNotTrustClientImageID(t *testing.T) {
	query := url.Values{
		"storageShadowCandidate": {nativeOptInCandidate},
		"storageShadowStack":     {"51"},
		"imageID":                {"9999"},
	}
	req := httptest.NewRequest(http.MethodGet, "/image/511/image?"+query.Encode(), nil)
	_, selected, err := storageShadowNativeRequest(req, 511)
	if !selected || err == nil {
		t.Fatal("native opt-in accepted browser-supplied image ID")
	}
}
