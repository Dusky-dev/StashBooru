package api

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/tag"
	"github.com/stashapp/stash/pkg/visualembedding"
)

type ratingHierarchyRepository struct {
	models.TagReaderWriter
	parents     []int
	descendants []*models.TagPath
	updated     bool
}

func (r *ratingHierarchyRepository) FindByName(context.Context, string, bool) (*models.Tag, error) {
	return &models.Tag{ID: 10, Name: "rating"}, nil
}
func (r *ratingHierarchyRepository) GetParentIDs(context.Context, int) ([]int, error) {
	return r.parents, nil
}
func (r *ratingHierarchyRepository) FindAllAncestors(context.Context, int, []int) ([]*models.TagPath, error) {
	return nil, nil
}
func (r *ratingHierarchyRepository) FindAllDescendants(context.Context, int, []int) ([]*models.TagPath, error) {
	return r.descendants, nil
}
func (r *ratingHierarchyRepository) UpdateParentTags(_ context.Context, _ int, ids []int) error {
	r.updated, r.parents = true, ids
	return nil
}

func TestContentRatingHierarchyRejectsCycle(t *testing.T) {
	for _, cyclic := range []bool{false, true} {
		r := &ratingHierarchyRepository{parents: []int{3}}
		if cyclic {
			r.descendants = []*models.TagPath{{Tag: models.Tag{ID: 10, Name: "rating"}}}
		}
		err := ensureContentRatingHierarchy(context.Background(), models.Repository{Tag: r}, &models.Tag{ID: 20, Name: "safe"})
		if cyclic {
			var hierarchyErr *tag.InvalidTagHierarchyError
			if !errors.As(err, &hierarchyErr) || r.updated {
				t.Fatalf("cycle was not rejected before writing: %v", err)
			}
		} else if err != nil || !reflect.DeepEqual(r.parents, []int{3, 10}) {
			t.Fatalf("valid parents not preserved: %v, %v", r.parents, err)
		}
	}
}

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
