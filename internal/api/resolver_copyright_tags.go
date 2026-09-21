package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *copyrightResolver) Tags(ctx context.Context, obj *models.Copyright) (ret []*models.Tag, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, err := r.repository.Copyright.GetTagIDs(ctx, obj.ID)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			ret = []*models.Tag{}
			return nil
		}
		ret, err = r.repository.Tag.FindMany(ctx, ids)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightTagsUpdate(ctx context.Context, copyrightID string, tagIds []string) (ret *models.Copyright, err error) {
	id, err := strconv.Atoi(copyrightID)
	if err != nil {
		return nil, err
	}
	tagIDs, err := copyrightIDsToInts(tagIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.UpdateTags(ctx, id, tagIDs); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
