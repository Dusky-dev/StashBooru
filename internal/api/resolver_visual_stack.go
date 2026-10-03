package api

import (
	"context"
	"github.com/stashapp/stash/pkg/models"
	"strconv"
)

type visualStackResolver struct{ *Resolver }
type visualStackMemberResolver struct{ *Resolver }

func (r *Resolver) VisualStack() VisualStackResolver { return &visualStackResolver{r} }
func (r *Resolver) VisualStackMember() VisualStackMemberResolver {
	return &visualStackMemberResolver{r}
}

func readVisualStack(ctx context.Context, r *Resolver, fn func(context.Context) (*models.VisualStack, error)) (ret *models.VisualStack, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = fn(ctx); return err })
	return
}
func writeVisualStack(ctx context.Context, r *Resolver, fn func(context.Context) (*models.VisualStack, error)) (ret *models.VisualStack, err error) {
	err = r.withTxn(ctx, func(ctx context.Context) error { ret, err = fn(ctx); return err })
	return
}
func (r *queryResolver) FindVisualStack(ctx context.Context, id string) (*models.VisualStack, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	return readVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) { return r.repository.VisualStack.Find(ctx, n) })
}
func (r *queryResolver) FindVisualStackForMedia(ctx context.Context, media models.MediaReference) (*models.VisualStack, error) {
	return readVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.FindByMedia(ctx, media)
	})
}
func (r *queryResolver) VisualStackProposals(ctx context.Context, media []*models.MediaReference) (ret []*models.VisualStackProposal, err error) {
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = r.repository.VisualStack.Propose(ctx, media); return err })
	return
}
func (r *imageResolver) VisualStack(ctx context.Context, obj *models.Image) (*models.VisualStack, error) {
	return readVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.FindByMedia(ctx, models.MediaReference{Kind: models.MediaKindImage, ID: obj.ID})
	})
}
func (r *sceneResolver) VisualStack(ctx context.Context, obj *models.Scene) (*models.VisualStack, error) {
	return readVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.FindByMedia(ctx, models.MediaReference{Kind: models.MediaKindVideo, ID: obj.ID})
	})
}
func (r *visualStackResolver) Members(ctx context.Context, obj *models.VisualStack) (ret []*models.VisualStackMember, err error) {
	if obj.Members != nil {
		return obj.Members, nil
	}
	stack, err := readVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.Find(ctx, obj.ID)
	})
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return []*models.VisualStackMember{}, nil
	}
	return stack.Members, nil
}
func (r *visualStackMemberResolver) Image(ctx context.Context, obj *models.VisualStackMember) (ret *models.Image, err error) {
	if obj.Media.Kind != models.MediaKindImage {
		return nil, nil
	}
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = r.repository.Image.Find(ctx, obj.Media.ID); return err })
	return
}
func (r *visualStackMemberResolver) Scene(ctx context.Context, obj *models.VisualStackMember) (ret *models.Scene, err error) {
	if obj.Media.Kind != models.MediaKindVideo {
		return nil, nil
	}
	err = r.withReadTxn(ctx, func(ctx context.Context) error { ret, err = r.repository.Scene.Find(ctx, obj.Media.ID); return err })
	return
}
func (r *mutationResolver) VisualStackCreate(ctx context.Context, input models.VisualStackCreateInput) (*models.VisualStack, error) {
	return writeVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.Create(ctx, input)
	})
}
func (r *mutationResolver) VisualStackUpdate(ctx context.Context, input models.VisualStackUpdateInput) (*models.VisualStack, error) {
	return writeVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.Update(ctx, input)
	})
}
func (r *mutationResolver) VisualStackSplit(ctx context.Context, input models.VisualStackSplitInput) (*models.VisualStack, error) {
	return writeVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.Split(ctx, input)
	})
}
func (r *mutationResolver) VisualStackMerge(ctx context.Context, input models.VisualStackMergeInput) (*models.VisualStack, error) {
	return writeVisualStack(ctx, r.Resolver, func(ctx context.Context) (*models.VisualStack, error) {
		return r.repository.VisualStack.Merge(ctx, input)
	})
}
func (r *mutationResolver) VisualStackDestroy(ctx context.Context, input models.VisualStackVersionInput) (bool, error) {
	err := r.withTxn(ctx, func(ctx context.Context) error { return r.repository.VisualStack.Destroy(ctx, input) })
	return err == nil, err
}
