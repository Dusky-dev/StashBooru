package api

import (
	"testing"

	"github.com/stashapp/stash/pkg/mediaconvert"
)

func TestConversionOutputValidation(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		format     mediaconvert.Format
		valid      bool
	}{
		{"image to JXL", "image", mediaconvert.Format{ID: "jxl", Extension: "jxl", Family: "image", Available: true}, true},
		{"animation to video", "image", mediaconvert.Format{ID: "av1-mp4", Extension: "mp4", Family: "video", Available: true}, true},
		{"scene cannot become still", "scene", mediaconvert.Format{ID: "jxl", Extension: "jxl", Family: "image", Available: true}, false},
		{"worker path injection", "image", mediaconvert.Format{ID: "jxl", Extension: "../../file", Family: "image", Available: true}, false},
		{"worker family mismatch", "scene", mediaconvert.Format{ID: "jxl", Extension: "jxl", Family: "video", Available: true}, false},
		{"unavailable output", "image", mediaconvert.Format{ID: "jxl", Extension: "jxl", Family: "image"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conversionOutputFormat(mediaconvert.Capabilities{Formats: []mediaconvert.Format{tc.format}}, tc.format.ID, tc.kind)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
