package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type FollowingStore struct{}

func (s *FollowingStore) Checkpoint(ctx context.Context, id string, revision int) (models.FollowingCheckpoint, error) {
	var state models.FollowingCheckpoint
	err := dbWrapper.Get(ctx, &state, `SELECT version,cursor,has_more,checked_at,last_error,error_at FROM following_checkpoints WHERE follow_id=? AND revision=?`, id, revision)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	return state, err
}

func (s *FollowingStore) ApplyPage(ctx context.Context, follow models.BooruFollow, version int, cursor string, more bool, posts []models.FollowingSnapshot) (models.FollowingCheckpoint, error) {
	var empty models.FollowingCheckpoint
	if version < 0 || len(cursor) > 4096 || len(posts) > 50 {
		return empty, models.ErrFollowingInvalid
	}
	ids := map[string]bool{}
	for _, post := range posts {
		if post.PostID == "" || len(post.PostID) > 256 || ids[post.PostID] || len(post.Payload) > models.MaxFollowingPayload || !json.Valid(post.Payload) {
			return empty, models.ErrFollowingInvalid
		}
		ids[post.PostID] = true
	}
	current, err := (&BooruFollowStore{}).Find(ctx, follow.ID)
	if err != nil {
		return empty, err
	}
	if current == nil || !current.Active || current.Version != follow.Version || current.QueryRevision != follow.QueryRevision || current.Instance != follow.Instance {
		return empty, models.ErrBooruFollowConflict
	}
	state, err := s.Checkpoint(ctx, follow.ID, follow.QueryRevision)
	if err != nil {
		return empty, err
	}
	if state.Version != version {
		return empty, models.ErrBooruFollowConflict
	}
	if more && cursor == state.Cursor {
		return empty, models.ErrFollowingInvalid
	}
	var count int
	if err := dbWrapper.Get(ctx, &count, `SELECT count(*) FROM following_posts`); err != nil {
		return empty, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, post := range posts {
		var existing int
		if err := dbWrapper.Get(ctx, &existing, `SELECT count(*) FROM following_posts WHERE instance=? AND post_id=?`, follow.Instance, post.PostID); err != nil {
			return empty, err
		}
		if existing == 0 {
			if count >= models.MaxFollowingPosts {
				return empty, models.ErrFollowingLimit
			}
			count++
		}
		_, err := dbWrapper.Exec(ctx, `INSERT INTO following_posts(instance,post_id,payload,retrieved_at) VALUES(?,?,?,?)
 ON CONFLICT(instance,post_id) DO UPDATE SET payload=excluded.payload,retrieved_at=excluded.retrieved_at`, follow.Instance, post.PostID, post.Payload, now)
		if err != nil {
			return empty, err
		}
		_, err = dbWrapper.Exec(ctx, `INSERT INTO following_matches(sequence,follow_id,revision,name,kind,target)
 SELECT sequence,?,?,?,?,? FROM following_posts WHERE instance=? AND post_id=?
 ON CONFLICT(sequence,follow_id,revision) DO NOTHING`, follow.ID, follow.QueryRevision, follow.Name, follow.Kind, follow.Target, follow.Instance, post.PostID)
		if err != nil {
			return empty, err
		}
	}
	_, err = dbWrapper.Exec(ctx, `INSERT INTO following_checkpoints(follow_id,revision,version,cursor,has_more,checked_at) VALUES(?,?,?,?,?,?)
 ON CONFLICT(follow_id,revision) DO UPDATE SET version=excluded.version,cursor=excluded.cursor,has_more=excluded.has_more,checked_at=excluded.checked_at,last_error='',error_at=''`,
		follow.ID, follow.QueryRevision, version+1, cursor, more, now)
	if err != nil {
		return empty, err
	}
	return models.FollowingCheckpoint{Version: version + 1, Cursor: cursor, HasMore: more, CheckedAt: now}, nil
}

func (s *FollowingStore) List(ctx context.Context, filter models.FollowingFilter) ([]models.FollowingPost, error) {
	if filter.Limit < 1 || filter.Limit > 51 || filter.Before < 0 {
		return nil, models.ErrFollowingInvalid
	}
	conditions := []string{"1=1"}
	args := []any{}
	if filter.Before != 0 {
		conditions = append(conditions, "p.sequence<?")
		args = append(args, filter.Before)
	}
	match := "m.sequence=p.sequence"
	if !filter.History {
		match += " AND f.active=1 AND f.query_revision=m.revision"
	}
	if filter.FollowID != "" {
		match += " AND m.follow_id=?"
		args = append(args, filter.FollowID)
	}
	conditions = append(conditions, `EXISTS(SELECT 1 FROM following_matches m JOIN booru_follows f ON f.id=m.follow_id WHERE `+match+`)`)
	if filter.Unseen {
		conditions = append(conditions, "COALESCE(s.seen_at,'')=''")
	}
	if !filter.ShowIgnored {
		conditions = append(conditions, "COALESCE(r.ignored_at,'')=''")
	}
	if filter.HideDownloaded {
		conditions = append(conditions, "COALESCE(r.downloaded_at,'')=''")
	}
	args = append(args, filter.Limit)
	rows := []models.FollowingPost{}
	err := dbWrapper.Select(ctx, &rows, `SELECT p.*,COALESCE(s.version,0) AS seen_version,COALESCE(s.seen_at,'') AS seen_at,
 COALESCE(r.ignored_at,'') AS ignored_at,COALESCE(r.downloaded_at,'') AS downloaded_at
 FROM following_posts p LEFT JOIN following_seen s ON s.instance=p.instance AND s.post_id=p.post_id
 LEFT JOIN booru_review r ON r.instance=p.instance AND r.post_id=p.post_id
 WHERE `+strings.Join(conditions, " AND ")+` ORDER BY p.sequence DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Matches = []models.FollowingMatch{}
		err := dbWrapper.Select(ctx, &rows[i].Matches, `SELECT m.follow_id,m.revision,f.name,m.kind,m.target,
 (f.active=1 AND f.query_revision=m.revision) AS current FROM following_matches m JOIN booru_follows f ON f.id=m.follow_id
 WHERE m.sequence=? ORDER BY (m.follow_id=?) DESC,current DESC,m.follow_id,m.revision DESC LIMIT 51`, rows[i].Sequence, filter.FollowID)
		if err != nil {
			return nil, err
		}
		if len(rows[i].Matches) > 50 {
			rows[i].MatchesTruncated = true
			rows[i].Matches = rows[i].Matches[:50]
		}
	}
	return rows, nil
}

func (s *FollowingStore) SetSeen(ctx context.Context, sequence int64, version int, seen bool) (int, string, error) {
	if sequence < 1 || version < 0 {
		return 0, "", models.ErrFollowingInvalid
	}
	var identity struct{ Instance, PostID string }
	err := dbWrapper.Get(ctx, &identity, `SELECT instance,post_id AS postid FROM following_posts WHERE sequence=?`, sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", models.ErrBooruFollowConflict
	}
	if err != nil {
		return 0, "", err
	}
	var current int
	err = dbWrapper.Get(ctx, &current, `SELECT version FROM following_seen WHERE instance=? AND post_id=?`, identity.Instance, identity.PostID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, "", err
	}
	if current != version {
		return 0, "", models.ErrBooruFollowConflict
	}
	at := ""
	if seen {
		at = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err = dbWrapper.Exec(ctx, `INSERT INTO following_seen(instance,post_id,version,seen_at) VALUES(?,?,?,?)
 ON CONFLICT(instance,post_id) DO UPDATE SET version=excluded.version,seen_at=excluded.seen_at`, identity.Instance, identity.PostID, version+1, at)
	return version + 1, at, err
}

func (s *FollowingStore) PruneSeen(ctx context.Context) (int64, error) {
	result, err := dbWrapper.Exec(ctx, `DELETE FROM following_posts WHERE EXISTS(SELECT 1 FROM following_seen s
 WHERE s.instance=following_posts.instance AND s.post_id=following_posts.post_id AND s.seen_at<>'')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// A failed attempt updates diagnostics, never the resume cursor or last successful
// check time. The same CAS rules prevent a late failure from replacing success.
func (s *FollowingStore) RecordError(ctx context.Context, follow models.BooruFollow, version int, message string) error {
	if version < 0 || message == "" || len(message) > 1024 {
		return models.ErrFollowingInvalid
	}
	current, err := (&BooruFollowStore{}).Find(ctx, follow.ID)
	if err != nil {
		return err
	}
	if current == nil || !current.Active || current.Version != follow.Version || current.QueryRevision != follow.QueryRevision || current.Instance != follow.Instance {
		return models.ErrBooruFollowConflict
	}
	state, err := s.Checkpoint(ctx, follow.ID, follow.QueryRevision)
	if err != nil {
		return err
	}
	if state.Version != version {
		return models.ErrBooruFollowConflict
	}
	_, err = dbWrapper.Exec(ctx, `INSERT INTO following_checkpoints(follow_id,revision,version,cursor,has_more,checked_at,last_error,error_at) VALUES(?,?,?,?,?,?,?,?)
 ON CONFLICT(follow_id,revision) DO UPDATE SET version=excluded.version,last_error=excluded.last_error,error_at=excluded.error_at`, follow.ID, follow.QueryRevision, version+1, state.Cursor, state.HasMore, state.CheckedAt, message, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
