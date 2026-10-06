//go:build integration

package characterdedup

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	_ "github.com/stashapp/stash/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) models.Repository {
	t.Helper()
	config.InitializeEmpty()
	db := sqlite.NewDatabase()
	db.SetBlobStoreOptions(sqlite.BlobStoreOptions{UseDatabase: true})
	require.NoError(t, db.Open(filepath.Join(t.TempDir(), "characters.sqlite")))
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	repo := db.Repository()
	require.NoError(t, repo.WithTxn(context.Background(), func(ctx context.Context) error {
		c, err := repo.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Series"})
		if err != nil {
			return err
		}
		for _, name := range []string{"Hatsune Miku", "Miku Hatsune", "Other Person", "Person Other"} {
			p := models.NewPerformer()
			p.Name = name
			if err := repo.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &p}); err != nil {
				return err
			}
			if err := repo.Copyright.SetPerformerCopyrights(ctx, p.ID, []int{c.ID}); err != nil {
				return err
			}
		}
		return nil
	}))
	return repo
}

func TestApplyPreservesNativeData(t *testing.T) {
	r := fixture(t)
	ctx := context.Background()
	require.NoError(t, r.WithTxn(ctx, func(ctx context.Context) error {
		source, err := r.Performer.Find(ctx, 2)
		if err != nil {
			return err
		}
		source.Details = "Useful profile"
		source.Favorite = true
		source.IgnoreAutoTag = true
		source.URLs = models.NewRelatedStrings([]string{"https://example.test/profile"})
		source.Aliases = models.NewRelatedStrings([]string{"Existing alias"})
		source.StashIDs = models.NewRelatedStashIDs([]models.StashID{{Endpoint: "https://example.test", StashID: "identity"}})
		tag := models.NewTag()
		tag.Name = "Profile tag"
		if err := r.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}); err != nil {
			return err
		}
		source.TagIDs = models.NewRelatedIDs([]int{tag.ID})
		if err := r.Performer.Update(ctx, &models.UpdatePerformerInput{Performer: source, CustomFields: models.CustomFieldsInput{Full: map[string]interface{}{"role": "lead"}}}); err != nil {
			return err
		}
		if err := r.Performer.UpdateImage(ctx, 2, []byte("portrait")); err != nil {
			return err
		}
		img := models.NewImage()
		if err := r.Image.Create(ctx, &models.CreateImageInput{Image: &img}); err != nil {
			return err
		}
		if err := r.Image.UpdatePerformers(ctx, img.ID, []int{1, 2}); err != nil {
			return err
		}
		scene := models.NewScene()
		scene.PerformerIDs = models.NewRelatedIDs([]int{2})
		if err := r.Scene.Create(ctx, &scene, nil); err != nil {
			return err
		}
		gallery := models.NewGallery()
		gallery.PerformerIDs = models.NewRelatedIDs([]int{2})
		return r.Gallery.Create(ctx, &models.CreateGalleryInput{Gallery: &gallery})
	}))
	p, err := Preview(ctx, r)
	require.NoError(t, err)
	require.Len(t, p.Groups, 2)
	require.NoError(t, Apply(ctx, r, p.Fingerprint, nil))
	require.NoError(t, r.WithReadTxn(ctx, func(ctx context.Context) error {
		dest, err := r.Performer.Find(ctx, 1)
		require.NoError(t, err)
		require.NoError(t, dest.LoadRelationships(ctx, r.Performer))
		require.NoError(t, dest.LoadURLs(ctx, r.Performer))
		require.Equal(t, "Hatsune Miku", dest.Name)
		require.Equal(t, "Useful profile", dest.Details)
		require.True(t, dest.Favorite)
		require.True(t, dest.IgnoreAutoTag)
		require.ElementsMatch(t, []string{"Existing alias", "Miku Hatsune (Series)"}, dest.Aliases.List())
		require.Equal(t, []string{"https://example.test/profile"}, dest.URLs.List())
		require.Equal(t, []int{1}, dest.TagIDs.List())
		require.Len(t, dest.StashIDs.List(), 1)
		fields, err := r.Performer.GetCustomFields(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "lead", fields["role"])
		image, err := r.Performer.GetImage(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, []byte("portrait"), image)
		copyrights, err := r.Copyright.FindByPerformerID(ctx, 1)
		require.NoError(t, err)
		require.Len(t, copyrights, 1)
		ps, err := r.Performer.FindByImageID(ctx, 1)
		require.NoError(t, err)
		require.Len(t, ps, 1)
		require.Equal(t, 1, ps[0].ID)
		ps, err = r.Performer.FindBySceneID(ctx, 1)
		require.NoError(t, err)
		require.Len(t, ps, 1)
		require.Equal(t, 1, ps[0].ID)
		ps, err = r.Performer.FindByGalleryID(ctx, 1)
		require.NoError(t, err)
		require.Len(t, ps, 1)
		require.Equal(t, 1, ps[0].ID)
		for _, id := range []int{2, 4} {
			deleted, err := r.Performer.Find(ctx, id)
			require.NoError(t, err)
			require.Nil(t, deleted)
		}
		return nil
	}))
	require.ErrorContains(t, Apply(ctx, r, p.Fingerprint, nil), "changed")
}

func TestStaleReviewAndCancellationLeaveAllCharacters(t *testing.T) {
	for _, operation := range []string{"alias edit", "new ambiguous character", "cancel after first group"} {
		t.Run(operation, func(t *testing.T) {
			r := fixture(t)
			ctx := context.Background()
			p, err := Preview(ctx, r)
			require.NoError(t, err)
			expected := 4
			if operation != "cancel after first group" {
				require.NoError(t, r.WithTxn(ctx, func(ctx context.Context) error {
					if operation == "alias edit" {
						v, err := r.Performer.Find(ctx, 2)
						if err != nil {
							return err
						}
						v.Aliases = models.NewRelatedStrings([]string{"new alias"})
						return r.Performer.Update(ctx, &models.UpdatePerformerInput{Performer: v})
					}
					expected++
					v := models.NewPerformer()
					v.Name = "Another Miku"
					if err := r.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &v}); err != nil {
						return err
					}
					return r.Copyright.SetPerformerCopyrights(ctx, v.ID, []int{1})
				}))
				require.ErrorContains(t, Apply(ctx, r, p.Fingerprint, nil), "changed")
			} else {
				cancelCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				require.ErrorIs(t, Apply(cancelCtx, r, p.Fingerprint, func(done, total int) {
					if done == 1 {
						cancel()
					}
				}), context.Canceled)
			}
			require.NoError(t, r.WithReadTxn(ctx, func(ctx context.Context) error {
				count, err := r.Performer.Count(ctx)
				require.Equal(t, expected, count)
				return err
			}))
		})
	}
}
