package autotag

import (
	"context"

	"github.com/stashapp/stash/pkg/match"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

func getCopyrightTaggers(copyright *models.Copyright, cache *match.Cache) []tagger {
	ret := []tagger{{
		ID:    copyright.ID,
		Type:  "copyright",
		Name:  copyright.Name,
		cache: cache,
	}}

	for _, alias := range copyright.Aliases {
		ret = append(ret, tagger{
			ID:    copyright.ID,
			Type:  "copyright",
			Name:  alias,
			cache: cache,
		})
	}

	return ret
}

func hasCopyright(items []*models.Copyright, id int) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

// CopyrightScenes searches for videos whose path matches the Copyright name or
// one of its aliases and attaches the Copyright when it is not already set.
func (tagger *Tagger) CopyrightScenes(
	ctx context.Context,
	copyright *models.Copyright,
	paths []string,
	sceneReader models.SceneQueryer,
	copyrightRW models.CopyrightReaderWriter,
) error {
	for _, matcher := range getCopyrightTaggers(copyright, tagger.Cache) {
		if err := matcher.tagScenes(ctx, paths, sceneReader, func(scene *models.Scene) (bool, error) {
			existing, err := copyrightRW.FindBySceneID(ctx, scene.ID)
			if err != nil {
				return false, err
			}
			if hasCopyright(existing, copyright.ID) {
				return false, nil
			}

			if err := txn.WithTxn(ctx, tagger.TxnManager, func(ctx context.Context) error {
				return copyrightRW.AddSceneCopyrights(ctx, scene.ID, []int{copyright.ID})
			}); err != nil {
				return false, err
			}
			return true, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// CopyrightImages searches for images whose path matches the Copyright name or
// one of its aliases and attaches the Copyright when it is not already set.
func (tagger *Tagger) CopyrightImages(
	ctx context.Context,
	copyright *models.Copyright,
	paths []string,
	imageReader models.ImageQueryer,
	copyrightRW models.CopyrightReaderWriter,
) error {
	for _, matcher := range getCopyrightTaggers(copyright, tagger.Cache) {
		if err := matcher.tagImages(ctx, paths, imageReader, func(image *models.Image) (bool, error) {
			existing, err := copyrightRW.FindByImageID(ctx, image.ID)
			if err != nil {
				return false, err
			}
			if hasCopyright(existing, copyright.ID) {
				return false, nil
			}

			if err := txn.WithTxn(ctx, tagger.TxnManager, func(ctx context.Context) error {
				return copyrightRW.AddImageCopyrights(ctx, image.ID, []int{copyright.ID})
			}); err != nil {
				return false, err
			}
			return true, nil
		}); err != nil {
			return err
		}
	}
	return nil
}
