//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/plugin/hook"
	"github.com/stretchr/testify/require"
)

type p06WorkflowFixture struct {
	*p06SQLiteFixture
	characterRoot  *models.Performer
	character      *models.Performer
	artistRoot     *models.Studio
	copyrightRoot  *models.Copyright
	copyrightLeft  *models.Copyright
	copyrightRight *models.Copyright
	copyright      *models.Copyright
	tagLeft        *models.Tag
	tag            *models.Tag
}

func newP06WorkflowFixture(t *testing.T) *p06WorkflowFixture {
	t.Helper()
	f := &p06WorkflowFixture{p06SQLiteFixture: newP06SQLiteFixture(t)}
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		root := models.NewPerformer()
		root.Name = "Base Hero"
		root.TagIDs = models.NewRelatedIDs([]int{f.profileTag.ID})
		require.NoError(t, f.repository.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &root}))
		f.characterRoot = &root
		character := models.NewPerformer()
		character.Name = "Variant Hero"
		character.ParentID = &root.ID
		require.NoError(t, f.repository.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &character}))
		f.character = &character

		artist := models.NewStudio()
		artist.Name = "Parent Artist"
		artist.TagIDs = models.NewRelatedIDs([]int{f.profileTag.ID})
		require.NoError(t, f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &artist}))
		f.artistRoot = &artist
		_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: f.artist.ID, ParentID: models.NewOptionalInt(artist.ID)})
		require.NoError(t, err)

		f.copyrightRoot, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Root Franchise"})
		require.NoError(t, err)
		require.NoError(t, f.repository.Copyright.UpdateTags(ctx, f.copyrightRoot.ID, []int{f.profileTag.ID}))
		f.copyrightLeft, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{
			Name: "Left Series", ParentIDs: []string{strconv.Itoa(f.copyrightRoot.ID)},
		})
		require.NoError(t, err)
		f.copyrightRight, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{
			Name: "Right Series", ParentIDs: []string{strconv.Itoa(f.copyrightRoot.ID)},
		})
		require.NoError(t, err)
		f.copyright, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{
			Name: "Selected Series", ParentIDs: []string{strconv.Itoa(f.copyrightLeft.ID), strconv.Itoa(f.copyrightRight.ID)},
		})
		require.NoError(t, err)
		require.NoError(t, f.repository.Copyright.SetPerformerCopyrights(ctx, f.character.ID, []int{f.copyright.ID}))

		left := models.NewTag()
		left.Name = "Native Tag Branch"
		left.ParentIDs = models.NewRelatedIDs([]int{f.parentTag.ID})
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &left}))
		f.tagLeft = &left
		tag := models.NewTag()
		tag.Name = "Native Selected Tag"
		tag.ParentIDs = models.NewRelatedIDs([]int{left.ID, f.profileTag.ID})
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &tag}))
		f.tag = &tag
		return nil
	}))
	return f
}

type p06NoHooks struct{}

func (p06NoHooks) ExecutePostHooks(context.Context, int, hook.TriggerEnum, interface{}, []string) {}

func p06GraphQL(t *testing.T, repository models.Repository, query string, input any) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": map[string]any{"input": input}})
	require.NoError(t, err)
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{
		repository: repository, hookExecutor: p06NoHooks{},
	}}))
	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var result struct {
		Errors []json.RawMessage `json:"errors"`
		Data   json.RawMessage   `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Empty(t, result.Errors, response.Body.String())
	require.NotEmpty(t, result.Data)
}

func (f *p06WorkflowFixture) manual(t *testing.T, bulk bool) {
	t.Helper()
	images := []*models.Image{f.image}
	videos := []*models.Scene{f.video}
	if bulk {
		require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
			image := models.NewImage()
			require.NoError(t, f.repository.Image.Create(ctx, &models.CreateImageInput{Image: &image}))
			images = append(images, &image)
			video := models.NewScene()
			require.NoError(t, f.repository.Scene.Create(ctx, &video, nil))
			videos = append(videos, &video)
			return nil
		}))
	}
	imageInput := map[string]any{"id": strconv.Itoa(f.image.ID), "studio_id": strconv.Itoa(f.artist.ID)}
	videoInput := map[string]any{"id": strconv.Itoa(f.video.ID), "studio_id": strconv.Itoa(f.artist.ID)}
	performers := []string{strconv.Itoa(f.character.ID)}
	tags := []string{strconv.Itoa(f.tag.ID)}
	if bulk {
		delete(imageInput, "id")
		delete(videoInput, "id")
		imageInput["ids"] = []string{strconv.Itoa(images[0].ID), strconv.Itoa(images[1].ID)}
		videoInput["ids"] = []string{strconv.Itoa(videos[0].ID), strconv.Itoa(videos[1].ID)}
		imageInput["performer_ids"] = map[string]any{"ids": performers, "mode": "SET"}
		imageInput["tag_ids"] = map[string]any{"ids": tags, "mode": "SET"}
		videoInput["performer_ids"] = imageInput["performer_ids"]
		videoInput["tag_ids"] = imageInput["tag_ids"]
		p06GraphQL(t, f.repository, "mutation($input: BulkImageUpdateInput!) { bulkImageUpdate(input: $input) { id } }", imageInput)
		p06GraphQL(t, f.repository, "mutation($input: BulkSceneUpdateInput!) { bulkSceneUpdate(input: $input) { id } }", videoInput)
	} else {
		imageInput["performer_ids"], imageInput["tag_ids"] = performers, tags
		videoInput["performer_ids"], videoInput["tag_ids"] = performers, tags
		p06GraphQL(t, f.repository, "mutation($input: ImageUpdateInput!) { imageUpdate(input: $input) { id } }", imageInput)
		p06GraphQL(t, f.repository, "mutation($input: SceneUpdateInput!) { sceneUpdate(input: $input) { id } }", videoInput)
	}
	resolver := (&Resolver{repository: f.repository}).Mutation()
	for index := range images {
		_, err := resolver.ImageCopyrightsUpdate(context.Background(), strconv.Itoa(images[index].ID), []string{strconv.Itoa(f.copyright.ID)})
		require.NoError(t, err)
		_, err = resolver.SceneCopyrightsUpdate(context.Background(), strconv.Itoa(videos[index].ID), []string{strconv.Itoa(f.copyright.ID)})
		require.NoError(t, err)
		if index > 0 {
			other := *f
			media := *f.p06SQLiteFixture
			media.image, media.video = images[index], videos[index]
			other.p06SQLiteFixture = &media
			other.assertSelected(t)
		}
	}
}

func (f *p06WorkflowFixture) effective(t *testing.T, video bool) *MediaEffectiveAssociations {
	t.Helper()
	resolver := &Resolver{repository: f.repository}
	var image *models.Image
	var scene *models.Scene
	require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		var err error
		if video {
			scene, err = f.repository.Scene.Find(ctx, f.video.ID)
		} else {
			image, err = f.repository.Image.Find(ctx, f.image.ID)
		}
		return err
	}))
	var result *MediaEffectiveAssociations
	var err error
	if video {
		result, err = resolver.Scene().EffectiveAssociations(context.Background(), scene)
	} else {
		result, err = resolver.Image().EffectiveAssociations(context.Background(), image)
	}
	require.NoError(t, err)
	return result
}

func p06Memberships(effective *MediaEffectiveAssociations, domain string) []int {
	var ids []int
	for _, membership := range effective.Provenance {
		if membership.AssociationType == domain {
			ids = append(ids, membership.AssociationID)
		}
	}
	return ids
}

func (f *p06WorkflowFixture) assertSelected(t *testing.T) {
	t.Helper()
	for _, video := range []bool{false, true} {
		direct := f.direct(t, video)
		require.Equal(t, []int{f.character.ID}, direct.performers)
		require.Equal(t, []int{f.artist.ID}, direct.artists)
		require.Equal(t, []int{f.copyright.ID}, direct.copyrights)
		require.Equal(t, []int{f.tag.ID}, direct.tags)
		effective := f.effective(t, video)
		require.ElementsMatch(t, []int{f.character.ID, f.characterRoot.ID}, p06Memberships(effective, "character"))
		require.ElementsMatch(t, []int{f.artist.ID, f.artistRoot.ID}, p06Memberships(effective, "artist"))
		require.ElementsMatch(t, []int{f.copyright.ID, f.copyrightLeft.ID, f.copyrightRight.ID, f.copyrightRoot.ID}, p06Memberships(effective, "copyright"))
		require.ElementsMatch(t, []int{f.tag.ID, f.tagLeft.ID, f.profileTag.ID, f.parentTag.ID}, p06Memberships(effective, "tag"))
		assertHasMediaAssociationOrigin(t, effective.Provenance, "copyright", f.copyrightRoot.ID, MediaAssociationOrigin{
			Kind: "ancestor", SourceType: "copyright", SourceID: f.copyright.ID, ViaType: "copyright", ViaID: f.copyrightLeft.ID,
		})
		assertHasMediaAssociationOrigin(t, effective.Provenance, "copyright", f.copyrightRoot.ID, MediaAssociationOrigin{
			Kind: "ancestor", SourceType: "copyright", SourceID: f.copyright.ID, ViaType: "copyright", ViaID: f.copyrightRight.ID,
		})
		assertHasMediaAssociationOrigin(t, effective.Provenance, "tag", f.parentTag.ID, MediaAssociationOrigin{
			Kind: "tag_ancestor", SourceType: "tag", SourceID: f.tag.ID, SourceTagID: f.tag.ID, ViaType: "tag", ViaID: f.tagLeft.ID,
		})
		assertHasMediaAssociationOrigin(t, effective.Provenance, "tag", f.profileTag.ID, MediaAssociationOrigin{
			Kind: "ancestor_profile_tag", SourceType: "character", SourceID: f.character.ID,
			ViaType: "character", ViaID: f.characterRoot.ID, SourceTagID: f.profileTag.ID,
		})
	}
}

func TestP06NativeManualSingleAndBulkEdits(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		name := "single"
		if bulk {
			name = "bulk"
		}
		t.Run(name, func(t *testing.T) {
			f := newP06WorkflowFixture(t)
			f.manual(t, bulk)
			f.assertSelected(t)
		})
	}
}

func TestP06NativeSharedSourcesDetachAndExplicitParents(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, false)
	f.assertSelected(t)
	var sibling *models.Copyright
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		var err error
		sibling, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{
			Name: "Sibling Series", ParentIDs: []string{strconv.Itoa(f.copyrightRoot.ID)},
		})
		return err
	}))
	resolver := (&Resolver{repository: f.repository}).Mutation()
	for _, video := range []bool{false, true} {
		if video {
			_, err := resolver.SceneCopyrightsUpdate(context.Background(), strconv.Itoa(f.video.ID),
				[]string{strconv.Itoa(sibling.ID), strconv.Itoa(f.copyrightRoot.ID)})
			require.NoError(t, err)
		} else {
			_, err := resolver.ImageCopyrightsUpdate(context.Background(), strconv.Itoa(f.image.ID),
				[]string{strconv.Itoa(sibling.ID), strconv.Itoa(f.copyrightRoot.ID)})
			require.NoError(t, err)
		}
		effective := f.effective(t, video)
		require.ElementsMatch(t, []int{sibling.ID, f.copyrightRoot.ID}, p06Memberships(effective, "copyright"))
		assertHasMediaAssociationOrigin(t, effective.Provenance, "copyright", f.copyrightRoot.ID, MediaAssociationOrigin{
			Kind: "direct", SourceType: "copyright", SourceID: f.copyrightRoot.ID,
		})
		assertHasMediaAssociationOrigin(t, effective.Provenance, "copyright", f.copyrightRoot.ID, MediaAssociationOrigin{
			Kind: "ancestor", SourceType: "copyright", SourceID: sibling.ID, ViaType: "copyright", ViaID: sibling.ID,
		})
		// Removing the Copyright source leaves shared Character/Artist profile Tags.
		if video {
			_, err := resolver.SceneCopyrightsUpdate(context.Background(), strconv.Itoa(f.video.ID), []string{})
			require.NoError(t, err)
		} else {
			_, err := resolver.ImageCopyrightsUpdate(context.Background(), strconv.Itoa(f.image.ID), []string{})
			require.NoError(t, err)
		}
		effective = f.effective(t, video)
		require.Empty(t, p06Memberships(effective, "copyright"))
		require.Contains(t, p06Memberships(effective, "tag"), f.profileTag.ID)
		require.Contains(t, p06Memberships(effective, "tag"), f.parentTag.ID)
	}
}

func TestP06NativeReparentingChangesNextReadWithoutMediaWrites(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, false)
	beforeImage, beforeVideo := f.direct(t, false), f.direct(t, true)
	var character *models.Performer
	var artist *models.Studio
	var copyright *models.Copyright
	var tag *models.Tag
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		newCharacter := models.NewPerformer()
		newCharacter.Name = "Replacement Hero"
		require.NoError(t, f.repository.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &newCharacter}))
		character = &newCharacter
		_, err := f.repository.Performer.UpdatePartial(ctx, f.character.ID, models.PerformerPartial{ParentID: models.NewOptionalInt(character.ID)})
		require.NoError(t, err)
		newArtist := models.NewStudio()
		newArtist.Name = "Replacement Artist"
		require.NoError(t, f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &newArtist}))
		artist = &newArtist
		_, err = f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: f.artist.ID, ParentID: models.NewOptionalInt(artist.ID)})
		require.NoError(t, err)
		copyright, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Replacement Franchise"})
		require.NoError(t, err)
		_, err = f.repository.Copyright.Update(ctx, models.CopyrightUpdateInput{
			ID: strconv.Itoa(f.copyright.ID), ParentIDs: []string{strconv.Itoa(copyright.ID)},
		})
		require.NoError(t, err)
		newTag := models.NewTag()
		newTag.Name = "Replacement Tag"
		require.NoError(t, f.repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &newTag}))
		tag = &newTag
		return f.repository.Tag.UpdateParentTags(ctx, f.tag.ID, []int{tag.ID})
	}))
	for _, video := range []bool{false, true} {
		effective := f.effective(t, video)
		require.ElementsMatch(t, []int{f.character.ID, character.ID}, p06Memberships(effective, "character"))
		require.ElementsMatch(t, []int{f.artist.ID, artist.ID}, p06Memberships(effective, "artist"))
		require.ElementsMatch(t, []int{f.copyright.ID, copyright.ID}, p06Memberships(effective, "copyright"))
		require.ElementsMatch(t, []int{f.tag.ID, tag.ID, f.profileTag.ID, f.parentTag.ID}, p06Memberships(effective, "tag"))
	}
	require.Equal(t, beforeImage, f.direct(t, false))
	require.Equal(t, beforeVideo, f.direct(t, true))
}

func TestP06NativeDetachAllSourcesRemovesOnlyUnsupportedInheritance(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, false)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Image.UpdatePartial(ctx, f.image.ID, models.ImagePartial{
			PerformerIDs: &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet, IDs: []int{}},
			TagIDs:       &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet, IDs: []int{f.parentTag.ID}},
		})
		require.NoError(t, err)
		_, err = f.repository.Scene.UpdatePartial(ctx, f.video.ID, models.ScenePartial{
			PerformerIDs: &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet, IDs: []int{}},
			TagIDs:       &models.UpdateIDs{Mode: models.RelationshipUpdateModeSet, IDs: []int{f.parentTag.ID}},
		})
		require.NoError(t, err)
		require.NoError(t, f.repository.ImageArtist.SetImageArtists(ctx, f.image.ID, nil))
		require.NoError(t, f.repository.SceneArtist.SetSceneArtists(ctx, f.video.ID, nil))
		require.NoError(t, f.repository.Copyright.SetImageCopyrights(ctx, f.image.ID, nil))
		return f.repository.Copyright.SetSceneCopyrights(ctx, f.video.ID, nil)
	}))
	for _, video := range []bool{false, true} {
		effective := f.effective(t, video)
		require.Empty(t, effective.Performers)
		require.Empty(t, effective.Artists)
		require.Empty(t, effective.Copyrights)
		require.Equal(t, []int{f.parentTag.ID}, p06Memberships(effective, "tag"))
		require.Equal(t, []int{f.parentTag.ID}, f.direct(t, video).tags)
	}
}
