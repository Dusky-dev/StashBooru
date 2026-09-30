//go:build integration

package api

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

type p06SQLiteFixture struct {
	repository models.Repository
	artist     *models.Studio
	profileTag *models.Tag
	parentTag  *models.Tag
	image      *models.Image
	video      *models.Scene
}

func newP06SQLiteFixture(t *testing.T) *p06SQLiteFixture {
	t.Helper()
	config.InitializeEmpty()
	database := sqlite.NewDatabase()
	require.NoError(t, database.Open(filepath.Join(t.TempDir(), "p06.sqlite")))
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	f := &p06SQLiteFixture{repository: database.Repository()}
	ctx := context.Background()
	require.NoError(t, f.repository.WithTxn(ctx, func(ctx context.Context) error {
		root := models.NewTag()
		root.Name = "Profile parent"
		if err := f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &root}); err != nil {
			return err
		}
		f.parentTag = &root
		profile := models.NewTag()
		profile.Name = "Profile child"
		profile.ParentIDs = models.NewRelatedIDs([]int{root.ID})
		if err := f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &profile}); err != nil {
			return err
		}
		f.profileTag = &profile
		artist := models.NewStudio()
		artist.Name = "Selected Artist"
		artist.TagIDs = models.NewRelatedIDs([]int{profile.ID})
		if err := f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &artist}); err != nil {
			return err
		}
		f.artist = &artist
		image := models.NewImage()
		image.Title = "Image"
		if err := f.repository.Image.Create(ctx, &models.CreateImageInput{Image: &image}); err != nil {
			return err
		}
		f.image = &image
		if err := f.repository.ImageArtist.SetImageArtists(ctx, image.ID, []int{artist.ID}); err != nil {
			return err
		}
		video := models.NewScene()
		video.Title = "Video"
		if err := f.repository.Scene.Create(ctx, &video, nil); err != nil {
			return err
		}
		f.video = &video
		return f.repository.SceneArtist.SetSceneArtists(ctx, video.ID, []int{artist.ID})
	}))
	return f
}

func (f *p06SQLiteFixture) direct(t *testing.T, video bool) directMediaAssociationIDs {
	t.Helper()
	var direct directMediaAssociationIDs
	require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		if video {
			item, err := f.repository.Scene.Find(ctx, f.video.ID)
			if err != nil {
				return err
			}
			direct, err = sceneDirectMediaAssociations(ctx, f.repository, item)
			return err
		}
		item, err := f.repository.Image.Find(ctx, f.image.ID)
		if err != nil {
			return err
		}
		direct, err = imageDirectMediaAssociations(ctx, f.repository, item)
		return err
	}))
	return direct
}

func TestP06SQLiteSourceLoadersDeduplicatePrimaryArtist(t *testing.T) {
	f := newP06SQLiteFixture(t)
	for _, video := range []bool{false, true} {
		direct := f.direct(t, video)
		require.Equal(t, []int{f.artist.ID}, direct.artists, "native multi-Artist readers also include the legacy primary Artist")
		require.Empty(t, direct.tags, "profile Tags must not become direct media Tags")
	}
}

func TestP06SQLiteTaggingPreviewOwnsReadTransaction(t *testing.T) {
	f := newP06SQLiteFixture(t)
	predictions := []camietagger.Tag{{
		Name:         f.artist.Name,
		Category:     "artist",
		TargetExists: true,
		TargetPath:   "/studios/" + strconv.Itoa(f.artist.ID),
	}}
	plan := buildTaggingChangePlan(predictions, false)

	// Image and Video HTTP previews call this helper without a database
	// transaction. A mock repository did not expose the missing transaction.
	require.NoError(t, populateTaggingChangePlanInheritedTags(context.Background(), f.repository, predictions, &plan))
	byID := make(map[int]taggingInheritedTag)
	for _, item := range plan.InheritedTags {
		byID[item.ID] = item
	}
	require.Contains(t, byID, f.profileTag.ID)
	require.Contains(t, byID, f.parentTag.ID)
	require.Contains(t, byID[f.profileTag.ID].Origins, effectiveAssociationTagOrigin{
		Kind: "artist_profile", EntityID: f.artist.ID, SourceTagID: f.profileTag.ID,
	})
	require.Contains(t, byID[f.parentTag.ID].Origins, effectiveAssociationTagOrigin{
		Kind: "artist_profile", EntityID: f.artist.ID, SourceTagID: f.profileTag.ID, TagAncestor: true,
	})
	require.Equal(t, predictions, taggingChangePlanPredictions(plan))
	require.Empty(t, f.direct(t, false).tags)
	require.Empty(t, f.direct(t, true).tags)
}
