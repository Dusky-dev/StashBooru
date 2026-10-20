package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/mattn/go-sqlite3"
	"github.com/stashapp/stash/pkg/models"
)

type BooruFollowStore struct{}

const booruFollowColumns = `id,instance,source_id,engine,source_url,name,kind,target,version,query_revision,active,created_at,updated_at`

func (s *BooruFollowStore) List(ctx context.Context) ([]models.BooruFollow, error) {
	rows := []models.BooruFollow{}
	err := dbWrapper.Select(ctx, &rows, `SELECT `+booruFollowColumns+` FROM booru_follows ORDER BY created_at,id LIMIT ?`, models.MaxBooruFollows)
	return rows, err
}

func (s *BooruFollowStore) Find(ctx context.Context, id string) (*models.BooruFollow, error) {
	var row models.BooruFollow
	err := dbWrapper.Get(ctx, &row, `SELECT `+booruFollowColumns+` FROM booru_follows WHERE id=?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func booruFollowWriteError(err error) error {
	var constraint sqlite3.Error
	if errors.As(err, &constraint) && constraint.ExtendedCode == sqlite3.ErrConstraintUnique {
		return models.ErrBooruFollowDuplicate
	}
	return err
}

// Call inside a repository write transaction, including the capacity check.
func (s *BooruFollowStore) Create(ctx context.Context, provider models.BooruReviewProvider, definition models.BooruFollowDefinition) (*models.BooruFollow, error) {
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	if provider.Instance == "" || provider.SourceID == "" || provider.Engine == "" || provider.URL == "" {
		return nil, models.ErrBooruFollowInvalid
	}
	var count int
	if err := dbWrapper.Get(ctx, &count, `SELECT count(*) FROM booru_follows`); err != nil {
		return nil, err
	}
	if count >= models.MaxBooruFollows {
		return nil, models.ErrBooruFollowLimit
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := rand.Text()
	_, err := dbWrapper.Exec(ctx, `INSERT INTO booru_follows (`+booruFollowColumns+`) VALUES(?,?,?,?,?,?,?,?,1,1,1,?,?)`,
		id, provider.Instance, provider.SourceID, provider.Engine, provider.URL, definition.Name, definition.Kind, definition.Target, now, now)
	if err != nil {
		return nil, booruFollowWriteError(err)
	}
	return s.Find(ctx, id)
}

// Updates use optimistic concurrency. Unfollow retains the row and its revision;
// refollow cannot reset versions or touch independent review/download history.
func (s *BooruFollowStore) Update(ctx context.Context, id string, version int, definition *models.BooruFollowDefinition, active *bool) (*models.BooruFollow, error) {
	if version < 1 || (definition == nil) == (active == nil) {
		return nil, models.ErrBooruFollowInvalid
	}
	if definition != nil {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
	}
	row, err := s.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil || row.Version != version {
		return nil, models.ErrBooruFollowConflict
	}
	if definition != nil {
		if row.Kind != definition.Kind || row.Target != definition.Target {
			row.QueryRevision++
		}
		row.BooruFollowDefinition = *definition
	} else {
		row.Active = *active
	}
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	result, err := dbWrapper.Exec(ctx, `UPDATE booru_follows SET name=?,kind=?,target=?,active=?,query_revision=?,version=version+1,updated_at=? WHERE id=? AND version=?`,
		row.Name, row.Kind, row.Target, row.Active, row.QueryRevision, row.UpdatedAt, id, version)
	if err != nil {
		return nil, booruFollowWriteError(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, models.ErrBooruFollowConflict
	}
	row.Version++
	return row, nil
}
