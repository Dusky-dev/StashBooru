package api

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
)

type effectiveAssociationResolverPerformerReader struct {
	effectiveAssociationPerformerReader
	items map[int]*models.Performer
}

func (r *effectiveAssociationResolverPerformerReader) FindMany(_ context.Context, ids []int) ([]*models.Performer, error) {
	result := make([]*models.Performer, 0, len(ids))
	for _, id := range ids {
		if item := r.items[id]; item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

type effectiveAssociationResolverStudioReader struct {
	effectiveAssociationStudioReader
	items map[int]*models.Studio
}

func (r *effectiveAssociationResolverStudioReader) FindMany(_ context.Context, ids []int) ([]*models.Studio, error) {
	result := make([]*models.Studio, 0, len(ids))
	for _, id := range ids {
		if item := r.items[id]; item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

type effectiveAssociationResolverCopyrightReader struct {
	effectiveAssociationCopyrightReader
	items  map[int]*models.Copyright
	images map[int][]*models.Copyright
	scenes map[int][]*models.Copyright
}

func (r *effectiveAssociationResolverCopyrightReader) FindByImageIDOrdered(_ context.Context, imageID int) ([]*models.Copyright, error) {
	return r.images[imageID], nil
}

func (r *effectiveAssociationResolverCopyrightReader) FindBySceneIDOrdered(_ context.Context, sceneID int) ([]*models.Copyright, error) {
	return r.scenes[sceneID], nil
}

func (r *effectiveAssociationResolverCopyrightReader) FindMany(_ context.Context, ids []int) ([]*models.Copyright, error) {
	result := make([]*models.Copyright, 0, len(ids))
	for _, id := range ids {
		if item := r.items[id]; item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

type effectiveAssociationResolverImageArtists struct {
	models.ImageArtistReaderWriter
	byImage map[int][]*models.Studio
}

func (r *effectiveAssociationResolverImageArtists) FindByImageID(_ context.Context, imageID int) ([]*models.Studio, error) {
	return r.byImage[imageID], nil
}

type effectiveAssociationResolverSceneArtists struct {
	models.SceneArtistReaderWriter
	byScene map[int][]*models.Studio
}

func (r *effectiveAssociationResolverSceneArtists) FindBySceneID(_ context.Context, sceneID int) ([]*models.Studio, error) {
	return r.byScene[sceneID], nil
}

func TestImageAndVideoResolversProjectEffectiveAssociationsFromDirectLinks(t *testing.T) {
	config.InitializeEmpty()
	parent := func(id int) *int { return &id }

	performers := &effectiveAssociationResolverPerformerReader{
		effectiveAssociationPerformerReader: effectiveAssociationPerformerReader{
			performers: map[int]*models.Performer{
				1:  {ID: 1, ParentID: parent(2)},
				2:  {ID: 2},
				20: {ID: 20, ParentID: parent(21)},
				21: {ID: 21},
			},
			tags: map[int][]int{1: {110}, 2: {111}, 20: {210}, 21: {211}},
		},
		items: map[int]*models.Performer{
			1: {ID: 1}, 2: {ID: 2}, 20: {ID: 20}, 21: {ID: 21},
		},
	}
	studios := &effectiveAssociationResolverStudioReader{
		effectiveAssociationStudioReader: effectiveAssociationStudioReader{
			studios: map[int]*models.Studio{
				4:  {ID: 4, ParentID: parent(5)},
				5:  {ID: 5},
				6:  {ID: 6, ParentID: parent(7)},
				7:  {ID: 7},
				30: {ID: 30, ParentID: parent(31)},
				31: {ID: 31},
			},
			tags: map[int][]int{
				4: {120}, 5: {121}, 6: {122}, 7: {123},
				30: {220}, 31: {221},
			},
		},
		items: map[int]*models.Studio{
			4: {ID: 4}, 5: {ID: 5}, 6: {ID: 6}, 7: {ID: 7},
			30: {ID: 30}, 31: {ID: 31},
		},
	}
	copyrights := &effectiveAssociationResolverCopyrightReader{
		effectiveAssociationCopyrightReader: effectiveAssociationCopyrightReader{
			parents: map[int][]*models.Copyright{
				10: {{ID: 11}, {ID: 12}},
				11: {{ID: 13}},
				12: {{ID: 13}},
				40: {{ID: 41}},
			},
			tags: map[int][]int{
				10: {130}, 11: {131}, 12: {132}, 13: {133},
				40: {230}, 41: {231},
			},
		},
		images: map[int][]*models.Copyright{1000: {{ID: 10}, {ID: 13}}},
		scenes: map[int][]*models.Copyright{2000: {{ID: 40}}},
		items: map[int]*models.Copyright{
			10: {ID: 10}, 11: {ID: 11}, 12: {ID: 12}, 13: {ID: 13},
			40: {ID: 40}, 41: {ID: 41},
		},
	}
	tags := &effectiveAssociationTagReader{
		parents: map[int][]*models.Tag{
			100: {{ID: 101}},
			110: {{ID: 112}},
			200: {{ID: 201}},
			210: {{ID: 212}},
		},
	}
	imageArtists := &effectiveAssociationResolverImageArtists{
		byImage: map[int][]*models.Studio{1000: {{ID: 6}, {ID: 7}}},
	}
	sceneArtists := &effectiveAssociationResolverSceneArtists{
		byScene: map[int][]*models.Studio{2000: {{ID: 30}}},
	}

	database := mocks.NewDatabase()
	repository := database.Repository()
	repository.Performer = performers
	repository.Studio = studios
	repository.Copyright = copyrights
	repository.Tag = tags
	repository.ImageArtist = imageArtists
	repository.SceneArtist = sceneArtists
	resolver := &Resolver{repository: repository}

	image := &models.Image{
		ID:           1000,
		StudioID:     parent(4), // legacy primary Artist and multi-Artist links coexist
		TagIDs:       models.NewRelatedIDs([]int{100, 101}),
		PerformerIDs: models.NewRelatedIDs([]int{1, 2}),
	}
	imageAssociations, err := resolver.Image().EffectiveAssociations(context.Background(), image)
	if err != nil {
		t.Fatalf("resolving Image associations: %v", err)
	}
	assertEffectiveIDs(t, "Image Tags", uniqueRelatedModelIDs(imageAssociations.Tags, func(item *models.Tag) int { return item.ID }), []int{100, 101, 110, 111, 112, 120, 121, 122, 123, 130, 131, 132, 133})
	assertEffectiveIDs(t, "Image Artists", uniqueRelatedModelIDs(imageAssociations.Artists, func(item *models.Studio) int { return item.ID }), []int{4, 5, 6, 7})
	assertEffectiveIDs(t, "Image Characters", uniqueRelatedModelIDs(imageAssociations.Performers, func(item *models.Performer) int { return item.ID }), []int{1, 2})
	assertEffectiveIDs(t, "Image Copyrights", uniqueRelatedModelIDs(imageAssociations.Copyrights, func(item *models.Copyright) int { return item.ID }), []int{10, 11, 12, 13})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "character", 1, MediaAssociationOrigin{
		Kind: "direct", SourceType: "character", SourceID: 1,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "character", 2, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "character", SourceID: 1, ViaType: "character", ViaID: 1,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "character", 2, MediaAssociationOrigin{
		Kind: "direct", SourceType: "character", SourceID: 2,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "artist", 4, MediaAssociationOrigin{
		Kind: "direct", SourceType: "artist", SourceID: 4,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "artist", 7, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "artist", SourceID: 6, ViaType: "artist", ViaID: 6,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "copyright", 13, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "copyright", SourceID: 10, ViaType: "copyright", ViaID: 11,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "copyright", 13, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "copyright", SourceID: 10, ViaType: "copyright", ViaID: 12,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "copyright", 13, MediaAssociationOrigin{
		Kind: "direct", SourceType: "copyright", SourceID: 13,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "tag", 111, MediaAssociationOrigin{
		Kind: "ancestor_profile_tag", SourceType: "character", SourceID: 1, ViaType: "character", ViaID: 2, SourceTagID: 111,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "tag", 111, MediaAssociationOrigin{
		Kind: "profile_tag", SourceType: "character", SourceID: 2, ViaType: "character", ViaID: 2, SourceTagID: 111,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "tag", 133, MediaAssociationOrigin{
		Kind: "ancestor_profile_tag", SourceType: "copyright", SourceID: 10, ViaType: "copyright", ViaID: 13, SourceTagID: 133,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "tag", 101, MediaAssociationOrigin{
		Kind: "tag_ancestor", SourceType: "tag", SourceID: 100, ViaType: "tag", ViaID: 100, SourceTagID: 100,
	})
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "tag", 101, MediaAssociationOrigin{
		Kind: "direct", SourceType: "tag", SourceID: 101, SourceTagID: 101,
	})

	video := &models.Scene{
		ID:           2000,
		StudioID:     parent(31),
		TagIDs:       models.NewRelatedIDs([]int{200}),
		PerformerIDs: models.NewRelatedIDs([]int{20}),
	}
	videoAssociations, err := resolver.Scene().EffectiveAssociations(context.Background(), video)
	if err != nil {
		t.Fatalf("resolving Video associations: %v", err)
	}
	assertEffectiveIDs(t, "Video Tags", uniqueRelatedModelIDs(videoAssociations.Tags, func(item *models.Tag) int { return item.ID }), []int{200, 201, 210, 211, 212, 220, 221, 230, 231})
	assertEffectiveIDs(t, "Video Artists", uniqueRelatedModelIDs(videoAssociations.Artists, func(item *models.Studio) int { return item.ID }), []int{30, 31})
	assertEffectiveIDs(t, "Video Characters", uniqueRelatedModelIDs(videoAssociations.Performers, func(item *models.Performer) int { return item.ID }), []int{20, 21})
	assertEffectiveIDs(t, "Video Copyrights", uniqueRelatedModelIDs(videoAssociations.Copyrights, func(item *models.Copyright) int { return item.ID }), []int{40, 41})
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "character", 21, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "character", SourceID: 20, ViaType: "character", ViaID: 20,
	})
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "artist", 31, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "artist", SourceID: 30, ViaType: "artist", ViaID: 30,
	})
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "copyright", 41, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "copyright", SourceID: 40, ViaType: "copyright", ViaID: 40,
	})
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "tag", 211, MediaAssociationOrigin{
		Kind: "ancestor_profile_tag", SourceType: "character", SourceID: 20, ViaType: "character", ViaID: 21, SourceTagID: 211,
	})
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "tag", 231, MediaAssociationOrigin{
		Kind: "ancestor_profile_tag", SourceType: "copyright", SourceID: 40, ViaType: "copyright", ViaID: 41, SourceTagID: 231,
	})

	if !reflect.DeepEqual(image.TagIDs.List(), []int{100, 101}) || !reflect.DeepEqual(video.TagIDs.List(), []int{200}) {
		t.Fatalf("resolving effective associations changed direct media Tags: image=%v video=%v", image.TagIDs.List(), video.TagIDs.List())
	}
}

func assertEffectiveIDs(t *testing.T, label string, got, want []int) {
	t.Helper()
	sort.Ints(got)
	sort.Ints(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func assertHasMediaAssociationOrigin(
	t *testing.T,
	provenance []MediaAssociationProvenance,
	associationType string,
	associationID int,
	want MediaAssociationOrigin,
) {
	t.Helper()
	for _, association := range provenance {
		if association.AssociationType != associationType || association.AssociationID != associationID {
			continue
		}
		for _, origin := range association.Origins {
			if origin == want {
				return
			}
		}
		t.Fatalf("%s %d origins = %+v, missing %+v", associationType, associationID, association.Origins, want)
	}
	t.Fatalf("provenance for %s %d not found", associationType, associationID)
}
