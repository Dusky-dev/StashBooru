package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type BooruReviewStore struct{}

func (s *BooruReviewStore) Read(ctx context.Context, instance string, ids []string) ([]models.BooruReviewState, error) {
	if len(ids) > 50 {
		return nil, fmt.Errorf("review pages support at most 50 posts")
	}
	ret := make([]models.BooruReviewState, 0, len(ids))
	for _, id := range ids {
		row := models.BooruReviewState{PostID: id}
		err := dbWrapper.Get(ctx, &row, "SELECT post_id,version,ignored_at,downloaded_at,updated_at FROM booru_review WHERE instance=? AND post_id=?", instance, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		ret = append(ret, row)
	}
	return ret, nil
}

// Update is called inside the repository write transaction. Retain cleared rows
// as versioned tombstones: clearing and re-marking must not permit an ABA edit.
func (s *BooruReviewStore) Update(ctx context.Context, provider models.BooruReviewProvider, changes []models.BooruReviewChange) ([]models.BooruReviewState, error) {
	if len(changes) < 1 || len(changes) > 50 {
		return nil, fmt.Errorf("select between 1 and 50 posts")
	}
	ids := make([]string, len(changes))
	seen := map[string]bool{}
	for i, change := range changes {
		if change.PostID == "" || change.Version < 0 || seen[change.PostID] || (change.Ignored == nil && change.Downloaded == nil) {
			return nil, fmt.Errorf("invalid review change")
		}
		seen[change.PostID] = true
		ids[i] = change.PostID
	}
	rows, err := s.Read(ctx, provider.Instance, ids)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		if row.Version != changes[i].Version {
			return nil, models.ErrBooruReviewConflict
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range rows {
		row, change := &rows[i], changes[i]
		if change.Ignored != nil {
			if !*change.Ignored {
				row.IgnoredAt = ""
			} else if row.IgnoredAt == "" {
				row.IgnoredAt = now
			}
		}
		if change.Downloaded != nil {
			if !*change.Downloaded {
				row.DownloadedAt = ""
			} else if row.DownloadedAt == "" {
				row.DownloadedAt = now
			}
		}
		previous := row.Version
		row.Version++
		row.UpdatedAt = now
		result, err := dbWrapper.Exec(ctx, `INSERT INTO booru_review(instance,post_id,source_id,engine,source_url,version,ignored_at,downloaded_at,updated_at)
   VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(instance,post_id) DO UPDATE SET
   source_id=excluded.source_id,engine=excluded.engine,source_url=excluded.source_url,version=excluded.version,
   ignored_at=excluded.ignored_at,downloaded_at=excluded.downloaded_at,updated_at=excluded.updated_at WHERE booru_review.version=?`,
			provider.Instance, row.PostID, provider.SourceID, provider.Engine, provider.URL, row.Version, row.IgnoredAt, row.DownloadedAt, row.UpdatedAt, previous)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, models.ErrBooruReviewConflict
		}
	}
	return rows, nil
}

// Candidates only consider active primary files with native MD5 fingerprints.
// A candidate is not evidence until its current bytes have been checked.
func (s *BooruReviewStore) Candidates(ctx context.Context, hashes []string, limit int) ([]models.BooruLocalCandidate, error) {
	if len(hashes) == 0 {
		return []models.BooruLocalCandidate{}, nil
	}
	if len(hashes) > 50 || limit < 1 || limit > 101 {
		return nil, fmt.Errorf("invalid candidate bounds")
	}
	query, args, err := sqlx.In(`SELECT ref.kind,ref.id,ref.file_id,fp.fingerprint AS md5 FROM (
  SELECT 'IMAGE' AS kind,i.id,j.file_id FROM images i JOIN images_files j ON j.image_id=i.id AND j."primary"=1
  UNION ALL SELECT 'VIDEO' AS kind,s.id,j.file_id FROM scenes s JOIN scenes_files j ON j.scene_id=s.id AND j."primary"=1
 ) ref JOIN files_fingerprints fp ON fp.file_id=ref.file_id AND fp.type='md5'
 WHERE fp.fingerprint IN (?) ORDER BY fp.fingerprint,ref.kind,ref.id LIMIT ?`, hashes, limit)
	if err != nil {
		return nil, err
	}
	var rows []models.BooruLocalCandidate
	if err := dbWrapper.Select(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}
