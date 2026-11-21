package api

import (
	"context"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/models"
)

// An Image's or Video's Date, when empty, is filled from the best source
// available, in this order: when the photo was taken (EXIF, Images only),
// then when it was posted on the site. Items with neither keep an empty Date
// and count by date added. A Date already set is never changed.
//
// Videos skip embedded creation times: they are usually the encode time.
func resolveMediaDate(kind, path, postedAt string, now time.Time) (*models.Date, string) {
	if kind == "image" && path != "" {
		if at, ok := image.CaptureDate(path, now); ok {
			return &models.Date{Time: at, Precision: models.DatePrecisionDay}, "file"
		}
	}
	if at, ok := booru.PostDate(postedAt, now); ok {
		return &models.Date{Time: at, Precision: models.DatePrecisionDay}, "posted"
	}
	return nil, ""
}

// fillMissingMediaDate sets the item's Date if it has none. It returns which
// source was used ("file", "posted") or "" when nothing changed.
func fillMissingMediaDate(ctx context.Context, repo models.Repository, kind string, id int, path, postedAt string) (string, error) {
	switch kind {
	case "image":
		item, err := repo.Image.Find(ctx, id)
		if err != nil || item == nil || item.Date != nil {
			return "", err
		}
		date, from := resolveMediaDate(kind, path, postedAt, time.Now())
		if date == nil {
			return "", nil
		}
		_, err = repo.Image.UpdatePartial(ctx, id, models.ImagePartial{Date: models.NewOptionalDate(*date)})
		return from, err
	case "scene":
		item, err := repo.Scene.Find(ctx, id)
		if err != nil || item == nil || item.Date != nil {
			return "", err
		}
		date, from := resolveMediaDate(kind, path, postedAt, time.Now())
		if date == nil {
			return "", nil
		}
		_, err = repo.Scene.UpdatePartial(ctx, id, models.ScenePartial{Date: models.NewOptionalDate(*date)})
		return from, err
	}
	return "", nil
}
