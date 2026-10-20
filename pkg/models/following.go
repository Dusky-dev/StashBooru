package models

import (
	"context"
	"errors"
)

const MaxFollowingPosts = 5000
const MaxFollowingPayload = 64 << 10

var ErrFollowingLimit = errors.New("following snapshot cache is full")
var ErrFollowingInvalid = errors.New("invalid following page or decision")

// Cursor and Payload belong to the provider adapter. Shared persistence does not
// assume numeric remote IDs, booru tags, timestamp ordering or account handles.
type FollowingCheckpoint struct {
	Version   int    `json:"version" db:"version"`
	Cursor    string `json:"-" db:"cursor"`
	CheckedAt string `json:"checkedAt" db:"checked_at"`
	HasMore   bool   `json:"hasMore" db:"has_more"`
	LastError string `json:"lastError" db:"last_error"`
	ErrorAt   string `json:"errorAt" db:"error_at"`
}

type FollowingSnapshot struct {
	PostID  string
	Payload []byte
}

type FollowingMatch struct {
	FollowID string `json:"followID" db:"follow_id"`
	Revision int    `json:"revision" db:"revision"`
	Name     string `json:"name" db:"name"`
	Kind     string `json:"kind" db:"kind"`
	Target   string `json:"target" db:"target"`
	Current  bool   `json:"current" db:"current"`
}

type FollowingPost struct {
	Sequence         int64            `json:"sequence,string" db:"sequence"`
	Instance         string           `json:"instance" db:"instance"`
	PostID           string           `json:"postID" db:"post_id"`
	Payload          []byte           `json:"-" db:"payload"`
	RetrievedAt      string           `json:"retrievedAt" db:"retrieved_at"`
	SeenVersion      int              `json:"seenVersion" db:"seen_version"`
	SeenAt           string           `json:"seenAt" db:"seen_at"`
	IgnoredAt        string           `json:"ignoredAt" db:"ignored_at"`
	DownloadedAt     string           `json:"downloadedAt" db:"downloaded_at"`
	Matches          []FollowingMatch `json:"matches"`
	MatchesTruncated bool             `json:"matchesTruncated" db:"-"`
}

type FollowingFilter struct {
	Before         int64
	Limit          int
	FollowID       string
	Unseen         bool
	History        bool
	ShowIgnored    bool
	HideDownloaded bool
}

type FollowingReaderWriter interface {
	Checkpoint(context.Context, string, int) (FollowingCheckpoint, error)
	// ApplyPage must share one write transaction with its capacity, follow-version
	// and checkpoint-version checks. A failed page never advances the cursor.
	ApplyPage(context.Context, BooruFollow, int, string, bool, []FollowingSnapshot) (FollowingCheckpoint, error)
	List(context.Context, FollowingFilter) ([]FollowingPost, error)
	SetSeen(context.Context, int64, int, bool) (int, string, error)
	PruneSeen(context.Context) (int64, error)
	RecordError(context.Context, BooruFollow, int, string) error
}
