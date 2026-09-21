package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/stashapp/stash/pkg/models"
)

const copyrightsTagsTable = "copyrights_tags"

func (s *CopyrightStore) GetTagIDs(ctx context.Context, id int) ([]int, error) {
	var ids []int
	if err := dbWrapper.Select(ctx, &ids,
		"SELECT tag_id FROM "+copyrightsTagsTable+" WHERE copyright_id = ? ORDER BY tag_id", id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return ids, nil
}

func (s *CopyrightStore) UpdateTags(ctx context.Context, id int, tagIDs []int) error {
	if _, err := dbWrapper.Exec(ctx, "DELETE FROM "+copyrightsTagsTable+" WHERE copyright_id = ?", id); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := dbWrapper.Exec(ctx,
			"INSERT OR IGNORE INTO "+copyrightsTagsTable+" (copyright_id, tag_id) VALUES (?, ?)", id, tagID); err != nil {
			return err
		}
	}
	return nil
}

var _ models.CopyrightReaderWriter = (*CopyrightStore)(nil)
