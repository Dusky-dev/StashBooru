//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func p07Fixture(t *testing.T) models.Repository {
	t.Helper()
	config.InitializeEmpty()
	database := sqlite.NewDatabase()
	require.NoError(t, database.Open(filepath.Join(t.TempDir(), "media.sqlite")))
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	repository := database.Repository()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repository.WithTxn(context.Background(), func(ctx context.Context) error {
		for i := 1; i <= 5; i++ {
			image, scene := models.NewImage(), models.NewScene()
			image.Title, scene.Title = fmt.Sprintf("Item %d", i*2), fmt.Sprintf("Item %d", i*2-1)
			image.CreatedAt, scene.CreatedAt = now, now
			image.UpdatedAt, scene.UpdatedAt = now, now
			if i < 5 {
				imageDay, sceneDay := 7-i*2, 8-i*2
				if i == 4 {
					imageDay, sceneDay = 2, 2
				}
				imageDate, err := models.ParseDate(fmt.Sprintf("2026-01-%02d", imageDay))
				require.NoError(t, err)
				sceneDate, err := models.ParseDate(fmt.Sprintf("2026-01-%02d", sceneDay))
				require.NoError(t, err)
				image.Date, scene.Date = &imageDate, &sceneDate
				rating := 10
				if i == 1 {
					rating = 90
					image.Details, scene.Details = "shared needle", "shared needle"
					image.Organized, scene.Organized = true, true
				}
				image.Rating, scene.Rating = &rating, &rating
			} else {
				image.Title, scene.Title = "", ""
			}
			require.NoError(t, repository.Image.Create(ctx, &models.CreateImageInput{Image: &image}))
			require.NoError(t, repository.Scene.Create(ctx, &scene, nil))
			require.Equal(t, i, image.ID)
			require.Equal(t, image.ID, scene.ID, "fixture deliberately collides across native tables")
		}
		folder := &models.Folder{Path: t.TempDir(), DirEntry: models.DirEntry{ModTime: now}, CreatedAt: now, UpdatedAt: now}
		require.NoError(t, repository.Folder.Create(ctx, folder))
		imageFile := &models.ImageFile{BaseFile: &models.BaseFile{
			Basename: "item10.png", Path: filepath.Join(folder.Path, "item10.png"), ParentFolderID: folder.ID,
			DirEntry: models.DirEntry{ModTime: now}, Size: 100, CreatedAt: now, UpdatedAt: now, FrameCount: 3,
		}, Format: "png", Width: 8, Height: 8}
		sceneFile := &models.VideoFile{BaseFile: &models.BaseFile{
			Basename: "item2.mp4", Path: filepath.Join(folder.Path, "item2.mp4"), ParentFolderID: folder.ID,
			DirEntry: models.DirEntry{ModTime: now}, Size: 200, CreatedAt: now, UpdatedAt: now,
		}, Format: "mp4", Width: 8, Height: 8, Duration: 60, VideoCodec: "h264", FrameRate: 30}
		require.NoError(t, repository.File.Create(ctx, imageFile))
		require.NoError(t, repository.File.Create(ctx, sceneFile))
		require.NoError(t, repository.Image.AddFileID(ctx, 1, imageFile.ID))
		require.NoError(t, repository.Scene.AddFileID(ctx, 1, sceneFile.ID))
		_, err := repository.Image.UpdatePartial(ctx, 1, models.ImagePartial{PrimaryFileID: &imageFile.ID})
		require.NoError(t, err)
		_, err = repository.Scene.UpdatePartial(ctx, 1, models.ScenePartial{PrimaryFileID: &sceneFile.ID})
		return err
	}))
	return repository
}

func p07Query(t *testing.T, repository models.Repository, filter *models.MediaFilterType, find *models.FindFilterType) *models.MediaQueryResult {
	t.Helper()
	var result *models.MediaQueryResult
	require.NoError(t, repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		var err error
		result, err = repository.Media.Query(ctx, filter, find)
		return err
	}))
	return result
}

func p07Keys(result *models.MediaQueryResult) []string {
	keys := make([]string, len(result.Items))
	for i, item := range result.Items {
		keys[i] = item.Key()
	}
	return keys
}

func TestP07GlobalPaginationAndStableNullLastOrdering(t *testing.T) {
	repository := p07Fixture(t)
	sort, size := "date", 3
	for _, test := range []struct {
		direction models.SortDirectionEnum
		want      []string
	}{
		{models.SortDirectionEnumDesc, []string{"scene:1", "image:1", "scene:2", "image:2", "image:4", "scene:3", "scene:4", "image:3", "image:5", "scene:5"}},
		{models.SortDirectionEnumAsc, []string{"image:3", "image:4", "scene:3", "scene:4", "image:2", "scene:2", "image:1", "scene:1", "image:5", "scene:5"}},
	} {
		t.Run(test.direction.String(), func(t *testing.T) {
			var got []string
			for page := 1; page <= 4; page++ {
				result := p07Query(t, repository, nil, &models.FindFilterType{Sort: &sort, Direction: &test.direction, Page: &page, PerPage: &size})
				require.Equal(t, 10, result.Count)
				require.Equal(t, 5, result.ImageCount)
				require.Equal(t, 5, result.VideoCount)
				got = append(got, p07Keys(result)...)
			}
			require.Equal(t, test.want, got, "ties cross page boundaries without duplication or omission")
			page := 5
			result := p07Query(t, repository, nil, &models.FindFilterType{Page: &page, PerPage: &size})
			require.Empty(t, result.Items)
			require.Equal(t, 10, result.Count, "a past-the-end page retains the global count")
		})
	}
	for _, test := range []struct {
		sort  string
		first []string
	}{
		{"title", []string{"scene:1", "image:1", "scene:2", "image:2"}},
		{"path", []string{"scene:1", "image:1"}},
		{"filesize", []string{"image:1", "scene:1"}},
		{"created_at", []string{"image:1", "image:2", "image:3", "image:4"}},
		{"updated_at", []string{"image:1", "image:2", "image:3", "image:4"}},
		{"rating", []string{"image:2", "image:3", "image:4", "scene:2"}},
	} {
		t.Run(test.sort, func(t *testing.T) {
			result := p07Query(t, repository, nil, &models.FindFilterType{Sort: &test.sort})
			require.Equal(t, test.first, p07Keys(result)[:len(test.first)])
		})
	}
}

func TestP07SharedFiltersAndKindCounts(t *testing.T) {
	repository := p07Fixture(t)
	q, organized := "needle", true
	for _, filter := range []*models.MediaFilterType{
		{Details: &models.StringCriterionInput{Value: q, Modifier: models.CriterionModifierIncludes}},
		{Rating100: &models.IntCriterionInput{Value: 80, Modifier: models.CriterionModifierGreaterThan}},
		{Date: &models.DateCriterionInput{Value: "2026-01-04", Modifier: models.CriterionModifierGreaterThan}},
		{Organized: &organized},
		{Path: &models.StringCriterionInput{Value: "item", Modifier: models.CriterionModifierIncludes}},
	} {
		result := p07Query(t, repository, filter, nil)
		require.Equal(t, 2, result.Count)
		require.Equal(t, 1, result.ImageCount)
		require.Equal(t, 1, result.VideoCount)
		require.Equal(t, []string{"image:1", "scene:1"}, p07Keys(result))
	}
	result := p07Query(t, repository, nil, &models.FindFilterType{Q: &q})
	require.Equal(t, []string{"image:1", "scene:1"}, p07Keys(result))
	for _, kind := range []models.MediaKind{models.MediaKindImage, models.MediaKindVideo} {
		result := p07Query(t, repository, &models.MediaFilterType{MediaTypes: []models.MediaKind{kind}}, nil)
		require.Equal(t, 5, result.Count)
		for _, item := range result.Items {
			require.Equal(t, kind, item.Kind)
		}
	}
	missing := p07Query(t, repository, &models.MediaFilterType{Title: &models.StringCriterionInput{Value: "absent", Modifier: models.CriterionModifierEquals}}, nil)
	require.Empty(t, missing.Items)
	require.Zero(t, missing.Count)
	require.Zero(t, missing.ImageCount)
	require.Zero(t, missing.VideoCount)
	for _, modifier := range []models.CriterionModifier{models.CriterionModifierGreaterThan, models.CriterionModifierIsNull} {
		filter := &models.MediaFilterType{Duration: &models.IntCriterionInput{Value: 30, Modifier: modifier}}
		result := p07Query(t, repository, filter, nil)
		require.Zero(t, result.ImageCount, "duration, even IS_NULL, excludes Images")
		filter.MediaTypes = []models.MediaKind{models.MediaKindImage}
		require.Zero(t, p07Query(t, repository, filter, nil).Count)
	}
}

func TestP07SharedAssociationFiltersReuseNativeInheritance(t *testing.T) {
	f := newP06WorkflowFixture(t)
	f.manual(t, false)
	depth := -1
	for _, filter := range []*models.MediaFilterType{
		{Tags: &models.HierarchicalMultiCriterionInput{Value: []string{strconv.Itoa(f.parentTag.ID)}, Modifier: models.CriterionModifierIncludes}},
		{Performers: &models.MultiCriterionInput{Value: []string{strconv.Itoa(f.characterRoot.ID)}, Modifier: models.CriterionModifierIncludes}},
		{Studios: &models.HierarchicalMultiCriterionInput{Value: []string{strconv.Itoa(f.artistRoot.ID)}, Modifier: models.CriterionModifierIncludes, Depth: &depth}},
		{Copyrights: &models.HierarchicalMultiCriterionInput{Value: []string{strconv.Itoa(f.copyrightRoot.ID)}, Modifier: models.CriterionModifierIncludes, Depth: &depth}},
	} {
		result := p07Query(t, f.repository, filter, nil)
		require.Equal(t, []string{"image:1", "scene:1"}, p07Keys(result))
		require.Equal(t, 2, result.Count, "multiple sources/diamonds count each native media once")
	}
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Performer.UpdatePartial(ctx, f.character.ID, models.PerformerPartial{Favorite: models.NewOptionalBool(true)})
		return err
	}))
	favorite := true
	require.Equal(t, 2, p07Query(t, f.repository, &models.MediaFilterType{PerformerFavorite: &favorite}, nil).Count)
	// A profile edit changes the next mixed read; no inherited Tag is stored.
	config.GetInstance().SetBool(config.AssociationInheritanceTags, false)
	result := p07Query(t, f.repository, &models.MediaFilterType{Tags: &models.HierarchicalMultiCriterionInput{
		Value: []string{strconv.Itoa(f.parentTag.ID)}, Modifier: models.CriterionModifierIncludes,
	}}, nil)
	require.Zero(t, result.Count)
}

func p07GraphQL(t *testing.T, repository models.Repository, query string, variables map[string]any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	require.NoError(t, err)
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{repository: repository, hookExecutor: p06NoHooks{}}}))
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
	return result.Data
}

func TestP07GraphQLTypedIdentityAndNativeBulkWriters(t *testing.T) {
	repository := p07Fixture(t)
	data := p07GraphQL(t, repository, `query { findMedia { count image_count video_count items { id kind image { id title } scene { id title } } } findImages { count } findScenes { count } }`, nil)
	var result struct {
		FindMedia struct {
			Count int
			Items []struct {
				ID    string
				Kind  models.MediaKind
				Image *struct{ ID, Title string }
				Scene *struct{ ID, Title string }
			}
		}
		FindImages, FindScenes struct{ Count int }
	}
	require.NoError(t, json.Unmarshal(data, &result))
	require.Equal(t, 10, result.FindMedia.Count)
	require.Equal(t, 5, result.FindImages.Count)
	require.Equal(t, 5, result.FindScenes.Count)
	for _, item := range result.FindMedia.Items {
		if item.Kind == models.MediaKindImage {
			require.NotNil(t, item.Image)
			require.Nil(t, item.Scene)
			require.Equal(t, "image:"+item.Image.ID, item.ID)
		} else {
			require.NotNil(t, item.Scene)
			require.Nil(t, item.Image)
			require.Equal(t, "scene:"+item.Scene.ID, item.ID)
		}
	}
	p07GraphQL(t, repository, `mutation($input: BulkImageUpdateInput!) { bulkImageUpdate(input: $input) { id } }`, map[string]any{"input": map[string]any{"ids": []string{"1"}, "title": "edited image", "organized": false}})
	p07GraphQL(t, repository, `mutation($input: BulkSceneUpdateInput!) { bulkSceneUpdate(input: $input) { id } }`, map[string]any{"input": map[string]any{"ids": []string{"2"}, "title": "edited video", "organized": true}})
	require.NoError(t, repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		image1, err := repository.Image.Find(ctx, 1)
		require.NoError(t, err)
		image2, err := repository.Image.Find(ctx, 2)
		require.NoError(t, err)
		scene1, err := repository.Scene.Find(ctx, 1)
		require.NoError(t, err)
		scene2, err := repository.Scene.Find(ctx, 2)
		require.NoError(t, err)
		require.Equal(t, "edited image", image1.Title)
		require.False(t, image1.Organized)
		require.Equal(t, "Item 4", image2.Title)
		require.Equal(t, "Item 1", scene1.Title)
		require.Equal(t, "edited video", scene2.Title)
		require.True(t, scene2.Organized)
		return nil
	}))
	// Empty and single-kind pages also hydrate safely.
	for _, kind := range []string{"IMAGE", "VIDEO"} {
		p07GraphQL(t, repository, `query($kind: MediaKind!) { findMedia(media_filter: {media_types: [$kind]}) { items { id image {id} scene {id} } } }`, map[string]any{"kind": kind})
	}
	p07GraphQL(t, repository, `query { findMedia(filter: {page: 100}) { count items {id} } }`, nil)
}

func TestP07RejectsUnboundedPagesAndUnsupportedSorts(t *testing.T) {
	repository := p07Fixture(t)
	for _, size := range []int{-1, -2, 0, 501} {
		require.ErrorContains(t, repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
			_, err := repository.Media.Query(ctx, nil, &models.FindFilterType{PerPage: &size})
			return err
		}), "media page size")
	}
	for _, sort := range []string{"random", "duration", "date; DROP TABLE images"} {
		require.ErrorContains(t, repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
			_, err := repository.Media.Query(ctx, nil, &models.FindFilterType{Sort: &sort})
			return err
		}), "unsupported media sort")
	}
	page, size := int(^uint(0)>>1), 500
	require.ErrorContains(t, repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		_, err := repository.Media.Query(ctx, nil, &models.FindFilterType{Page: &page, PerPage: &size})
		return err
	}), "media page is too large")
}
