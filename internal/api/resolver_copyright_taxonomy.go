package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *copyrightResolver) OrderedChildren(ctx context.Context, obj *models.Copyright) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindOrderedChildren(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) StructuralRole(ctx context.Context, obj *models.Copyright) (ret string, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.StructuralRole(ctx, obj.ID)
		return err
	}); err != nil {
		return "", err
	}
	return ret, nil
}

func (r *copyrightResolver) Breadcrumb(ctx context.Context, obj *models.Copyright) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.Breadcrumb(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *copyrightResolver) SubtreeImageCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.ImageCountDepth(ctx, obj.ID, -1)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *copyrightResolver) SubtreeSceneCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.SceneCountDepth(ctx, obj.ID, -1)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *copyrightResolver) SubtreePerformerCount(ctx context.Context, obj *models.Copyright) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.PerformerCountDepth(ctx, obj.ID, -1)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *copyrightResolver) SubtreeImages(ctx context.Context, obj *models.Copyright) (ret []*models.Image, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindImageIDsDepth(ctx, obj.ID, -1)
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

func (r *copyrightResolver) SubtreeScenes(ctx context.Context, obj *models.Copyright) (ret []*models.Scene, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindSceneIDsDepth(ctx, obj.ID, -1)
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

func (r *copyrightResolver) SubtreePerformers(ctx context.Context, obj *models.Copyright) (ret []*models.Performer, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ids, findErr := r.repository.Copyright.FindPerformerIDsDepth(ctx, obj.ID, -1)
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

func (r *tagResolver) StructuralRole(ctx context.Context, obj *models.Tag) (ret string, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.TagStructuralRole(ctx, obj.ID)
		return err
	}); err != nil {
		return "", err
	}
	return ret, nil
}

func (r *imageResolver) OrderedCopyrights(ctx context.Context, obj *models.Image) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindByImageIDOrdered(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *sceneResolver) OrderedCopyrights(ctx context.Context, obj *models.Scene) (ret []*models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Copyright.FindBySceneIDOrdered(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func findPrimaryCopyright(ctx context.Context, repository models.Repository, id *int) (*models.Copyright, error) {
	if id == nil {
		return nil, nil
	}
	return repository.Copyright.Find(ctx, *id)
}

func (r *imageResolver) PrimaryCopyright(ctx context.Context, obj *models.Image) (ret *models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		id, findErr := r.repository.Copyright.PrimaryImageCopyrightID(ctx, obj.ID)
		if findErr != nil {
			return findErr
		}
		ret, err = findPrimaryCopyright(ctx, r.repository, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *sceneResolver) PrimaryCopyright(ctx context.Context, obj *models.Scene) (ret *models.Copyright, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		id, findErr := r.repository.Copyright.PrimarySceneCopyrightID(ctx, obj.ID)
		if findErr != nil {
			return findErr
		}
		ret, err = findPrimaryCopyright(ctx, r.repository, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func optionalCopyrightID(raw *string) (*int, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := strconv.Atoi(*raw)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid Copyright id %q", *raw)
	}
	return &id, nil
}

func (r *mutationResolver) CopyrightStructuralRoleUpdate(ctx context.Context, copyrightID string, role string) (ret *models.Copyright, err error) {
	id, err := strconv.Atoi(copyrightID)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid Copyright id %q", copyrightID)
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetStructuralRole(ctx, id, role); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) TagStructuralRoleUpdate(ctx context.Context, tagID string, role string) (ret *models.Tag, err error) {
	id, err := strconv.Atoi(tagID)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid Tag id %q", tagID)
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetTagStructuralRole(ctx, id, role); err != nil {
			return err
		}
		ret, err = r.repository.Tag.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) CopyrightChildrenOrderUpdate(ctx context.Context, parentID string, childIds []string) (ret *models.Copyright, err error) {
	parent, err := strconv.Atoi(parentID)
	if err != nil || parent <= 0 {
		return nil, fmt.Errorf("invalid Copyright id %q", parentID)
	}
	ids, err := copyrightIDsToInts(childIds)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetChildOrder(ctx, parent, ids); err != nil {
			return err
		}
		ret, err = r.repository.Copyright.Find(ctx, parent)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) ImagePrimaryCopyrightUpdate(ctx context.Context, imageID string, copyrightID *string) (ret *models.Image, err error) {
	id, err := strconv.Atoi(imageID)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid image id %q", imageID)
	}
	primaryID, err := optionalCopyrightID(copyrightID)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetPrimaryImageCopyright(ctx, id, primaryID); err != nil {
			return err
		}
		ret, err = r.repository.Image.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *mutationResolver) ScenePrimaryCopyrightUpdate(ctx context.Context, sceneID string, copyrightID *string) (ret *models.Scene, err error) {
	id, err := strconv.Atoi(sceneID)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid video id %q", sceneID)
	}
	primaryID, err := optionalCopyrightID(copyrightID)
	if err != nil {
		return nil, err
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if err := r.repository.Copyright.SetPrimarySceneCopyright(ctx, id, primaryID); err != nil {
			return err
		}
		ret, err = r.repository.Scene.Find(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
