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

func TestDecodingSpeedControlsAndDefaults(t *testing.T) {
	for _, tc := range []struct {
		format mediaconvert.Format
		levels int
		def    int
	}{
		{mediaconvert.Format{ID: "ajxl", Controls: []string{"decodingSpeed"}, DecodingSpeedLevels: 4}, 4, 2},
		{mediaconvert.Format{ID: "av1-mp4", Controls: []string{"decodingSpeed"}, DecodingSpeedLevels: 1}, 1, 0},
		{mediaconvert.Format{ID: "ajxl", Controls: []string{"fasterDecoding"}}, 4, 2},
	} {
		if !formatSupportsControl(tc.format, "decodingSpeed") && !formatSupportsControl(tc.format, "fasterDecoding") {
			t.Fatalf("format %s should support decode speed", tc.format.ID)
		}
		if levels := formatDecodingSpeedLevels(tc.format); levels != tc.levels {
			t.Fatalf("format %s has %d levels, want %d", tc.format.ID, levels, tc.levels)
		}
		if value := defaultConversionDecodingSpeed(tc.format); *value != tc.def {
			t.Fatalf("format %s default is %d, want %d", tc.format.ID, *value, tc.def)
		}
	}
	unsupported := mediaconvert.Format{ID: "av1-mp4", Controls: []string{"quality"}, DecodingSpeedLevels: 1}
	if formatSupportsControl(unsupported, "decodingSpeed") {
		t.Fatal("worker must not receive a control it did not advertise")
	}
}

func TestConversionUsesDecodingSpeedDefaults(t *testing.T) {
	useDefaults := true
	useOverride := false
	for _, tc := range []struct {
		name    string
		request conversionRequest
		want    bool
	}{
		{"legacy saved encoding defaults", conversionRequest{UseEncodingDefaults: true}, true},
		{"legacy explicit encoding options", conversionRequest{UseEncodingDefaults: false}, false},
		{"saved decode speed with custom quality", conversionRequest{UseDecodingSpeedDefaults: &useDefaults}, true},
		{"decode speed override with saved quality", conversionRequest{UseEncodingDefaults: true, UseDecodingSpeedDefaults: &useOverride}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := conversionUsesDecodingSpeedDefaults(tc.request); got != tc.want {
				t.Fatalf("conversionUsesDecodingSpeedDefaults() = %v, want %v", got, tc.want)
			}
		})
	}
}
