package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

const performersCopyrightsTable = "performers_copyrights"

func (s *CopyrightStore) GetImage(ctx context.Context, id int) ([]byte, error) {
	var image []byte
	if err := dbWrapper.Get(ctx, &image, "SELECT image_blob FROM copyrights WHERE id = ? AND image_blob IS NOT NULL", id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return image, nil
}

func (s *CopyrightStore) HasImage(ctx context.Context, id int) (bool, error) {
	var count int
	if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM copyrights WHERE id = ? AND image_blob IS NOT NULL", id); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *CopyrightStore) UpdateImage(ctx context.Context, id int, image []byte) error {
	if len(image) == 0 {
		_, err := dbWrapper.Exec(ctx, "UPDATE copyrights SET image_blob = NULL, updated_at = ? WHERE id = ?", time.Now(), id)
		return err
	}
	_, err := dbWrapper.Exec(ctx, "UPDATE copyrights SET image_blob = ?, updated_at = ? WHERE id = ?", image, time.Now(), id)
	return err
}

func (s *CopyrightStore) PerformerCount(ctx context.Context, id int) (int, error) {
	return copyrightRelationCount(ctx, performersCopyrightsTable, "copyright_id", id)
}

func (s *CopyrightStore) FindByPerformerID(ctx context.Context, performerID int) ([]*models.Copyright, error) {
	return s.findForMedia(ctx, performersCopyrightsTable, "performer_id", performerID)
}

func findCopyrightMediaIDs(ctx context.Context, table, mediaColumn string, copyrightID int) ([]int, error) {
	var ids []int
	if err := dbWrapper.Select(ctx, &ids, "SELECT "+mediaColumn+" FROM "+table+" WHERE copyright_id = ? ORDER BY "+mediaColumn, copyrightID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return ids, nil
}

func (s *CopyrightStore) FindImageIDs(ctx context.Context, copyrightID int) ([]int, error) {
	return findCopyrightMediaIDs(ctx, imagesCopyrightsTable, "image_id", copyrightID)
}

func (s *CopyrightStore) FindSceneIDs(ctx context.Context, copyrightID int) ([]int, error) {
	return findCopyrightMediaIDs(ctx, scenesCopyrightsTable, "scene_id", copyrightID)
}

func (s *CopyrightStore) FindPerformerIDs(ctx context.Context, copyrightID int) ([]int, error) {
	return findCopyrightMediaIDs(ctx, performersCopyrightsTable, "performer_id", copyrightID)
}

func (s *CopyrightStore) SetPerformerCopyrights(ctx context.Context, performerID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, performersCopyrightsTable, "performer_id", performerID, copyrightIDs, true)
}

func (s *CopyrightStore) AddPerformerCopyrights(ctx context.Context, performerID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, performersCopyrightsTable, "performer_id", performerID, copyrightIDs, false)
}

func setCopyrightMedia(ctx context.Context, table, mediaColumn string, copyrightID int, mediaIDs []int) error {
	if _, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE copyright_id = ?", copyrightID); err != nil {
		return err
	}
	for _, mediaID := range mediaIDs {
		if _, err := dbWrapper.Exec(ctx, "INSERT OR IGNORE INTO "+table+" ("+mediaColumn+", copyright_id) VALUES (?, ?)", mediaID, copyrightID); err != nil {
			return err
		}
	}
	return nil
}

func (s *CopyrightStore) SetCopyrightImages(ctx context.Context, copyrightID int, imageIDs []int) error {
	return setCopyrightMedia(ctx, imagesCopyrightsTable, "image_id", copyrightID, imageIDs)
}

func (s *CopyrightStore) SetCopyrightScenes(ctx context.Context, copyrightID int, sceneIDs []int) error {
	return setCopyrightMedia(ctx, scenesCopyrightsTable, "scene_id", copyrightID, sceneIDs)
}

func (s *CopyrightStore) SetCopyrightPerformers(ctx context.Context, copyrightID int, performerIDs []int) error {
	return setCopyrightMedia(ctx, performersCopyrightsTable, "performer_id", copyrightID, performerIDs)
}
