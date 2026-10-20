package models

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxBooruFollows = 500

var (
	ErrBooruFollowInvalid   = errors.New("invalid follow definition")
	ErrBooruFollowConflict  = errors.New("follow changed")
	ErrBooruFollowDuplicate = errors.New("follow already exists")
	ErrBooruFollowLimit     = errors.New("follow storage is full")
)

// Targets are exact provider names or provider-native queries. A category is the
// user's intent, not a verified provider category or a native catalogue mapping.
type BooruFollowDefinition struct {
	Name   string `json:"name" db:"name"`
	Kind   string `json:"kind" db:"kind"`
	Target string `json:"target" db:"target"`
}

func (d BooruFollowDefinition) Validate() error {
	validText := func(s string, limit int) bool {
		return utf8.ValidString(s) && len(s) <= limit && strings.TrimSpace(s) != "" && !strings.ContainsFunc(s, unicode.IsControl)
	}
	if !validText(d.Name, 120) || !validText(d.Target, 1024) {
		return ErrBooruFollowInvalid
	}
	switch d.Kind {
	case "artist", "character", "copyright", "query":
		return nil
	default:
		return ErrBooruFollowInvalid
	}
}

type BooruFollow struct {
	BooruFollowDefinition
	ID            string `json:"id" db:"id"`
	Instance      string `json:"instance" db:"instance"`
	SourceID      string `json:"source" db:"source_id"`
	Engine        string `json:"engine" db:"engine"`
	SourceURL     string `json:"sourceURL" db:"source_url"`
	Version       int    `json:"version" db:"version"`
	QueryRevision int    `json:"queryRevision" db:"query_revision"`
	Active        bool   `json:"active" db:"active"`
	CreatedAt     string `json:"createdAt" db:"created_at"`
	UpdatedAt     string `json:"updatedAt" db:"updated_at"`
}

type BooruFollowReaderWriter interface {
	List(context.Context) ([]BooruFollow, error)
	Find(context.Context, string) (*BooruFollow, error)
	Create(context.Context, BooruReviewProvider, BooruFollowDefinition) (*BooruFollow, error)
	// Exactly one of definition/active must be supplied. Source binding is immutable.
	Update(context.Context, string, int, *BooruFollowDefinition, *bool) (*BooruFollow, error)
}
