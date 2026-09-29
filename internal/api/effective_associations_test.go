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
	tags    map[int]*models.Tag
}

func (r *effectiveAssociationTagReader) FindByChildTagID(_ context.Context, id int) ([]*models.Tag, error) {
	return r.parents[id], nil
}

func (r *effectiveAssociationTagReader) FindMany(_ context.Context, ids []int) ([]*models.Tag, error) {
	ret := make([]*models.Tag, 0, len(ids))
	for _, id := range ids {
		if tag := r.tags[id]; tag != nil {
			ret = append(ret, tag)
		} else {
			ret = append(ret, &models.Tag{ID: id})
		}
	}
	return ret, nil
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
		// Explicitly linking a parent with its child still returns the shared
		// parent only once.
		tags:       []int{30, 29, 30},
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

func TestResolveInheritedTaggingTagsKeepsDerivedTagsOutOfDirectSelectionAndTracksOrigins(t *testing.T) {
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
			tags: map[int][]int{3: {31, 33}, 4: {34}},
		},
		Copyright: &effectiveAssociationCopyrightReader{
			parents: map[int][]*models.Copyright{5: {{ID: 6}}, 6: nil},
			tags:    map[int][]int{5: {35}, 6: {36}},
		},
		Tag: &effectiveAssociationTagReader{
			parents: map[int][]*models.Tag{
				30: {{ID: 40}}, // predicted tag ancestor
				31: {{ID: 41}, {ID: 42}}, // character profile Tag has multiple parents
				32: {{ID: 41}, {ID: 42}}, // shared by both Character profiles
				33: {{ID: 43}}, // Artist profile tag ancestor
				34: {{ID: 44}}, // parent Artist profile tag ancestor
				35: {{ID: 45}}, // Copyright profile tag ancestor
				36: {{ID: 46}}, // parent Copyright profile tag ancestor
			},
			tags: map[int]*models.Tag{
				30: {ID: 30, Name: "selected"},
				31: {ID: 31, Name: "character"},
				32: {ID: 32, Name: "parent-character"},
				33: {ID: 33, Name: "artist"},
				34: {ID: 34, Name: "parent-artist"},
				35: {ID: 35, Name: "copyright"},
				36: {ID: 36, Name: "parent-copyright"},
				40: {ID: 40, Name: "selected-parent"},
				41: {ID: 41, Name: "character-parent"},
				42: {ID: 42, Name: "parent-character-parent"},
				43: {ID: 43, Name: "artist-parent"},
				44: {ID: 44, Name: "parent-artist-parent"},
				45: {ID: 45, Name: "copyright-parent"},
				46: {ID: 46, Name: "parent-copyright-parent"},
			},
		},
	}
	direct := directMediaAssociationIDs{
		tags:       []int{30},
		performers: []int{1},
		artists:    []int{3},
		copyrights: []int{5},
	}

	inherited, err := resolveInheritedTaggingTags(context.Background(), repository, direct, config.AssociationInheritanceSettings{
		Characters: true,
		Artists:    true,
		Copyrights: true,
		Tags:       true,
	})
	if err != nil {
		t.Fatalf("resolveInheritedTaggingTags() error = %v", err)
	}

	want := []int{31, 32, 33, 34, 35, 36, 40, 41, 42, 43, 44, 45, 46}
	gotIDs := make([]int, 0, len(inherited))
	byID := make(map[int]taggingInheritedTag, len(inherited))
	for _, tag := range inherited {
		gotIDs = append(gotIDs, tag.ID)
		byID[tag.ID] = tag
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("inherited Tag IDs = %v, want %v", gotIDs, want)
	}
	if !reflect.DeepEqual(direct.tags, []int{30}) {
		t.Fatalf("selected Tag IDs changed while calculating inherited Tags: %v", direct.tags)
	}
	containsOrigin := func(tagID int, want effectiveAssociationTagOrigin) bool {
		for _, origin := range byID[tagID].Origins {
			if origin == want {
				return true
			}
		}
		return false
	}
	if !containsOrigin(31, effectiveAssociationTagOrigin{Kind: "character_profile", EntityID: 1, SourceTagID: 31}) {
		t.Fatalf("Character profile origin missing from inherited Tag: %+v", byID[31])
	}
	if !containsOrigin(31, effectiveAssociationTagOrigin{Kind: "artist_profile", EntityID: 3, SourceTagID: 31}) {
		t.Fatalf("shared Artist profile origin missing from inherited Tag: %+v", byID[31])
	}
	if !containsOrigin(32, effectiveAssociationTagOrigin{Kind: "character_profile", EntityID: 2, EntityAncestor: true, SourceTagID: 32}) {
		t.Fatalf("ancestor Character origin missing from inherited Tag: %+v", byID[32])
	}
	if !containsOrigin(40, effectiveAssociationTagOrigin{Kind: "selected_tag", SourceTagID: 30, TagAncestor: true}) {
		t.Fatalf("selected Tag hierarchy origin missing from inherited Tag: %+v", byID[40])
	}
	for _, sourceTagID := range []int{31, 32} {
		if !containsOrigin(41, effectiveAssociationTagOrigin{
			Kind: "character_profile", EntityID: sourceTagID - 30, SourceTagID: sourceTagID, TagAncestor: true,
		}) {
			t.Errorf("shared Tag parent lacks Character profile origin from Tag %d: %+v", sourceTagID, byID[41])
		}
	}
}

func TestResolveEffectiveMediaAssociationsReflectsReparentingOnNextRead(t *testing.T) {
	parentID := 2
	child := &models.Performer{ID: 1, ParentID: &parentID}
	repository := models.Repository{
		Performer: &effectiveAssociationPerformerReader{
			performers: map[int]*models.Performer{
				1: child,
				2: {ID: 2},
				3: {ID: 3},
			},
		},
	}
	direct := directMediaAssociationIDs{performers: []int{1}}
	settings := config.AssociationInheritanceSettings{Characters: true}

	before, err := resolveEffectiveMediaAssociationIDs(context.Background(), repository, direct, settings)
	if err != nil {
		t.Fatalf("resolving before reparent: %v", err)
	}
	if !reflect.DeepEqual(before.performers, []int{1, 2}) {
		t.Fatalf("before reparenting got Characters %v, want [1 2]", before.performers)
	}

	parentID = 3
	after, err := resolveEffectiveMediaAssociationIDs(context.Background(), repository, direct, settings)
	if err != nil {
		t.Fatalf("resolving after reparent: %v", err)
	}
	if !reflect.DeepEqual(after.performers, []int{1, 3}) {
		t.Fatalf("after reparenting got Characters %v, want [1 3]", after.performers)
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
