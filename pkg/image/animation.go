package image

import (
	"context"

	"github.com/stashapp/stash/pkg/models"
)

type AnimationTags interface {
	models.TagNameFinder
	models.TagCreator
}

type AnimationImages interface {
	UpdatePartial(context.Context, int, models.ImagePartial) (*models.Image, error)
}

// AddAnimatedTag runs in the caller's write transaction. Existing relationships
// are preserved, and a user-supplied animated tag is reused case-insensitively.
func AddAnimatedTag(ctx context.Context, tags AnimationTags, images AnimationImages, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	tag, err := tags.FindByName(ctx, "animated", true)
	if err != nil {
		return err
	}
	if tag == nil {
		created := models.NewTag()
		created.Name = "animated"
		if err := tags.Create(ctx, &models.CreateTagInput{Tag: &created}); err != nil {
			return err
		}
		tag = &created
	}
	for _, id := range ids {
		partial := models.NewImagePartial()
		partial.TagIDs = &models.UpdateIDs{IDs: []int{tag.ID}, Mode: models.RelationshipUpdateModeAdd}
		if _, err := images.UpdatePartial(ctx, id, partial); err != nil {
			return err
		}
	}
	return nil
}
