package api

import (
	"strings"
	"testing"
)

// A JPEG selected from the browser's file picker is rendered from a blob:
// object URL before it is uploaded. CSP wildcard sources do not cover blob:.
func TestPageImageSourcePolicyAllowsLocalUploadPreviews(t *testing.T) {
	sources := strings.Fields(pageImageSourcePolicy)
	for _, required := range []string{"blob:", "data:", "*"} {
		present := false
		for _, actual := range sources {
			if actual == required {
				present = true
				break
			}
		}
		if !present {
			t.Fatalf("image CSP %q must explicitly include %q for upload previews", pageImageSourcePolicy, required)
		}
	}
}
