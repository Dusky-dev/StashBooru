package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

type effectiveAssociationTestTxnManager struct{}

func (effectiveAssociationTestTxnManager) Begin(ctx context.Context, _ bool) (context.Context, error) {
	return ctx, nil
}

func (effectiveAssociationTestTxnManager) Commit(context.Context) error   { return nil }
func (effectiveAssociationTestTxnManager) Rollback(context.Context) error { return nil }
func (effectiveAssociationTestTxnManager) IsLocked(error) bool            { return false }
func (effectiveAssociationTestTxnManager) WithDatabase(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

type effectiveAssociationTestImageStore struct {
	models.ImageReaderWriter
	image *models.Image
}

func (s *effectiveAssociationTestImageStore) Find(_ context.Context, id int) (*models.Image, error) {
	if s.image != nil && s.image.ID == id {
		return s.image, nil
	}
	return nil, nil
}

type effectiveAssociationTestSceneStore struct {
	models.SceneReaderWriter
	scene *models.Scene
}

func (s *effectiveAssociationTestSceneStore) Find(_ context.Context, id int) (*models.Scene, error) {
	if s.scene != nil && s.scene.ID == id {
		return s.scene, nil
	}
	return nil, nil
}

type effectiveAssociationTestImageArtists struct{ models.ImageArtistReaderWriter }

func (effectiveAssociationTestImageArtists) FindByImageID(context.Context, int) ([]*models.Studio, error) {
	return nil, nil
}

type effectiveAssociationTestSceneArtists struct{ models.SceneArtistReaderWriter }

func (effectiveAssociationTestSceneArtists) FindBySceneID(context.Context, int) ([]*models.Studio, error) {
	return nil, nil
}

type effectiveAssociationTestCopyrightStore struct {
	models.CopyrightReaderWriter
	imageIDs map[int][]int
	sceneIDs map[int][]int
	items    map[int]*models.Copyright
	parents  map[int][]int
}

func (s *effectiveAssociationTestCopyrightStore) SetImageCopyrights(_ context.Context, imageID int, ids []int) error {
	s.imageIDs[imageID] = append([]int(nil), ids...)
	return nil
}

func (s *effectiveAssociationTestCopyrightStore) SetSceneCopyrights(_ context.Context, sceneID int, ids []int) error {
	s.sceneIDs[sceneID] = append([]int(nil), ids...)
	return nil
}

func (s *effectiveAssociationTestCopyrightStore) FindByImageIDOrdered(_ context.Context, imageID int) ([]*models.Copyright, error) {
	return s.findMany(s.imageIDs[imageID]), nil
}

func (s *effectiveAssociationTestCopyrightStore) FindBySceneIDOrdered(_ context.Context, sceneID int) ([]*models.Copyright, error) {
	return s.findMany(s.sceneIDs[sceneID]), nil
}

func (s *effectiveAssociationTestCopyrightStore) FindMany(_ context.Context, ids []int) ([]*models.Copyright, error) {
	return s.findMany(ids), nil
}

func (s *effectiveAssociationTestCopyrightStore) FindParents(_ context.Context, id int) ([]*models.Copyright, error) {
	return s.findMany(s.parents[id]), nil
}

func (s *effectiveAssociationTestCopyrightStore) GetTagIDs(context.Context, int) ([]int, error) {
	return nil, nil
}

func (s *effectiveAssociationTestCopyrightStore) findMany(ids []int) []*models.Copyright {
	result := make([]*models.Copyright, 0, len(ids))
	for _, id := range ids {
		if item := s.items[id]; item != nil {
			result = append(result, item)
		}
	}
	return result
}

func TestManualImageAndVideoCopyrightEditsExposeInheritedCopyrights(t *testing.T) {
	config.InitializeEmpty()
	const (
		imageID         = 100
		videoID         = 200
		subCopyrightID  = 10
		mainCopyrightID = 11
	)

	image := &models.Image{
		ID:           imageID,
		TagIDs:       models.NewRelatedIDs([]int{}),
		PerformerIDs: models.NewRelatedIDs([]int{}),
	}
	video := &models.Scene{
		ID:           videoID,
		TagIDs:       models.NewRelatedIDs([]int{}),
		PerformerIDs: models.NewRelatedIDs([]int{}),
	}
	copyrights := &effectiveAssociationTestCopyrightStore{
		imageIDs: make(map[int][]int),
		sceneIDs: make(map[int][]int),
		items: map[int]*models.Copyright{
			subCopyrightID:  {ID: subCopyrightID, Name: "Sub Copyright"},
			mainCopyrightID: {ID: mainCopyrightID, Name: "Main Copyright"},
		},
		parents: map[int][]int{subCopyrightID: {mainCopyrightID}},
	}
	resolver := &Resolver{
		repository: models.Repository{
			TxnManager:  effectiveAssociationTestTxnManager{},
			Image:       &effectiveAssociationTestImageStore{image: image},
			ImageArtist: effectiveAssociationTestImageArtists{},
			Performer:   &effectiveAssociationPerformerReader{},
			Scene:       &effectiveAssociationTestSceneStore{scene: video},
			SceneArtist: effectiveAssociationTestSceneArtists{},
			Studio:      &effectiveAssociationStudioReader{},
			Copyright:   copyrights,
		},
	}
	mutation := &mutationResolver{Resolver: resolver}
	ctx := context.Background()
	wantIDs := []int{subCopyrightID, mainCopyrightID}

	updatedImage, err := mutation.ImageCopyrightsUpdate(ctx, "100", []string{"10"})
	if err != nil {
		t.Fatalf("manual Image Copyright update: %v", err)
	}
	if !reflect.DeepEqual(copyrights.imageIDs[imageID], []int{subCopyrightID}) {
		t.Fatalf("stored direct Image Copyright IDs = %v, want only the selected sub-Copyright", copyrights.imageIDs[imageID])
	}
	imageAssociations, err := resolver.Image().EffectiveAssociations(ctx, updatedImage)
	if err != nil {
		t.Fatalf("Image effective associations after manual update: %v", err)
	}
	assertEffectiveIDs(t, "manually edited Image Copyrights", uniqueRelatedModelIDs(imageAssociations.Copyrights, func(item *models.Copyright) int { return item.ID }), wantIDs)
	assertHasMediaAssociationOrigin(t, imageAssociations.Provenance, "copyright", mainCopyrightID, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "copyright", SourceID: subCopyrightID, ViaType: "copyright", ViaID: subCopyrightID,
	})

	updatedVideo, err := mutation.SceneCopyrightsUpdate(ctx, "200", []string{"10"})
	if err != nil {
		t.Fatalf("manual Video Copyright update: %v", err)
	}
	if !reflect.DeepEqual(copyrights.sceneIDs[videoID], []int{subCopyrightID}) {
		t.Fatalf("stored direct Video Copyright IDs = %v, want only the selected sub-Copyright", copyrights.sceneIDs[videoID])
	}
	videoAssociations, err := resolver.Scene().EffectiveAssociations(ctx, updatedVideo)
	if err != nil {
		t.Fatalf("Video effective associations after manual update: %v", err)
	}
	assertEffectiveIDs(t, "manually edited Video Copyrights", uniqueRelatedModelIDs(videoAssociations.Copyrights, func(item *models.Copyright) int { return item.ID }), wantIDs)
	assertHasMediaAssociationOrigin(t, videoAssociations.Provenance, "copyright", mainCopyrightID, MediaAssociationOrigin{
		Kind: "ancestor", SourceType: "copyright", SourceID: subCopyrightID, ViaType: "copyright", ViaID: subCopyrightID,
	})
}
