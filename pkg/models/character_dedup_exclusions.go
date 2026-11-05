package models

import "context"

// CharacterDedupPair records a user decision that two native Characters must
// never be automatically merged. IDs are ordered to make the pair symmetric.
type CharacterDedupPair struct {
	LeftID  int `json:"leftId" db:"left_id"`
	RightID int `json:"rightId" db:"right_id"`
}

type CharacterDedupExclusionReaderWriter interface {
	List(context.Context) ([]CharacterDedupPair, error)
	Add(context.Context, []CharacterDedupPair) error
	Remove(context.Context, []CharacterDedupPair) error
}
