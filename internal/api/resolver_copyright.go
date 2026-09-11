package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

type copyrightResolver struct{ *Resolver }

func (r *Resolver) Copyright() CopyrightResolver {
	return &copyrightResolver{r}
}

func copyrightIDsToInts(ids []string) ([]int, error) {
	ret := make([]int, 0, len(ids))
	for _, raw := range ids {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("converting copyright id %q: %w", raw, err)
		}
		ret = append(ret, id)
	}
	return ret, nil
}

func (r *queryResolver) FindCopyright(ctx context.Context, id string) (ret *models.Copyright, err error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindCopyrights(ctx context.Context, filter *models.FindFilterType, ids []string) (ret *models.FindCopyrightsResultType, err error) {
	idInts, err := copyrightIDsToInts(ids)
	if err != nil {
		return nil, err
	}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var items []*models.Copyright
		var count int
		if len(idInts) > 0 {
			items, err = r.repository.Copyright.FindMany(ctx, idInts)
			count = len(items)
		} else {
			items, count, err = r.repository.Copyright.Query(ctx, filter)
		}
		if err != nil {
			return err
		}
		ret = &models.FindCopyrightsResultType{Count: count, Copyrights: items}
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightCreate(ctx context.Context, input models.CopyrightCreateInput) (ret *models.Copyright, err error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Create(ctx, input)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightUpdate(ctx context.Context, input models.CopyrightUpdateInput) (ret *models.Copyright, err error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Update(ctx, input)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightDestroy(ctx context.Context, input models.CopyrightDestroyInput) (bool, error) {
	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return false, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.Copyright.Destroy(ctx, id)
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) ImageCopyrightsUpdate(ctx context.Context, imageID string, copyrightIds []string) (ret *models.Image, err error) {
	id, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(copyrightIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetImageCopyrights(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Image.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) SceneCopyrightsUpdate(ctx context.Context, sceneID string, copyrightIds []string) (ret *models.Scene, err error) {
	id, err := strconv.Atoi(sceneID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(copyrightIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetSceneCopyrights(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Scene.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) Parents(ctx context.Context, obj *models.Copyright) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindParents(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) Children(ctx context.Context, obj *models.Copyright) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindChildren(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) ImageCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.ImageCount(ctx, obj.ID)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *copyrightResolver) SceneCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.SceneCount(ctx, obj.ID)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *imageResolver) Copyrights(ctx context.Context, obj *models.Image) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindByImageID(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *sceneResolver) Copyrights(ctx context.Context, obj *models.Scene) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindBySceneID(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
