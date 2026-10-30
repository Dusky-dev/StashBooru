package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStorageShadowRejectsUnsafeAndUnknownArguments(t *testing.T) {
	good := "candidateID=stack-51-delta-0123456789abcdef01234567&stackID=51&imageID=511"
	request := httptest.NewRequest("GET", "/image/storage-shadow?"+good, nil)
	got, err := storageShadowParseRequest(request)
	if err != nil || got.StackID != 51 || got.ImageID != 511 {
		t.Fatalf("valid request refused: %+v: %v", got, err)
	}
	for _, query := range []string{
		"",
		"candidateID=../../secret&stackID=51&imageID=511",
		"candidateID=stack-51-delta-0123456789abcdef01234567&stackID=0&imageID=511",
		"candidateID=stack-51-delta-0123456789abcdef01234567&stackID=51&imageID=0",
		good + "&archive=%2Fetc%2Fpasswd",
		good + "&candidateID=stack-51-delta-0123456789abcdef01234567",
	} {
		if _, err := storageShadowParseRequest(httptest.NewRequest("GET", "/image/storage-shadow?"+query, nil)); err == nil {
			t.Fatalf("accepted unsafe shadow request: %s", query)
		}
	}
}

func TestStorageShadowNeverServesNonMembers(t *testing.T) {
	group := preflightTestGroup()
	m, err := storageShadowMember(group, "image:999")
	if err == nil || m.ID != "" {
		t.Fatal("accepted unlisted image identity")
	}
}

func TestShadowInlinePolicyPreventsExecutableImageContent(t *testing.T) {
	types := []struct {
		input  string
		kind   string
		inline bool
	}{
		{input: "<svg xmlns='http://www.w3.org/2000'><script>alert(1)</script></svg>", kind: "application/octet-stream", inline: false},
		{input: "<html><script>alert(1)</script></html>", kind: "application/octet-stream", inline: false},
		{input: "not an image", kind: "application/octet-stream", inline: false},
	}
	for _, item := range types {
		got, inline := storageShadowContentType([]byte(item.input))
		if got != item.kind || inline != item.inline {
			t.Fatalf("wrong safe output policy for %q: %s/%t", strings.TrimSpace(item.input), got, inline)
		}
	}
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0, 0, 0, 0, 0}
	kind, inline := storageShadowContentType(png)
	if kind != "image/png" || !inline {
		t.Fatalf("expected safe PNG inline policy, got %s/%t", kind, inline)
	}
}

func TestStorageShadowThumbnailNeverAcceptsArbitraryInput(t *testing.T) {
	for _, query := range []string{
		"stackID=0&imageID=1&candidateID=not-a-candidate",
		"stackID=1&imageID=1&candidateID=../../archive",
		"stackID=1&imageID=1&candidateID=stack-1-delta-0123456789abcdef01234567&source=/etc/passwd",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/image/storage-shadow-thumbnail?"+query, nil)
		(imageRoutes{}).StorageShadowThumbnail(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("unsafe packed thumbnail request was not rejected: %s (%d)", query, recorder.Code)
		}
	}
}

func TestStorageShadowThumbnailSizeRespectsVerifiedByteEncoderBounds(t *testing.T) {
	for _, tc := range []struct {
		preferred int
		expected  int
	}{
		{preferred: -1, expected: 320},
		{preferred: 0, expected: 320},
		{preferred: 96, expected: 96},
		{preferred: 320, expected: 320},
		{preferred: 512, expected: 512},
		{preferred: 640, expected: 512},
		{preferred: 2048, expected: 512},
	} {
		if got := storageShadowThumbnailSize(tc.preferred); got != tc.expected {
			t.Errorf("thumbnail bound for %d: got %d, want %d", tc.preferred, got, tc.expected)
		}
	}
}
