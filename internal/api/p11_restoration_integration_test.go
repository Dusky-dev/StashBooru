//go:build integration

package api

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/imagerestore"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

type p11BrokenStack struct{ models.VisualStackReaderWriter }

func (p11BrokenStack) FindByMedia(context.Context, models.MediaReference) (*models.VisualStack, error) {
	return nil, errors.New("injected stack failure")
}

func p11Preview(t *testing.T) (models.Repository, imagerestore.Store, *imagerestore.Record, []byte) {
	t.Helper()
	repo := p07Fixture(t)
	ctx := context.Background()
	s := imagerestore.Store{Root: t.TempDir()}
	record := &imagerestore.Record{ID: mediaconvert.NewID(), SourceImageID: 1, SourceFileID: 1, Status: "preview", ExpiresAt: time.Now().Add(time.Hour), Options: imagerestore.Options{Hardware: "cpu", Steps: 20, Seed: 42, Guidance: 7.5, Strength: 1}, Receipt: imagerestore.Receipt{Protocol: 1, Model: "fixture-model", Revision: "pinned-fixture", Width: 64, Height: 48}}
	require.NoError(t, s.Save(record))
	dir, err := s.Directory(record.ID)
	require.NoError(t, err)
	source := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	output := image.NewNRGBA(source.Bounds())
	mask := image.NewGray(source.Bounds())
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 17, A: 123})
		}
	}
	copy(output.Pix, source.Pix)
	output.SetNRGBA(30, 20, color.NRGBA{R: 200, G: 100, B: 50, A: 123})
	mask.SetGray(30, 20, color.Gray{Y: 255})
	for name, data := range map[string]image.Image{"source.png": source, "output.png": output, "mask.png": mask, "effective-mask.png": mask} {
		require.NoError(t, imagerestore.WritePNG(filepath.Join(dir, name), data))
	}
	require.NoError(t, repo.WithTxn(ctx, func(ctx context.Context) error {
		files, err := repo.File.Find(ctx, 1)
		require.NoError(t, err)
		f := files[0].(*models.ImageFile)
		record.SourcePath = f.Path
		require.NoError(t, imagerestore.WritePNG(f.Path, source))
		stat, err := os.Stat(f.Path)
		require.NoError(t, err)
		f.Size, f.ModTime, f.FrameCount, f.Width, f.Height = stat.Size(), stat.ModTime(), 1, 64, 48
		record.SourceSize = f.Size
		record.SourceSHA256, err = imagerestore.SHA256(f.Path)
		require.NoError(t, err)
		record.SourceMD5, err = mediaconvert.MD5(f.Path)
		require.NoError(t, err)
		f.SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: record.SourceMD5})
		f.SetFingerprint(models.Fingerprint{Type: "phash", Fingerprint: int64(123)})
		require.NoError(t, repo.File.Update(ctx, f))
		character, tag := models.NewPerformer(), models.NewTag()
		character.Name = "Native Character"
		tag.Name = "Direct Tag"
		require.NoError(t, repo.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &character}))
		require.NoError(t, repo.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
		require.NoError(t, repo.Image.UpdatePerformers(ctx, 1, []int{character.ID}))
		require.NoError(t, repo.Image.UpdateTags(ctx, 1, []int{tag.ID}))
		var artists []int
		for _, name := range []string{"Artist one", "Artist two"} {
			artist := models.NewStudio()
			artist.Name = name
			require.NoError(t, repo.Studio.Create(ctx, &models.CreateStudioInput{Studio: &artist}))
			artists = append(artists, artist.ID)
		}
		require.NoError(t, repo.ImageArtist.SetImageArtists(ctx, 1, artists))
		copyright, err := repo.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Native Copyright"})
		require.NoError(t, err)
		require.NoError(t, repo.Copyright.SetImageCopyrights(ctx, 1, []int{copyright.ID}))
		gallery := models.NewGallery()
		require.NoError(t, repo.Gallery.Create(ctx, &models.CreateGalleryInput{Gallery: &gallery}))
		require.NoError(t, repo.Gallery.AddImages(ctx, gallery.ID, 2, 1, 3))
		_, err = repo.Image.UpdatePartial(ctx, 1, models.ImagePartial{URLs: &models.UpdateStrings{Values: []string{"https://example.test/source"}, Mode: models.RelationshipUpdateModeSet}, CustomFields: models.CustomFieldsInput{Full: map[string]interface{}{"fixture": "preserved"}}})
		return err
	}))
	record.CanonicalSHA256, err = imagerestore.SHA256(filepath.Join(dir, "source.png"))
	require.NoError(t, err)
	record.MaskSHA256, err = imagerestore.SHA256(filepath.Join(dir, "mask.png"))
	require.NoError(t, err)
	record.EffectiveMaskSHA256, err = imagerestore.SHA256(filepath.Join(dir, "effective-mask.png"))
	require.NoError(t, err)
	record.Receipt.OutputSHA256, err = imagerestore.SHA256(filepath.Join(dir, "output.png"))
	require.NoError(t, err)
	require.NoError(t, s.Save(record))
	bytes, err := os.ReadFile(record.SourcePath)
	require.NoError(t, err)
	return repo, s, record, bytes
}

func TestP11ReviewedDerivativePreservesSourceNativeMetadataAndStack(t *testing.T) {
	repo, s, record, original := p11Preview(t)
	ctx := context.Background()
	dir, _ := s.Directory(record.ID)
	preview, err := os.ReadFile(filepath.Join(dir, "output.png"))
	require.NoError(t, err)
	require.NoError(t, publishRestoration(ctx, repo, s, record, true))
	require.Equal(t, "saved", record.Status)
	require.NotEqual(t, 1, record.DerivedImageID)
	final, err := os.ReadFile(record.Destination)
	require.NoError(t, err)
	require.Equal(t, preview, final, "saved bytes are the exact reviewed preview")
	after, err := os.ReadFile(record.SourcePath)
	require.NoError(t, err)
	require.Equal(t, original, after)
	_, err = os.Stat(filepath.Join(dir, "mask.png"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "output.png"))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, repo.WithReadTxn(ctx, func(ctx context.Context) error {
		source, err := repo.Image.Find(ctx, 1)
		require.NoError(t, err)
		derived, err := repo.Image.Find(ctx, record.DerivedImageID)
		require.NoError(t, err)
		require.Equal(t, models.FileID(1), *source.PrimaryFileID)
		require.Contains(t, derived.Title, "Generated restoration")
		require.Equal(t, source.Details, derived.Details)
		require.Equal(t, source.Rating, derived.Rating)
		require.Equal(t, source.Organized, derived.Organized)
		for _, im := range []*models.Image{source, derived} {
			require.NoError(t, im.LoadURLs(ctx, repo.Image))
			require.NoError(t, im.LoadTagIDs(ctx, repo.Image))
			require.NoError(t, im.LoadPerformerIDs(ctx, repo.Image))
			require.NoError(t, im.LoadGalleryIDs(ctx, repo.Image))
		}
		require.Equal(t, source.URLs.List(), derived.URLs.List())
		require.Equal(t, source.TagIDs.List(), derived.TagIDs.List())
		require.Equal(t, source.PerformerIDs.List(), derived.PerformerIDs.List())
		require.Equal(t, source.GalleryIDs.List(), derived.GalleryIDs.List())
		for _, id := range source.GalleryIDs.List() {
			members, err := repo.Gallery.GetImageIDs(ctx, id)
			require.NoError(t, err)
			require.Equal(t, []int{1, 2, 3}, members[:3])
		}
		sa, err := repo.ImageArtist.FindByImageID(ctx, 1)
		require.NoError(t, err)
		da, err := repo.ImageArtist.FindByImageID(ctx, derived.ID)
		require.NoError(t, err)
		require.Len(t, sa, 2)
		require.Len(t, da, 2)
		require.Equal(t, sa[0].ID, da[0].ID)
		require.Equal(t, sa[1].ID, da[1].ID)
		sc, err := repo.Copyright.FindByImageID(ctx, 1)
		require.NoError(t, err)
		dc, err := repo.Copyright.FindByImageID(ctx, derived.ID)
		require.NoError(t, err)
		require.Equal(t, sc[0].ID, dc[0].ID)
		fields, err := repo.Image.GetCustomFields(ctx, derived.ID)
		require.NoError(t, err)
		require.Equal(t, "preserved", fields["fixture"])
		require.Contains(t, fields["StashBooru restoration"], record.MaskSHA256)
		files, err := repo.File.Find(ctx, *derived.PrimaryFileID)
		require.NoError(t, err)
		require.NotEqual(t, record.SourceMD5, files[0].Base().Fingerprints.GetString("md5"))
		require.Equal(t, record.SourceMD5, files[0].Base().Fingerprints.GetString("source_md5"))
		require.Nil(t, files[0].Base().Fingerprints.For("phash"))
		stack, err := repo.VisualStack.FindByMedia(ctx, models.MediaReference{Kind: models.MediaKindImage, ID: 1})
		require.NoError(t, err)
		stack, err = repo.VisualStack.Find(ctx, stack.ID)
		require.NoError(t, err)
		require.Len(t, stack.Members, 2)
		return nil
	}))
}

func TestP11ChangedSourceTamperedPreviewAndCatalogueRollback(t *testing.T) {
	for _, failure := range []string{"source", "preview", "stack"} {
		t.Run(failure, func(t *testing.T) {
			repo, s, record, _ := p11Preview(t)
			dir, _ := s.Directory(record.ID)
			if failure == "source" {
				require.NoError(t, os.WriteFile(record.SourcePath, []byte("changed"), 0600))
			}
			if failure == "preview" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "output.png"), []byte("corrupt"), 0600))
			}
			if failure == "stack" {
				repo.VisualStack = p11BrokenStack{repo.VisualStack}
			}
			require.Error(t, publishRestoration(context.Background(), repo, s, record, true))
			if record.Destination != "" {
				_, err := os.Stat(record.Destination)
				require.True(t, os.IsNotExist(err), "failed save removes new file")
			}
			require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
				images, err := repo.Image.All(ctx)
				require.NoError(t, err)
				require.Len(t, images, 5, "failed processing cannot register a broken derivative")
				return nil
			}))
		})
	}
}

func TestP11AppendRestorationPreservesExistingStack(t *testing.T) {
	repo, s, record, _ := p11Preview(t)
	ctx := context.Background()
	source := models.MediaReference{Kind: models.MediaKindImage, ID: 1}
	video := models.MediaReference{Kind: models.MediaKindVideo, ID: 1}
	var original *models.VisualStack
	require.NoError(t, repo.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		original, err = repo.VisualStack.Create(ctx, models.VisualStackCreateInput{
			Title: "Existing mixed-media variants", Representative: video,
			Members: []*models.VisualStackMemberInput{{Media: video, Label: "Video reference"}, {Media: source, Label: "Original still"}},
		})
		return err
	}))
	require.NoError(t, publishRestoration(ctx, repo, s, record, true))
	require.NoError(t, repo.WithReadTxn(ctx, func(ctx context.Context) error {
		stack, err := repo.VisualStack.Find(ctx, original.ID)
		require.NoError(t, err)
		require.Equal(t, original.Title, stack.Title)
		require.Equal(t, original.Representative, stack.Representative)
		require.Equal(t, original.Version+1, stack.Version)
		require.Len(t, stack.Members, 3)
		for i := range original.Members {
			require.Equal(t, original.Members[i], stack.Members[i], "existing member identity, order, label and representative survive")
		}
		require.Equal(t, models.MediaReference{Kind: models.MediaKindImage, ID: record.DerivedImageID}, stack.Members[2].Media)
		require.Equal(t, "Generated restoration", stack.Members[2].Label)
		return nil
	}))
}
