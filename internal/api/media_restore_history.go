package api

import (
	"context"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/txn"
)

type mediaRestoreRecord struct {
	*mediaconvert.Record
	MediaKind string `json:"mediaKind,omitempty"`
	MediaID   int    `json:"mediaID,omitempty"`
}

func decorateMediaRestoreRecords(ctx context.Context, records []*mediaconvert.Record) ([]mediaRestoreRecord, error) {
	mgr := manager.GetInstance()
	ret := make([]mediaRestoreRecord, len(records))
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		for i, record := range records {
			ret[i].Record = record
			before := record.Before.File()
			if before == nil {
				continue
			}
			fileID := before.Base().ID
			images, err := mgr.Repository.Image.FindByFileID(ctx, fileID)
			if err != nil {
				return err
			}
			if len(images) == 1 {
				ret[i].MediaKind = "image"
				ret[i].MediaID = images[0].ID
				continue
			}
			scenes, err := mgr.Repository.Scene.FindByFileID(ctx, fileID)
			if err != nil {
				return err
			}
			if len(scenes) == 1 {
				ret[i].MediaKind = "scene"
				ret[i].MediaID = scenes[0].ID
			}
		}
		return nil
	})
	return ret, err
}

func normalizeRestoreHistoryFingerprints(records []*mediaconvert.Record) {
	// JSON numbers cannot represent every 64-bit pHash exactly in browsers.
	for _, record := range records {
		for _, snap := range []mediaconvert.Snapshot{record.Before, record.After} {
			if f := snap.File(); f != nil {
				for i, fp := range f.Base().Fingerprints {
					f.Base().Fingerprints[i].Fingerprint = fp.Value()
				}
			}
		}
	}
}
