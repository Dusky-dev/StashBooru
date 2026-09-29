package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

type effectiveAssociationPerformerReader struct {
	models.PerformerReaderWriter
	performers map[int]*models.Performer
	tags       map[int][]int
}

func (r *effectiveAssociationPerformerReader) Find(_ context.Context, id int) (*models.Performer, error) {
	return r.performers[id], nil
}

func (r *effectiveAssociationPerformerReader) GetTagIDs(_ context.Context, id int) ([]int, error) {
	return r.tags[id], nil
}

type effectiveAssociationStudioReader struct {
	models.StudioReaderWriter
	studios map[int]*models.Studio
	tags    map[int][]int
}

func (r *effectiveAssociationStudioReader) Find(_ context.Context, id int) (*models.Studio, error) {
	return r.studios[id], nil
}

func (r *effectiveAssociationStudioReader) GetTagIDs(_ context.Context, id int) ([]int, error) {
	return r.tags[id], nil
}

type effectiveAssociationCopyrightReader struct {
	models.CopyrightReaderWriter
	parents map[int][]*models.Copyright
	tags    map[int][]int
}

func (r *effectiveAssociationCopyrightReader) FindParents(_ context.Context, id int) ([]*models.Copyright, error) {
	return r.parents[id], nil
}

func (r *effectiveAssociationCopyrightReader) GetTagIDs(_ context.Context, id int) ([]int, error) {
	return r.tags[id], nil
}

type effectiveAssociationTagReader struct {
	models.TagReaderWriter
	parents map[int][]*models.Tag
}

func (r *effectiveAssociationTagReader) FindByChildTagID(_ context.Context, id int) ([]*models.Tag, error) {
	return r.parents[id], nil
}

func TestResolveEffectiveMediaAssociationIDsIncludesLiveAncestorsAndProfileTags(t *testing.T) {
	parent := func(id int) *int { return &id }

	repository := models.Repository{
		Performer: &effectiveAssociationPerformerReader{
			performers: map[int]*models.Performer{
				1: {ID: 1, ParentID: parent(3)}, // cycle guard
				2: {ID: 2, ParentID: parent(1)},
				3: {ID: 3, ParentID: parent(2)},
			},
			tags: map[int][]int{1: {42}, 2: {41}, 3: {40}},
		},
		Studio: &effectiveAssociationStudioReader{
			studios: map[int]*models.Studio{
				11: {ID: 11},
				12: {ID: 12, ParentID: parent(11)},
				13: {ID: 13, ParentID: parent(12)},
			},
			tags: map[int][]int{11: {45}, 12: {44}, 13: {43}},
		},
		Copyright: &effectiveAssociationCopyrightReader{
			parents: map[int][]*models.Copyright{
				20: nil,
				21: {{ID: 20}},
				22: {{ID: 20}},
				23: {{ID: 22}, {ID: 21}},
			},
			tags: map[int][]int{20: {49}, 21: {48}, 22: {47}, 23: {46}},
		},
		Tag: &effectiveAssociationTagReader{
			parents: map[int][]*models.Tag{
				28: {{ID: 30}}, // a cycle must not expand forever
				29: {{ID: 28}},
				30: {{ID: 29}},
				39: {{ID: 40}},
				40: {{ID: 39}},
			},
		},
	}

	got, err := resolveEffectiveMediaAssociationIDs(context.Background(), repository, directMediaAssociationIDs{
		tags:       []int{30, 30},
		artists:    []int{13},
		performers: []int{3},
		copyrights: []int{23},
	}, config.AssociationInheritanceSettings{
		Characters: true,
		Artists:    true,
		Copyrights: true,
		Tags:       true,
	})
	if err != nil {
		t.Fatalf("resolveEffectiveMediaAssociationIDs() error = %v", err)
	}

	assertIDs := func(label string, got []int, want []int) {
		t.Helper()
		present := make(map[int]bool, len(got))
		for _, id := range got {
			if present[id] {
				t.Errorf("%s contains duplicate ID %d: %v", label, id, got)
			}
			present[id] = true
		}
		for _, id := range want {
			if !present[id] {
				t.Errorf("%s is missing ID %d: %v", label, id, got)
			}
		}
	}

	assertIDs("performers", got.performers, []int{3, 2, 1})
	assertIDs("artists", got.artists, []int{13, 12, 11})
	assertIDs("copyrights", got.copyrights, []int{23, 22, 21, 20})
	assertIDs("tags", got.tags, []int{30, 29, 28, 40, 39, 41, 42, 43, 44, 45, 46, 47, 48, 49})
}

func TestInheritResolvedProfileTagsIncludesAncestorProfilesAndTagParents(t *testing.T) {
	parent := func(id int) *int { return &id }
	repository := models.Repository{
		Performer: &effectiveAssociationPerformerReader{
			performers: map[int]*models.Performer{
				1: {ID: 1, ParentID: parent(2)},
				2: {ID: 2},
			},
			tags: map[int][]int{1: {31}, 2: {32}},
		},
		Studio: &effectiveAssociationStudioReader{
			studios: map[int]*models.Studio{
				3: {ID: 3, ParentID: parent(4)},
				4: {ID: 4},
			},
			tags: map[int][]int{3: {33}, 4: {34}},
		},
		Copyright: &effectiveAssociationCopyrightReader{
			parents: map[int][]*models.Copyright{5: {{ID: 6}}, 6: nil},
			tags:    map[int][]int{5: {35}, 6: {36}},
		},
		Tag: &effectiveAssociationTagReader{
			parents: map[int][]*models.Tag{
				30: {{ID: 40}}, // predicted tag ancestor
				31: {{ID: 41}}, // character profile tag ancestor
				32: {{ID: 42}}, // parent Character profile tag ancestor
				33: {{ID: 43}}, // Artist profile tag ancestor
				34: {{ID: 44}}, // parent Artist profile tag ancestor
				35: {{ID: 45}}, // Copyright profile tag ancestor
				36: {{ID: 46}}, // parent Copyright profile tag ancestor
			},
		},
	}
	resolved := taggingResolvedEntities{
		CharacterIDs: []int{1},
		ArtistIDs:    []int{3},
		CopyrightIDs: []int{5},
		TagIDs:       []int{30},
	}

	if err := inheritResolvedProfileTagIDs(context.Background(), repository, &resolved, config.AssociationInheritanceSettings{
		Characters: true,
		Artists:    true,
		Copyrights: true,
		Tags:       true,
	}); err != nil {
		t.Fatalf("inheritResolvedProfileTagIDs() error = %v", err)
	}

	want := []int{30, 31, 32, 33, 34, 35, 36, 40, 41, 42, 43, 44, 45, 46}
	if !reflect.DeepEqual(resolved.TagIDs, want) {
		t.Fatalf("resolved Tags = %v, want %v", resolved.TagIDs, want)
	}
}

func TestResolveEffectiveMediaAssociationIDsRespectsDomainSettings(t *testing.T) {
	parent := func(id int) *int { return &id }
	repository := models.Repository{
		Performer: &effectiveAssociationPerformerReader{
			performers: map[int]*models.Performer{1: {ID: 1, ParentID: parent(2)}, 2: {ID: 2}},
			tags:       map[int][]int{1: {31}, 2: {32}},
		},
		Studio: &effectiveAssociationStudioReader{
			studios: map[int]*models.Studio{3: {ID: 3, ParentID: parent(4)}, 4: {ID: 4}},
			tags:    map[int][]int{3: {33}, 4: {34}},
		},
		Copyright: &effectiveAssociationCopyrightReader{
			parents: map[int][]*models.Copyright{5: {{ID: 6}}, 6: nil},
			tags:    map[int][]int{5: {35}, 6: {36}},
		},
		Tag: &effectiveAssociationTagReader{
			parents: map[int][]*models.Tag{
				30: {{ID: 40}},
				31: {{ID: 41}},
				32: {{ID: 42}},
				33: {{ID: 43}},
				34: {{ID: 44}},
				35: {{ID: 45}},
				36: {{ID: 46}},
			},
		},
	}
	direct := directMediaAssociationIDs{
		tags:       []int{30},
		artists:    []int{3},
		performers: []int{1},
		copyrights: []int{5},
	}

	got, err := resolveEffectiveMediaAssociationIDs(
		context.Background(), repository, direct, config.AssociationInheritanceSettings{},
	)
	if err != nil {
		t.Fatalf("resolve with inheritance disabled: %v", err)
	}
	if !reflect.DeepEqual(got.performers, []int{1}) || !reflect.DeepEqual(got.artists, []int{3}) || !reflect.DeepEqual(got.copyrights, []int{5}) {
		t.Fatalf("disabled hierarchy domains included ancestors: performers=%v artists=%v copyrights=%v", got.performers, got.artists, got.copyrights)
	}
	if !reflect.DeepEqual(got.tags, []int{30, 31, 33, 35}) {
		t.Fatalf("disabled hierarchy domains should keep direct/profile Tags only: got %v", got.tags)
	}

	got, err = resolveEffectiveMediaAssociationIDs(
		context.Background(), repository, direct, config.AssociationInheritanceSettings{
			Characters: true,
			Copyrights: true,
			Tags:       true,
		},
	)
	if err != nil {
		t.Fatalf("resolve with selective inheritance: %v", err)
	}
	if !reflect.DeepEqual(got.performers, []int{1, 2}) || !reflect.DeepEqual(got.artists, []int{3}) || !reflect.DeepEqual(got.copyrights, []int{5, 6}) {
		t.Fatalf("selective hierarchy settings not applied: performers=%v artists=%v copyrights=%v", got.performers, got.artists, got.copyrights)
	}
	for _, want := range []int{30, 40, 31, 41, 32, 42, 33, 43, 35, 45, 36, 46} {
		found := false
		for _, gotID := range got.tags {
			if gotID == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("effective Tags %v missing inherited ID %d", got.tags, want)
		}
	}
	for _, unexpected := range []int{34, 44} {
		for _, gotID := range got.tags {
			if gotID == unexpected {
				t.Errorf("disabled Artist inheritance unexpectedly contributed Tag %d: %v", unexpected, got.tags)
			}
		}
	}
}
