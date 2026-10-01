package api

import (
	"context"

	"github.com/stashapp/stash/pkg/models"
)

type MediaItem struct {
	ID    string
	Kind  models.MediaKind
	Image *models.Image
	Scene *models.Scene
}

func (r *queryResolver) FindMedia(ctx context.Context, mediaFilter *models.MediaFilterType, filter *models.FindFilterType) (ret *FindMediaResultType, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		result, err := r.repository.Media.Query(ctx, mediaFilter, filter)
		if err != nil {
			return err
		}
		var imageIDs, sceneIDs []int
		for _, row := range result.Items {
			if row.Kind == models.MediaKindImage {
				imageIDs = append(imageIDs, row.ID)
			} else {
				sceneIDs = append(sceneIDs, row.ID)
			}
		}
		images, err := r.repository.Image.FindMany(ctx, imageIDs)
		if err != nil {
			return err
		}
		scenes, err := r.repository.Scene.FindMany(ctx, sceneIDs)
		if err != nil {
			return err
		}
		imageMap := make(map[int]*models.Image, len(images))
		for _, image := range images {
			imageMap[image.ID] = image
		}
		sceneMap := make(map[int]*models.Scene, len(scenes))
		for _, scene := range scenes {
			sceneMap[scene.ID] = scene
		}
		ret = &FindMediaResultType{Count: result.Count, ImageCount: result.ImageCount, VideoCount: result.VideoCount, Items: []*MediaItem{}}
		for _, row := range result.Items {
			// Equal numeric IDs in the two native tables must not leak across kinds.
			item := &MediaItem{ID: row.Key(), Kind: row.Kind}
			if row.Kind == models.MediaKindImage {
				item.Image = imageMap[row.ID]
			} else {
				item.Scene = sceneMap[row.ID]
			}
			ret.Items = append(ret.Items, item)
		}
		return nil
	})
	return ret, err
}
