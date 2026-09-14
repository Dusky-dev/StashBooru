package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/txn"
)

type camieBulkSelection struct {
	ImageIDs []int
	All      bool
}

type camieBulkImageSource struct {
	ID   int
	Path string
}

func resolveCamieBulkImageIDs(ctx context.Context, mgr *manager.Manager, selection camieBulkSelection) ([]int, error) {
	if !selection.All {
		seen := make(map[int]struct{}, len(selection.ImageIDs))
		ids := make([]int, 0, len(selection.ImageIDs))
		for _, id := range selection.ImageIDs {
			if id <= 0 {
				return nil, fmt.Errorf("invalid image id %d", id)
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		return ids, nil
	}

	var ids []int
	if err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		images, err := mgr.Repository.Image.All(ctx)
		if err != nil {
			return err
		}
		ids = make([]int, 0, len(images))
		for _, image := range images {
			ids = append(ids, image.ID)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("loading images for Camie tagging: %w", err)
	}
	return ids, nil
}

func loadCamieBulkImageSource(ctx context.Context, mgr *manager.Manager, imageID int) (camieBulkImageSource, error) {
	var source camieBulkImageSource
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		image, err := mgr.Repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("image %d not found", imageID)
		}
		if err := image.LoadPrimaryFile(ctx, mgr.Repository.File); err != nil {
			return err
		}
		primary := image.Files.Primary()
		if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
			return fmt.Errorf("image %d has no primary file", imageID)
		}
		source = camieBulkImageSource{ID: imageID, Path: primary.Base().Path}
		return nil
	})
	return source, err
}
