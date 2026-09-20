package api

import (
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/visualembedding"
)

func TestEva02TagPredictionToCamieRating(t *testing.T) {
	tests := []struct {
		name       string
		prediction visualembedding.TagPrediction
		wantName   string
		wantRaw    string
		wantCat    string
		wantOK     bool
	}{
		{
			name:       "general rating becomes safe without general alias",
			prediction: visualembedding.TagPrediction{Name: "general", Category: "meta", Score: 0.9},
			wantName:   "safe",
			wantRaw:    "safe",
			wantCat:    "rating",
			wantOK:     true,
		},
		{
			name:       "explicit rating",
			prediction: visualembedding.TagPrediction{Name: "explicit", Category: "meta", Score: 0.8},
			wantName:   "explicit",
			wantRaw:    "explicit",
			wantCat:    "rating",
			wantOK:     true,
		},
		{
			name:       "future worker rating category",
			prediction: visualembedding.TagPrediction{Name: "questionable", Category: "rating", Score: 0.7},
			wantName:   "questionable",
			wantRaw:    "questionable",
			wantCat:    "rating",
			wantOK:     true,
		},
		{
			name:       "ordinary general tag remains general",
			prediction: visualembedding.TagPrediction{Name: "blue_hair", Category: "general", Score: 0.6},
			wantName:   "blue_hair",
			wantRaw:    "blue_hair",
			wantCat:    "general",
			wantOK:     true,
		},
		{
			name:       "unknown meta tag is rejected",
			prediction: visualembedding.TagPrediction{Name: "some_meta", Category: "meta", Score: 0.5},
			wantOK:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := eva02TagPredictionToCamie(test.prediction)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				return
			}
			if got.Name != test.wantName || got.RawName != test.wantRaw || got.Category != test.wantCat {
				t.Fatalf("prediction = %+v, want name=%q raw=%q category=%q", got, test.wantName, test.wantRaw, test.wantCat)
			}
			if got.Source != "eva02" || got.Score != test.prediction.Score {
				t.Fatalf("prediction provenance changed: %+v", got)
			}
		})
	}
}

func TestAppendUniqueTagIDs(t *testing.T) {
	seen := map[int]struct{}{2: {}}
	got := appendUniqueTagIDs([]int{2}, seen, []int{0, 2, 3, 3, 4})
	want := []int{2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendUniqueTagIDs() = %v, want %v", got, want)
	}
}
