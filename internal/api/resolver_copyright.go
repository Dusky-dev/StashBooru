package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
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
			return nil, fmt.Errorf("converting id %q: %w", raw, err)
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
	var imageData []byte
	if input.Image != nil {
		imageData, err = utils.ProcessImageInput(ctx, *input.Image)
		if err != nil {
			return nil, fmt.Errorf("processing image: %w", err)
		}
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Create(ctx, input)
		if err != nil {
			return err
		}
		if len(imageData) > 0 {
			if err := r.repository.Copyright.UpdateImage(ctx, ret.ID, imageData); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightUpdate(ctx context.Context, input models.CopyrightUpdateInput) (ret *models.Copyright, err error) {
	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}
	imageIncluded := translator.hasField("image")
	var imageData []byte
	if input.Image != nil {
		imageData, err = utils.ProcessImageInput(ctx, *input.Image)
		if err != nil {
			return nil, fmt.Errorf("processing image: %w", err)
		}
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Update(ctx, input)
		if err != nil {
			return err
		}
		if imageIncluded {
			if err := r.repository.Copyright.UpdateImage(ctx, ret.ID, imageData); err != nil {
				return err
			}
		}
		return nil
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

func (r *mutationResolver) PerformerCopyrightsUpdate(ctx context.Context, performerID string, copyrightIds []string) (ret *models.Performer, err error) {
	id, err := strconv.Atoi(performerID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(copyrightIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetPerformerCopyrights(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Performer.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightImagesUpdate(ctx context.Context, copyrightID string, imageIds []string) (ret *models.Copyright, err error) {
	id, err := strconv.Atoi(copyrightID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(imageIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetCopyrightImages(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightScenesUpdate(ctx context.Context, copyrightID string, sceneIds []string) (ret *models.Copyright, err error) {
	id, err := strconv.Atoi(copyrightID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(sceneIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetCopyrightScenes(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightPerformersUpdate(ctx context.Context, copyrightID string, performerIds []string) (ret *models.Copyright, err error) {
	id, err := strconv.Atoi(copyrightID)
	if err != nil {
		return nil, err
	}
	ids, err := copyrightIDsToInts(performerIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetCopyrightPerformers(ctx, id, ids); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) ImagePath(ctx context.Context, obj *models.Copyright) (string, error) {
	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)
	return fmt.Sprintf("%s/tag/copyright/%d/image", strings.TrimRight(baseURL, "/"), obj.ID), nil
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

func (r *copyrightResolver) PerformerCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.PerformerCount(ctx, obj.ID)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *copyrightResolver) Images(ctx context.Context, obj *models.Copyright) (ret []*models.Image, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindImageIDs(ctx, obj.ID)
		if findErr != nil {
			return findErr
		}
		if len(ids) == 0 {
			ret = []*models.Image{}
			return nil
		}
		ret, err = r.repository.Image.FindMany(ctx, ids)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) Scenes(ctx context.Context, obj *models.Copyright) (ret []*models.Scene, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindSceneIDs(ctx, obj.ID)
		if findErr != nil {
			return findErr
		}
		if len(ids) == 0 {
			ret = []*models.Scene{}
			return nil
		}
		ret, err = r.repository.Scene.FindMany(ctx, ids)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) Performers(ctx context.Context, obj *models.Copyright) (ret []*models.Performer, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindPerformerIDs(ctx, obj.ID)
		if findErr != nil {
			return findErr
		}
		if len(ids) == 0 {
			ret = []*models.Performer{}
			return nil
		}
		ret, err = r.repository.Performer.FindMany(ctx, ids)
		return err
	}); err != nil {
		return nil, err
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

func (r *performerResolver) Copyrights(ctx context.Context, obj *models.Performer) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindByPerformerID(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
