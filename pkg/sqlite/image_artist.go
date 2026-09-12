package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

type ImageArtistStore struct {
	studios models.StudioReader
}

func NewImageArtistStore(studios models.StudioReader) *ImageArtistStore {
	return &ImageArtistStore{studios: studios}
}

func uniqueArtistIDs(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	ret := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ret = append(ret, id)
	}
	return ret
}

func (s *ImageArtistStore) findIDs(ctx context.Context, imageID int) ([]int, error) {
	var primary sql.NullInt64
	if err := dbWrapper.Get(ctx, &primary, "SELECT studio_id FROM images WHERE id = ?", imageID); err != nil {
		return nil, err
	}

	var rows []struct {
		ID int `db:"studio_id"`
	}
	if err := dbWrapper.Select(ctx, &rows,
		"SELECT studio_id FROM images_artists WHERE image_id = ? ORDER BY position, studio_id", imageID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	ids := make([]int, 0, len(rows)+1)
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if primary.Valid {
		ids = append(ids, int(primary.Int64))
	}
	return uniqueArtistIDs(ids), nil
}

func (s *ImageArtistStore) FindByImageID(ctx context.Context, imageID int) ([]*models.Studio, error) {
	ids, err := s.findIDs(ctx, imageID)
	if err != nil || len(ids) == 0 {
		return []*models.Studio{}, err
	}

	items, err := s.studios.FindMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*models.Studio, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ret := make([]*models.Studio, 0, len(ids))
	for _, id := range ids {
		if item := byID[id]; item != nil {
			ret = append(ret, item)
		}
	}
	return ret, nil
}

func (s *ImageArtistStore) SetImageArtists(ctx context.Context, imageID int, studioIDs []int) error {
	ids := uniqueArtistIDs(studioIDs)
	if len(ids) > 0 {
		studios, err := s.studios.FindMany(ctx, ids)
		if err != nil {
			return err
		}
		if len(studios) != len(ids) {
			return fmt.Errorf("one or more Artist IDs do not exist")
		}
	}

	if _, err := dbWrapper.Exec(ctx, "DELETE FROM images_artists WHERE image_id = ?", imageID); err != nil {
		return err
	}
	for position, studioID := range ids {
		if _, err := dbWrapper.Exec(ctx,
			"INSERT INTO images_artists (image_id, studio_id, position) VALUES (?, ?, ?)",
			imageID, studioID, position); err != nil {
			return err
		}
	}

	if len(ids) == 0 {
		_, err := dbWrapper.Exec(ctx, "UPDATE images SET studio_id = NULL WHERE id = ?", imageID)
		return err
	}
	_, err := dbWrapper.Exec(ctx, "UPDATE images SET studio_id = ? WHERE id = ?", ids[0], imageID)
	return err
}

func (s *ImageArtistStore) AddImageArtists(ctx context.Context, imageID int, studioIDs []int) error {
	current, err := s.findIDs(ctx, imageID)
	if err != nil {
		return err
	}
	return s.SetImageArtists(ctx, imageID, append(current, studioIDs...))
}
