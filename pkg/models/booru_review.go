package models

import (
	"context"
	"errors"
)

var ErrBooruReviewConflict = errors.New("booru review state changed")

type BooruReviewState struct {
	PostID       string `json:"id" db:"post_id"`
	Version      int    `json:"version" db:"version"`
	IgnoredAt    string `json:"ignoredAt" db:"ignored_at"`
	DownloadedAt string `json:"downloadedAt" db:"downloaded_at"`
	UpdatedAt    string `json:"updatedAt" db:"updated_at"`
}

type BooruReviewChange struct {
	PostID     string `json:"id"`
	Version    int    `json:"version"`
	Ignored    *bool  `json:"ignored,omitempty"`
	Downloaded *bool  `json:"downloaded,omitempty"`
}

type BooruReviewProvider struct{ Instance, SourceID, Engine, URL string }

type BooruLocalCandidate struct {
	Kind   MediaKind `db:"kind"`
	ID     int       `db:"id"`
	FileID FileID    `db:"file_id"`
	MD5    string    `db:"md5"`
}

type BooruReviewReaderWriter interface {
	Read(context.Context, string, []string) ([]BooruReviewState, error)
	Update(context.Context, BooruReviewProvider, []BooruReviewChange) ([]BooruReviewState, error)
	Candidates(context.Context, []string, int) ([]BooruLocalCandidate, error)
}
