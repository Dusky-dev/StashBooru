package api

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/models"
)

type randomImageQueryer interface {
	image.Queryer
	image.QueryCounter
}

// randomRelatedImage returns one image attached to an entity without loading
// the entity's complete image collection. The random page makes the fallback
// vary between page loads while keeping each query to a single image.
func randomRelatedImage(
	ctx context.Context,
	queryer randomImageQueryer,
	filter *models.ImageFilterType,
) (*models.Image, error) {
	count, err := queryer.QueryCount(ctx, filter, nil)
	if err != nil || count == 0 {
		return nil, err
	}

	perPage := 1
	page := rand.Intn(count) + 1
	findFilter := &models.FindFilterType{
		Page:    &page,
		PerPage: &perPage,
	}

	images, err := image.Query(ctx, queryer, filter, findFilter)
	if err != nil || len(images) == 0 {
		return nil, err
	}
	return images[0], nil
}

func randomID(ids []int) int {
	if len(ids) == 0 {
		return 0
	}
	return ids[rand.Intn(len(ids))]
}

func imageThumbnailURL(baseURL string, imageID int) string {
	return fmt.Sprintf(
		"%s/image/%d/thumbnail",
		strings.TrimRight(baseURL, "/"),
		imageID,
	)
}
