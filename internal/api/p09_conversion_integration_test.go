//go:build integration

package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Native content identity in synthetic fixtures.
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestP09TrialApplyRestorePreservesNativeCatalogue(t *testing.T) {
	r := p07Fixture(t)
	store := mediaconvert.Store{Root: filepath.Join(t.TempDir(), "conversion"), Repository: mediaconvert.ModelRepository{Repository: r}}
	source := []byte(strings.Repeat("synthetic original", 100))
	output := []byte("synthetic converted")
	gallery := models.NewGallery()
	ref := models.MediaReference{Kind: models.MediaKindImage, ID: 1}
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		files, err := r.File.Find(ctx, 1)
		require.NoError(t, err)
		f := files[0]
		require.NoError(t, os.WriteFile(f.Base().Path, source, 0600))
		stat, err := os.Stat(f.Base().Path)
		require.NoError(t, err)
		f.Base().Size, f.Base().ModTime, f.Base().FrameCount = stat.Size(), stat.ModTime(), 1
		f.Base().SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: fmt.Sprintf("%x", md5.Sum(source))})
		f.Base().SetFingerprint(models.Fingerprint{Type: "phash", Fingerprint: int64(-9223372036854775701)})
		require.NoError(t, r.File.Update(ctx, f))
		character, artist, tag := models.NewPerformer(), models.NewStudio(), models.NewTag()
		character.Name, artist.Name, tag.Name = "Character", "Artist", "Direct tag"
		require.NoError(t, r.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &character}))
		require.NoError(t, r.Studio.Create(ctx, &models.CreateStudioInput{Studio: &artist}))
		require.NoError(t, r.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
		require.NoError(t, r.Image.UpdatePerformers(ctx, 1, []int{character.ID}))
		require.NoError(t, r.Image.UpdateTags(ctx, 1, []int{tag.ID}))
		require.NoError(t, r.ImageArtist.SetImageArtists(ctx, 1, []int{artist.ID}))
		copyright, err := r.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Native copyright"})
		require.NoError(t, err)
		require.NoError(t, r.Copyright.SetImageCopyrights(ctx, 1, []int{copyright.ID}))
		require.NoError(t, r.Gallery.Create(ctx, &models.CreateGalleryInput{Gallery: &gallery}))
		require.NoError(t, r.Gallery.AddImages(ctx, gallery.ID, 2, 1, 3))
		_, err = r.Image.UpdatePartial(ctx, 1, models.ImagePartial{
			URLs:         &models.UpdateStrings{Values: []string{"https://example.test/provenance"}, Mode: models.RelationshipUpdateModeSet},
			CustomFields: models.CustomFieldsInput{Full: map[string]interface{}{"provenance": "original"}},
		})
		require.NoError(t, err)
		_, err = r.VisualStack.Create(ctx, models.VisualStackCreateInput{Title: "Native stack", Members: []*models.VisualStackMemberInput{
			{Media: ref}, {Media: models.MediaReference{Kind: models.MediaKindVideo, ID: 1}},
		}, Representative: ref})
		return err
	}))
	snapshot := func() []byte {
		var ret []byte
		require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
			image, err := r.Image.Find(ctx, 1)
			require.NoError(t, err)
			require.NoError(t, image.LoadURLs(ctx, r.Image))
			require.NoError(t, image.LoadTagIDs(ctx, r.Image))
			require.NoError(t, image.LoadPerformerIDs(ctx, r.Image))
			require.NoError(t, image.LoadGalleryIDs(ctx, r.Image))
			image.Path, image.Checksum = "", "" // Active file fields intentionally change.
			galleryIDs, err := r.Gallery.GetImageIDs(ctx, gallery.ID)
			require.NoError(t, err)
			copyrights, err := r.Copyright.FindByImageIDOrdered(ctx, 1)
			require.NoError(t, err)
			artists, err := r.ImageArtist.FindByImageID(ctx, 1)
			require.NoError(t, err)
			fields, err := r.Image.GetCustomFields(ctx, 1)
			require.NoError(t, err)
			stack, err := r.VisualStack.FindByMedia(ctx, ref)
			require.NoError(t, err)
			ret, err = json.Marshal([]any{image, galleryIDs, copyrights, artists, fields, stack})
			return err
		}))
		return ret
	}
	before := snapshot()
	nativeBefore, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		result, _ := json.Marshal(mediaconvert.Result{Width: 8, Height: 8, Frames: 1, Size: int64(len(output)), Format: "webp", VideoCodec: "webp", Signature: "p09-fixture"})
		w.Header().Set("X-Stash-Conversion", base64.StdEncoding.EncodeToString(result))
		w.Header().Set("X-Stash-Content-MD5", fmt.Sprintf("%x", md5.Sum(output)))
		w.Header().Set("Content-Length", fmt.Sprint(len(output)))
		_, _ = w.Write(output)
	}))
	defer server.Close()
	client := mediaconvert.Client{URL: server.URL}
	options := mediaconvert.Options{Format: "webp", Hardware: "cpu", Quality: 80, Effort: 7}
	trial, err := store.CreateTrial(context.Background(), 1, "trial", mediaconvert.TrialTarget{Kind: "image", ID: 1}, client, options,
		mediaconvert.Format{ID: "webp", Family: "animation", Extension: "webp"}, mediaconvert.SavingsThresholds{}, "p09-fixture", true)
	require.NoError(t, err)
	require.Equal(t, "verified", trial.Status)
	require.Equal(t, before, snapshot(), "trial must leave native relationships and metadata intact")
	nativeDuring, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, mediaconvert.SameFile(nativeBefore, nativeDuring))
	history, err := store.History()
	require.NoError(t, err)
	require.Empty(t, history, "trial must not consume restore entries")
	applied, err := store.ApplyTrial(context.Background(), trial.ID, "apply", client, options, trial.Savings, "p09-fixture")
	require.NoError(t, err)
	require.Equal(t, "complete", applied.Status)
	require.Equal(t, before, snapshot(), "apply preserves media IDs, galleries/order, direct associations, stack, metadata and provenance")
	nativeAfter, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, nativeBefore.Base().ID, nativeAfter.Base().ID)
	require.NotEqual(t, nativeBefore.Base().Path, nativeAfter.Base().Path)
	require.NoError(t, store.Restore(context.Background(), applied.ID))
	require.Equal(t, before, snapshot(), "restore preserves the native catalogue")
	restored, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	data, err := os.ReadFile(restored.Base().Path)
	require.NoError(t, err)
	require.Equal(t, source, data)
	require.Equal(t, nativeBefore.Base().Fingerprints, restored.Base().Fingerprints)
}
