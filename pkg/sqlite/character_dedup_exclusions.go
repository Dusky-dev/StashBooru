package sqlite

import (
	"context"
	"errors"

	"github.com/stashapp/stash/pkg/models"
)

type CharacterDedupExclusionStore struct{}

func (s *CharacterDedupExclusionStore) List(ctx context.Context) ([]models.CharacterDedupPair, error) {
	result := []models.CharacterDedupPair{}
	err := dbWrapper.Select(ctx, &result, "SELECT left_id, right_id FROM character_dedup_exclusions ORDER BY left_id, right_id")
	return result, err
}

func validCharacterDedupPair(pair models.CharacterDedupPair) bool {
	return pair.LeftID > 0 && pair.RightID > pair.LeftID
}

func (s *CharacterDedupExclusionStore) Add(ctx context.Context, pairs []models.CharacterDedupPair) error {
	if len(pairs) == 0 {
		return errors.New("select Character pairs to mark as not duplicates")
	}
	for _, pair := range pairs {
		if !validCharacterDedupPair(pair) {
			return errors.New("invalid Character pair")
		}
		if _, err := dbWrapper.Exec(ctx, "INSERT INTO character_dedup_exclusions(left_id,right_id) VALUES(?,?) ON CONFLICT DO NOTHING", pair.LeftID, pair.RightID); err != nil {
			return err
		}
	}
	return nil
}

func (s *CharacterDedupExclusionStore) Remove(ctx context.Context, pairs []models.CharacterDedupPair) error {
	if len(pairs) == 0 {
		return errors.New("select Character pairs to reconsider")
	}
	for _, pair := range pairs {
		if !validCharacterDedupPair(pair) {
			return errors.New("invalid Character pair")
		}
		result, err := dbWrapper.Exec(ctx, "DELETE FROM character_dedup_exclusions WHERE left_id=? AND right_id=?", pair.LeftID, pair.RightID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return errors.New("Character not-duplicate decision changed; preview again")
		}
	}
	return nil
}
