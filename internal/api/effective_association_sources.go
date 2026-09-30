package api

import (
	"context"

	"github.com/stashapp/stash/pkg/models"
)

// Callers own the transaction so a review uses one consistent library snapshot.
func imageDirectMediaAssociations(ctx context.Context, repository models.Repository, image *models.Image) (directMediaAssociationIDs, error) {
	if !image.TagIDs.Loaded() {
		if err := image.LoadTagIDs(ctx, repository.Image); err != nil {
			return directMediaAssociationIDs{}, err
		}
	}
	if !image.PerformerIDs.Loaded() {
		if err := image.LoadPerformerIDs(ctx, repository.Image); err != nil {
			return directMediaAssociationIDs{}, err
		}
	}
	copyrights, err := repository.Copyright.FindByImageIDOrdered(ctx, image.ID)
	if err != nil {
		return directMediaAssociationIDs{}, err
	}
	artists, err := repository.ImageArtist.FindByImageID(ctx, image.ID)
	if err != nil {
		return directMediaAssociationIDs{}, err
	}
	return directMediaAssociations(image.TagIDs.List(), image.PerformerIDs.List(), artists, copyrights, image.StudioID), nil
}

func sceneDirectMediaAssociations(ctx context.Context, repository models.Repository, scene *models.Scene) (directMediaAssociationIDs, error) {
	if !scene.TagIDs.Loaded() {
		if err := scene.LoadTagIDs(ctx, repository.Scene); err != nil {
			return directMediaAssociationIDs{}, err
		}
	}
	if !scene.PerformerIDs.Loaded() {
		if err := scene.LoadPerformerIDs(ctx, repository.Scene); err != nil {
			return directMediaAssociationIDs{}, err
		}
	}
	copyrights, err := repository.Copyright.FindBySceneIDOrdered(ctx, scene.ID)
	if err != nil {
		return directMediaAssociationIDs{}, err
	}
	artists, err := repository.SceneArtist.FindBySceneID(ctx, scene.ID)
	if err != nil {
		return directMediaAssociationIDs{}, err
	}
	return directMediaAssociations(scene.TagIDs.List(), scene.PerformerIDs.List(), artists, copyrights, scene.StudioID), nil
}

func directMediaAssociations(tags, performers []int, artists []*models.Studio, copyrights []*models.Copyright, primaryArtist *int) directMediaAssociationIDs {
	artistIDs := uniqueRelatedModelIDs(artists, func(artist *models.Studio) int { return artist.ID })
	if primaryArtist != nil {
		seen := make(map[int]struct{}, len(artistIDs))
		for _, id := range artistIDs {
			seen[id] = struct{}{}
		}
		artistIDs = appendUniqueIDs(artistIDs, seen, []int{*primaryArtist})
	}
	return directMediaAssociationIDs{
		tags:       tags,
		artists:    artistIDs,
		performers: performers,
		copyrights: uniqueRelatedModelIDs(copyrights, func(copyright *models.Copyright) int { return copyright.ID }),
	}
}
