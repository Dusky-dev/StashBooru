package mediaconvert

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

type ModelRepository struct{ Repository models.Repository }

func (r ModelRepository) Get(ctx context.Context, id models.FileID) (models.File, error) {
	var ret models.File
	err := txn.WithReadTxn(ctx, r.Repository.TxnManager, func(ctx context.Context) error {
		files, err := r.Repository.File.Find(ctx, id)
		if len(files) > 0 {
			ret = files[0]
		}
		return err
	})
	return ret, err
}

func (r ModelRepository) Swap(ctx context.Context, before, after models.File) error {
	return txn.WithTxn(ctx, r.Repository.TxnManager, func(ctx context.Context) error {
		files, err := r.Repository.File.Find(ctx, before.Base().ID)
		if err != nil {
			return err
		}
		if len(files) != 1 || !SameFile(files[0], before) {
			return fmt.Errorf("file record changed during conversion; retry after rescanning")
		}
		return r.Repository.File.Update(ctx, after)
	})
}
