//go:build integration

package api

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/autotag"
	"github.com/stashapp/stash/internal/log"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/camietagger"
	imagepkg "github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/match"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/jsonschema"
	"github.com/stashapp/stash/pkg/models/paths"
	"github.com/stashapp/stash/pkg/plugin"
	scenepkg "github.com/stashapp/stash/pkg/scene"
	"github.com/stretchr/testify/require"
)

func p06TaggingManager(t *testing.T, repository models.Repository) {
	t.Helper()
	t.Setenv("STASH_CONFIG_FILE", filepath.Join(t.TempDir(), "new-system.yml"))
	cfg, err := config.Initialize()
	require.NoError(t, err)
	require.True(t, cfg.IsNewSystem())
	mgr, err := manager.Initialize(cfg, log.NewLogger())
	require.NoError(t, err)
	mgr.Repository = repository
	t.Cleanup(mgr.JobManager.Stop)
}

func (f *p06WorkflowFixture) predictions() []camietagger.Tag {
	return []camietagger.Tag{
		{Name: f.character.Name, Category: "character", Score: 1, Source: "existing",
			TargetExists: true, TargetPath: "/performers/" + strconv.Itoa(f.character.ID)},
		{Name: f.artist.Name, Category: "artist", Score: 1, Source: "existing",
			TargetExists: true, TargetPath: "/studios/" + strconv.Itoa(f.artist.ID)},
		{Name: f.copyright.Name, Category: "copyright", Score: 1, Source: "existing",
			TargetExists: true, TargetPath: "/copyrights/" + strconv.Itoa(f.copyright.ID)},
		{Name: f.tag.Name, Category: "general", Score: 1, Source: "existing",
			TargetExists: true, TargetPath: "/tags/" + strconv.Itoa(f.tag.ID)},
	}
}

func (f *p06WorkflowFixture) files(t *testing.T) (*models.ImageFile, *models.VideoFile) {
	t.Helper()
	now := time.Now()
	folder := &models.Folder{
		Path: t.TempDir(), DirEntry: models.DirEntry{ModTime: now}, CreatedAt: now, UpdatedAt: now,
	}
	var image *models.ImageFile
	var video *models.VideoFile
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		require.NoError(t, f.repository.Folder.Create(ctx, folder))
		imageName := "native.png"
		image = &models.ImageFile{BaseFile: &models.BaseFile{
			Basename: imageName, Path: filepath.Join(folder.Path, imageName), ParentFolderID: folder.ID,
			DirEntry: models.DirEntry{ModTime: now}, CreatedAt: now, UpdatedAt: now, FrameCount: 3,
		}, Format: "png", Width: 8, Height: 8}
		videoName := "[" + f.artist.Name + "](" + f.copyright.Name + ")." + f.character.Name + "_0123456789abcdef0123456789abcdef.mp4"
		video = &models.VideoFile{BaseFile: &models.BaseFile{
			Basename: videoName, Path: filepath.Join(folder.Path, videoName), ParentFolderID: folder.ID,
			DirEntry: models.DirEntry{ModTime: now}, CreatedAt: now, UpdatedAt: now,
		}, Format: "mp4", Width: 8, Height: 8, Duration: 1, VideoCodec: "h264", FrameRate: 30}
		require.NoError(t, f.repository.File.Create(ctx, image))
		require.NoError(t, f.repository.File.Create(ctx, video))
		require.NoError(t, f.repository.Image.AddFileID(ctx, f.image.ID, image.ID))
		return f.repository.Scene.AddFileID(ctx, f.video.ID, video.ID)
	}))
	return image, video
}

func TestP06NativeReviewedTaggingAndBulkVideoReview(t *testing.T) {
	f := newP06WorkflowFixture(t)
	p06TaggingManager(t, f.repository)
	f.files(t)
	predictions := f.predictions()
	plan := buildTaggingChangePlan(predictions, true)
	require.True(t, plan.CanApply)
	require.NoError(t, populateTaggingChangePlanInheritedTags(context.Background(), f.repository, predictions, &plan))
	require.NotEmpty(t, plan.InheritedTags)
	require.Empty(t, f.direct(t, false).performers)
	require.Empty(t, f.direct(t, true).copyrights)
	for repeat := 0; repeat < 2; repeat++ {
		image, err := applyCamieMetadataV2(context.Background(), f.image.ID, taggingChangePlanPredictions(plan), true)
		require.NoError(t, err)
		require.NotEmpty(t, image.InheritedTags)
		video, err := applySceneTaggingMetadata(context.Background(), f.video.ID, taggingChangePlanPredictions(plan), true)
		require.NoError(t, err)
		require.NotEmpty(t, video.InheritedTags)
		f.assertSelected(t)
	}

	// Review the real batch filename path before applying it to cleared native links.
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Scene.UpdatePartial(ctx, f.video.ID, models.ScenePartial{
			PerformerIDs: &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet, IDs: []int{}},
		})
		require.NoError(t, err)
		require.NoError(t, f.repository.SceneArtist.SetSceneArtists(ctx, f.video.ID, nil))
		return f.repository.Copyright.SetSceneCopyrights(ctx, f.video.ID, nil)
	}))
	review := processSceneTaggingBatchItem(context.Background(), f.video.ID, defaultCamieConfig(), sceneTaggingBatchRequest{})
	require.Empty(t, review.Error)
	require.Len(t, review.AutoApply, 3)
	require.Empty(t, review.NeedsReview)
	require.Nil(t, review.Applied)
	require.Empty(t, f.direct(t, true).performers)
	for repeat := 0; repeat < 2; repeat++ {
		applied := processSceneTaggingBatchItem(context.Background(), f.video.ID, defaultCamieConfig(), sceneTaggingBatchRequest{AutoApply: true, ReplaceArtist: true})
		require.Empty(t, applied.Error)
		require.NotNil(t, applied.Applied)
		require.NotEmpty(t, applied.Applied.InheritedTags)
		f.assertSelected(t)
	}
}

func TestP06NativeAutoTagKeepsOnlyMatchedSelections(t *testing.T) {
	f := newP06WorkflowFixture(t)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		require.NoError(t, f.repository.ImageArtist.SetImageArtists(ctx, f.image.ID, nil))
		require.NoError(t, f.repository.SceneArtist.SetSceneArtists(ctx, f.video.ID, nil))
		require.NoError(t, f.repository.Copyright.SetImageCopyrights(ctx, f.image.ID, []int{f.copyright.ID}))
		require.NoError(t, f.repository.Copyright.SetSceneCopyrights(ctx, f.video.ID, []int{f.copyright.ID}))
		cache := &match.Cache{}
		path := filepath.Join(t.TempDir(), f.character.Name+" "+f.artist.Name+" "+f.tag.Name+".mp4")
		image, err := f.repository.Image.Find(ctx, f.image.ID)
		require.NoError(t, err)
		image.Path = path
		require.NoError(t, autotag.ImagePerformers(ctx, image, f.repository.Image, f.repository.Performer, cache))
		require.NoError(t, autotag.ImageStudios(ctx, image, f.repository.Image, f.repository.Studio, cache))
		require.NoError(t, autotag.ImageTags(ctx, image, f.repository.Image, f.repository.Tag, cache))
		video, err := f.repository.Scene.Find(ctx, f.video.ID)
		require.NoError(t, err)
		video.Path = path
		require.NoError(t, autotag.ScenePerformers(ctx, video, f.repository.Scene, f.repository.Performer, cache))
		require.NoError(t, autotag.SceneStudios(ctx, video, f.repository.Scene, f.repository.Studio, cache))
		return autotag.SceneTags(ctx, video, f.repository.Scene, f.repository.Tag, cache)
	}))
	f.assertSelected(t)
}

func TestP06NativeImportAndRepeatedImportPreserveInheritance(t *testing.T) {
	f := newP06WorkflowFixture(t)
	imageFile, videoFile := f.files(t)
	// The legacy JSON schema has no Copyright field; its native links survive an import.
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		require.NoError(t, f.repository.Copyright.SetImageCopyrights(ctx, f.image.ID, []int{f.copyright.ID}))
		return f.repository.Copyright.SetSceneCopyrights(ctx, f.video.ID, []int{f.copyright.ID})
	}))
	for repeat := 0; repeat < 2; repeat++ {
		require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
			image := imagepkg.Importer{
				ReaderWriter: f.repository.Image, FileFinder: f.repository.File, StudioWriter: f.repository.Studio,
				GalleryFinder: f.repository.Gallery, PerformerWriter: f.repository.Performer, TagWriter: f.repository.Tag,
				MissingRefBehaviour: models.ImportMissingRefEnumFail,
				Input: jsonschema.Image{Title: "Imported Image", Studio: f.artist.Name,
					Performers: []string{f.character.Name}, Tags: []string{f.tag.Name}, Files: []string{imageFile.Path}},
			}
			require.NoError(t, image.PreImport(ctx))
			imageID, err := image.FindExistingID(ctx)
			require.NoError(t, err)
			require.Equal(t, &f.image.ID, imageID)
			require.NoError(t, image.Update(ctx, *imageID))
			require.NoError(t, image.PostImport(ctx, *imageID))
			video := scenepkg.Importer{
				ReaderWriter: f.repository.Scene, FileFinder: f.repository.File, StudioWriter: f.repository.Studio,
				GalleryFinder: f.repository.Gallery, PerformerWriter: f.repository.Performer, TagWriter: f.repository.Tag,
				GroupWriter: f.repository.Group, MissingRefBehaviour: models.ImportMissingRefEnumFail,
				FileNamingAlgorithm: models.HashAlgorithmMd5,
				Input: jsonschema.Scene{Title: "Imported Video", Studio: f.artist.Name,
					Performers: []string{f.character.Name}, Tags: []string{f.tag.Name}, Files: []string{videoFile.Path}},
			}
			require.NoError(t, video.PreImport(ctx))
			videoID, err := video.FindExistingID(ctx)
			require.NoError(t, err)
			require.Equal(t, &f.video.ID, videoID)
			require.NoError(t, video.Update(ctx, *videoID))
			return video.PostImport(ctx, *videoID)
		}))
		f.assertSelected(t)
	}
}

type p06ImageGenerator struct{}

func (p06ImageGenerator) Generate(context.Context, *models.Image, models.File) error { return nil }

type p06VideoGenerator struct{}

func (p06VideoGenerator) Generate(context.Context, *models.Scene, *models.VideoFile) error {
	return nil
}

func TestP06NativeScanAndRescanPreserveExplicitSelections(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, false)
	imageFile, videoFile := f.files(t)
	var animated *models.Tag
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		tag := models.NewTag()
		tag.Name = "animated"
		tag.ParentIDs = models.NewRelatedIDs([]int{f.parentTag.ID})
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
		animated = &tag
		return nil
	}))
	cfg := config.GetInstance()
	mediaPaths := paths.NewPaths(t.TempDir(), t.TempDir())
	cache := plugin.NewCache(cfg)
	imageScan := imagepkg.ScanHandler{
		AnimationTags: f.repository.Tag, CreatorUpdater: f.repository.Image,
		GalleryFinder: f.repository.Gallery, SceneFinderUpdater: f.repository.Scene,
		ScanGenerator: p06ImageGenerator{}, ScanConfig: cfg, PluginCache: cache, Paths: &mediaPaths,
	}
	videoScan := scenepkg.ScanHandler{
		CreatorUpdater: f.repository.Scene, GalleryFinderUpdater: f.repository.Gallery,
		ScanGenerator: p06VideoGenerator{}, CaptionUpdater: f.repository.File, PluginCache: cache,
		FileNamingAlgorithm: models.HashAlgorithmMd5, Paths: &mediaPaths,
	}
	for repeat := 0; repeat < 2; repeat++ {
		var oldImage, oldVideo models.File
		if repeat > 0 {
			oldImage, oldVideo = imageFile.Clone(), videoFile.Clone()
		}
		require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
			require.NoError(t, imageScan.Handle(ctx, imageFile, oldImage))
			return videoScan.Handle(ctx, videoFile, oldVideo)
		}))
		for _, video := range []bool{false, true} {
			direct := f.direct(t, video)
			require.Equal(t, []int{f.character.ID}, direct.performers)
			require.Equal(t, []int{f.artist.ID}, direct.artists)
			require.Equal(t, []int{f.copyright.ID}, direct.copyrights)
			tags := []int{f.tag.ID}
			if !video {
				tags = append(tags, animated.ID)
			}
			require.ElementsMatch(t, tags, direct.tags)
			effective := f.effective(t, video)
			require.Contains(t, p06Memberships(effective, "character"), f.characterRoot.ID)
			require.Contains(t, p06Memberships(effective, "artist"), f.artistRoot.ID)
			require.Contains(t, p06Memberships(effective, "copyright"), f.copyrightRoot.ID)
			require.Contains(t, p06Memberships(effective, "tag"), f.parentTag.ID)
		}
	}
}
