package config

import "testing"

func TestExperimentalQualityFallbackAndBounds(t *testing.T) {
	for _, tc := range []struct {
		value interface{}
		want int
	}{
		{nil, 70}, {"garbage", 70}, {float64(-1), 70},
		{float64(101), 70}, {float64(35), 35}, {95, 95},
	} {
		got := experimentalQuality(map[string]interface{}{"thumbnailImageQuality": tc.value}, "thumbnailImageQuality", 70)
		if got != tc.want {
			t.Errorf("quality(%v) = %d; want %d", tc.value, got, tc.want)
		}
	}
}
