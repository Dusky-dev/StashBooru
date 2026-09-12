package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *mutationResolver) ImageArtistsUpdate(ctx context.Context, imageID string, artistIds []string) (ret *models.Image, err error) {
	id, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(artistIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.ImageArtist.SetImageArtists(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Image.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *imageResolver) Artists(ctx context.Context, obj *models.Image) (ret []*models.Studio, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.ImageArtist.FindByImageID(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
