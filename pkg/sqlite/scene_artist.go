package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

type SceneArtistStore struct {
	studios models.StudioReader
}

func NewSceneArtistStore(studios models.StudioReader) *SceneArtistStore {
	return &SceneArtistStore{studios: studios}
}

func (s *SceneArtistStore) findIDs(ctx context.Context, sceneID int) ([]int, error) {
	var primary sql.NullInt64
	if err := dbWrapper.Get(ctx, &primary, "SELECT studio_id FROM scenes WHERE id = ?", sceneID); err != nil {
		return nil, err
	}

	var rows []struct {
		ID int `db:"studio_id"`
	}
	if err := dbWrapper.Select(ctx, &rows,
		"SELECT studio_id FROM scenes_artists WHERE scene_id = ? ORDER BY position, studio_id", sceneID); err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func (s *SceneArtistStore) FindBySceneID(ctx context.Context, sceneID int) ([]*models.Studio, error) {
	ids, err := s.findIDs(ctx, sceneID)
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

func (s *SceneArtistStore) SetSceneArtists(ctx context.Context, sceneID int, studioIDs []int) error {
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

	if _, err := dbWrapper.Exec(ctx, "DELETE FROM scenes_artists WHERE scene_id = ?", sceneID); err != nil {
		return err
	}
	for position, studioID := range ids {
		if _, err := dbWrapper.Exec(ctx,
			"INSERT INTO scenes_artists (scene_id, studio_id, position) VALUES (?, ?, ?)",
			sceneID, studioID, position); err != nil {
			return err
		}
	}

	if len(ids) == 0 {
		_, err := dbWrapper.Exec(ctx, "UPDATE scenes SET studio_id = NULL WHERE id = ?", sceneID)
		return err
	}
	_, err := dbWrapper.Exec(ctx, "UPDATE scenes SET studio_id = ? WHERE id = ?", ids[0], sceneID)
	return err
}

func (s *SceneArtistStore) AddSceneArtists(ctx context.Context, sceneID int, studioIDs []int) error {
	current, err := s.findIDs(ctx, sceneID)
	if err != nil {
		return err
	}
	return s.SetSceneArtists(ctx, sceneID, append(current, studioIDs...))
}
