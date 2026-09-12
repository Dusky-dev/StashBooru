package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

const (
	copyrightTable          = "copyrights"
	copyrightAliasesTable   = "copyright_aliases"
	copyrightRelationsTable = "copyright_relations"
	imagesCopyrightsTable   = "images_copyrights"
	scenesCopyrightsTable   = "scenes_copyrights"
)

type copyrightRow struct {
	ID          int       `db:"id"`
	Name        string    `db:"name"`
	SortName    string    `db:"sort_name"`
	Description string    `db:"description"`
	Favorite    bool      `db:"favorite"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r copyrightRow) model() *models.Copyright {
	return &models.Copyright{
		ID:          r.ID,
		Name:        r.Name,
		SortName:    r.SortName,
		Description: r.Description,
		Favorite:    r.Favorite,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type CopyrightStore struct{}

func NewCopyrightStore() *CopyrightStore { return &CopyrightStore{} }

func (s *CopyrightStore) loadAliases(ctx context.Context, item *models.Copyright) error {
	var aliases []string
	if err := dbWrapper.Select(ctx, &aliases,
		"SELECT alias FROM copyright_aliases WHERE copyright_id = ? ORDER BY alias COLLATE NOCASE", item.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	item.Aliases = aliases
	return nil
}

func (s *CopyrightStore) Find(ctx context.Context, id int) (*models.Copyright, error) {
	var row copyrightRow
	if err := dbWrapper.Get(ctx, &row,
		"SELECT id, name, sort_name, description, favorite, created_at, updated_at FROM copyrights WHERE id = ?", id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ret := row.model()
	if err := s.loadAliases(ctx, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *CopyrightStore) FindMany(ctx context.Context, ids []int) ([]*models.Copyright, error) {
	if len(ids) == 0 {
		return []*models.Copyright{}, nil
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := fmt.Sprintf("SELECT id, name, sort_name, description, favorite, created_at, updated_at FROM copyrights WHERE id IN %s ORDER BY COALESCE(NULLIF(sort_name, ''), name) COLLATE NOCASE", getInBinding(len(ids)))
	var rows []copyrightRow
	if err := dbWrapper.Select(ctx, &rows, query, args...); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	ret := make([]*models.Copyright, 0, len(rows))
	for _, row := range rows {
		item := row.model()
		if err := s.loadAliases(ctx, item); err != nil {
			return nil, err
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func (s *CopyrightStore) FindByName(ctx context.Context, name string, nocase bool) (*models.Copyright, error) {
	operator := "= ?"
	if nocase {
		operator += " COLLATE NOCASE"
	}
	var row copyrightRow
	query := "SELECT id, name, sort_name, description, favorite, created_at, updated_at FROM copyrights WHERE name " + operator + " LIMIT 1"
	if err := dbWrapper.Get(ctx, &row, query, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ret := row.model()
	if err := s.loadAliases(ctx, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *CopyrightStore) FindByAlias(ctx context.Context, alias string, nocase bool) (*models.Copyright, error) {
	collation := ""
	if nocase {
		collation = " COLLATE NOCASE"
	}
	var row copyrightRow
	query := `SELECT c.id, c.name, c.sort_name, c.description, c.favorite, c.created_at, c.updated_at
FROM copyrights c INNER JOIN copyright_aliases a ON a.copyright_id = c.id
WHERE a.alias = ?` + collation + ` LIMIT 1`
	if err := dbWrapper.Get(ctx, &row, query, alias); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ret := row.model()
	if err := s.loadAliases(ctx, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *CopyrightStore) Query(ctx context.Context, filter *models.FindFilterType) ([]*models.Copyright, int, error) {
	ff := models.FindFilterType{}
	if filter != nil {
		ff = *filter
	}
	where := ""
	args := []interface{}{}
	if ff.Q != nil && strings.TrimSpace(*ff.Q) != "" {
		q := "%" + strings.TrimSpace(*ff.Q) + "%"
		where = ` WHERE c.name LIKE ? COLLATE NOCASE OR c.sort_name LIKE ? COLLATE NOCASE OR EXISTS (
SELECT 1 FROM copyright_aliases a WHERE a.copyright_id = c.id AND a.alias LIKE ? COLLATE NOCASE)`
		args = append(args, q, q, q)
	}

	var count int
	if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM copyrights c"+where, args...); err != nil {
		return nil, 0, err
	}

	sortColumn := "COALESCE(NULLIF(c.sort_name, ''), c.name)"
	switch ff.GetSort("name") {
	case "created_at":
		sortColumn = "c.created_at"
	case "updated_at":
		sortColumn = "c.updated_at"
	case "name", "sort_name":
		// default
	}
	direction := ff.GetDirection()
	if direction != "DESC" {
		direction = "ASC"
	}
	query := `SELECT c.id, c.name, c.sort_name, c.description, c.favorite, c.created_at, c.updated_at FROM copyrights c` + where +
		" ORDER BY " + sortColumn + " COLLATE NOCASE " + direction
	queryArgs := append([]interface{}{}, args...)
	if !ff.IsGetAll() {
		pageSize := ff.GetPageSize()
		query += " LIMIT ? OFFSET ?"
		queryArgs = append(queryArgs, pageSize, (ff.GetPage()-1)*pageSize)
	}

	var rows []copyrightRow
	if err := dbWrapper.Select(ctx, &rows, query, queryArgs...); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}
	ret := make([]*models.Copyright, 0, len(rows))
	for _, row := range rows {
		item := row.model()
		if err := s.loadAliases(ctx, item); err != nil {
			return nil, 0, err
		}
		ret = append(ret, item)
	}
	return ret, count, nil
}

func normalizeCopyrightAliases(name string, aliases []string) []string {
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(name)): true}
	ret := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		key := strings.ToLower(alias)
		if alias == "" || seen[key] {
			continue
		}
		seen[key] = true
		ret = append(ret, alias)
	}
	return ret
}

func replaceCopyrightAliases(ctx context.Context, id int, aliases []string) error {
	if _, err := dbWrapper.Exec(ctx, "DELETE FROM copyright_aliases WHERE copyright_id = ?", id); err != nil {
		return err
	}
	for _, alias := range aliases {
		if _, err := dbWrapper.Exec(ctx, "INSERT INTO copyright_aliases (copyright_id, alias) VALUES (?, ?)", id, alias); err != nil {
			return err
		}
	}
	return nil
}

func replaceCopyrightRelations(ctx context.Context, id int, ids []int, parents bool) error {
	column := "child_id"
	other := "parent_id"
	if !parents {
		column, other = "parent_id", "child_id"
	}
	if _, err := dbWrapper.Exec(ctx, "DELETE FROM copyright_relations WHERE "+column+" = ?", id); err != nil {
		return err
	}
	for _, related := range ids {
		if related == id {
			continue
		}
		query := "INSERT OR IGNORE INTO copyright_relations (" + column + ", " + other + ") VALUES (?, ?)"
		if _, err := dbWrapper.Exec(ctx, query, id, related); err != nil {
			return err
		}
	}
	return nil
}

func (s *CopyrightStore) Create(ctx context.Context, input models.CopyrightCreateInput) (*models.Copyright, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("copyright name cannot be empty")
	}
	sortName := ""
	if input.SortName != nil {
		sortName = strings.TrimSpace(*input.SortName)
	}
	description := ""
	if input.Description != nil {
		description = *input.Description
	}
	favorite := false
	if input.Favorite != nil {
		favorite = *input.Favorite
	}
	now := time.Now()
	result, err := dbWrapper.Exec(ctx,
		"INSERT INTO copyrights (name, sort_name, description, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, sortName, description, favorite, now, now)
	if err != nil {
		return nil, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	id := int(id64)
	if err := replaceCopyrightAliases(ctx, id, normalizeCopyrightAliases(name, input.Aliases)); err != nil {
		return nil, err
	}
	parentIDs, err := stringIDsToInts(input.ParentIDs)
	if err != nil {
		return nil, err
	}
	childIDs, err := stringIDsToInts(input.ChildIDs)
	if err != nil {
		return nil, err
	}
	if err := replaceCopyrightRelations(ctx, id, parentIDs, true); err != nil {
		return nil, err
	}
	if err := replaceCopyrightRelations(ctx, id, childIDs, false); err != nil {
		return nil, err
	}
	return s.Find(ctx, id)
}

func stringIDsToInts(ids []string) ([]int, error) {
	ret := make([]int, 0, len(ids))
	for _, raw := range ids {
		var id int
		if _, err := fmt.Sscanf(raw, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid id %q: %w", raw, err)
		}
		ret = append(ret, id)
	}
	return ret, nil
}

func (s *CopyrightStore) Update(ctx context.Context, input models.CopyrightUpdateInput) (*models.Copyright, error) {
	ids, err := stringIDsToInts([]string{input.ID})
	if err != nil {
		return nil, err
	}
	id := ids[0]
	current, err := s.Find(ctx, id)
	if err != nil || current == nil {
		return current, err
	}

	name := current.Name
	sets := []string{}
	args := []interface{}{}
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("copyright name cannot be empty")
		}
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if input.SortName != nil {
		sets = append(sets, "sort_name = ?")
		args = append(args, strings.TrimSpace(*input.SortName))
	}
	if input.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *input.Description)
	}
	if input.Favorite != nil {
		sets = append(sets, "favorite = ?")
		args = append(args, *input.Favorite)
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at = ?")
		args = append(args, time.Now(), id)
		if _, err := dbWrapper.Exec(ctx, "UPDATE copyrights SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
			return nil, err
		}
	}
	if input.Aliases != nil {
		if err := replaceCopyrightAliases(ctx, id, normalizeCopyrightAliases(name, input.Aliases)); err != nil {
			return nil, err
		}
	}
	if input.ParentIDs != nil {
		parentIDs, err := stringIDsToInts(input.ParentIDs)
		if err != nil {
			return nil, err
		}
		if err := replaceCopyrightRelations(ctx, id, parentIDs, true); err != nil {
			return nil, err
		}
	}
	if input.ChildIDs != nil {
		childIDs, err := stringIDsToInts(input.ChildIDs)
		if err != nil {
			return nil, err
		}
		if err := replaceCopyrightRelations(ctx, id, childIDs, false); err != nil {
			return nil, err
		}
	}
	return s.Find(ctx, id)
}

func (s *CopyrightStore) Destroy(ctx context.Context, id int) error {
	_, err := dbWrapper.Exec(ctx, "DELETE FROM copyrights WHERE id = ?", id)
	return err
}

func (s *CopyrightStore) findRelated(ctx context.Context, id int, parents bool) ([]*models.Copyright, error) {
	joinColumn := "parent_id"
	whereColumn := "child_id"
	if !parents {
		joinColumn, whereColumn = "child_id", "parent_id"
	}
	query := `SELECT c.id, c.name, c.sort_name, c.description, c.favorite, c.created_at, c.updated_at
FROM copyrights c INNER JOIN copyright_relations r ON c.id = r.` + joinColumn + `
WHERE r.` + whereColumn + ` = ? ORDER BY COALESCE(NULLIF(c.sort_name, ''), c.name) COLLATE NOCASE`
	var rows []copyrightRow
	if err := dbWrapper.Select(ctx, &rows, query, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	ret := make([]*models.Copyright, 0, len(rows))
	for _, row := range rows {
		item := row.model()
		if err := s.loadAliases(ctx, item); err != nil {
			return nil, err
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func (s *CopyrightStore) FindParents(ctx context.Context, id int) ([]*models.Copyright, error) {
	return s.findRelated(ctx, id, true)
}

func (s *CopyrightStore) FindChildren(ctx context.Context, id int) ([]*models.Copyright, error) {
	return s.findRelated(ctx, id, false)
}

func copyrightRelationCount(ctx context.Context, table, idColumn string, id int) (int, error) {
	var count int
	err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM "+table+" WHERE "+idColumn+" = ?", id)
	return count, err
}

func (s *CopyrightStore) ImageCount(ctx context.Context, id int) (int, error) {
	return copyrightRelationCount(ctx, imagesCopyrightsTable, "copyright_id", id)
}

func (s *CopyrightStore) SceneCount(ctx context.Context, id int) (int, error) {
	return copyrightRelationCount(ctx, scenesCopyrightsTable, "copyright_id", id)
}

func (s *CopyrightStore) findForMedia(ctx context.Context, table, mediaColumn string, mediaID int) ([]*models.Copyright, error) {
	query := `SELECT c.id, c.name, c.sort_name, c.description, c.favorite, c.created_at, c.updated_at
FROM copyrights c INNER JOIN ` + table + ` j ON j.copyright_id = c.id
WHERE j.` + mediaColumn + ` = ? ORDER BY COALESCE(NULLIF(c.sort_name, ''), c.name) COLLATE NOCASE`
	var rows []copyrightRow
	if err := dbWrapper.Select(ctx, &rows, query, mediaID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	ret := make([]*models.Copyright, 0, len(rows))
	for _, row := range rows {
		item := row.model()
		if err := s.loadAliases(ctx, item); err != nil {
			return nil, err
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func (s *CopyrightStore) FindByImageID(ctx context.Context, imageID int) ([]*models.Copyright, error) {
	return s.findForMedia(ctx, imagesCopyrightsTable, "image_id", imageID)
}

func (s *CopyrightStore) FindBySceneID(ctx context.Context, sceneID int) ([]*models.Copyright, error) {
	return s.findForMedia(ctx, scenesCopyrightsTable, "scene_id", sceneID)
}

func setMediaCopyrights(ctx context.Context, table, mediaColumn string, mediaID int, copyrightIDs []int, replace bool) error {
	if replace {
		if _, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE "+mediaColumn+" = ?", mediaID); err != nil {
			return err
		}
	}
	for _, copyrightID := range copyrightIDs {
		if _, err := dbWrapper.Exec(ctx, "INSERT OR IGNORE INTO "+table+" ("+mediaColumn+", copyright_id) VALUES (?, ?)", mediaID, copyrightID); err != nil {
			return err
		}
	}
	return nil
}

func (s *CopyrightStore) SetImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, imagesCopyrightsTable, "image_id", imageID, copyrightIDs, true)
}

func (s *CopyrightStore) AddImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, imagesCopyrightsTable, "image_id", imageID, copyrightIDs, false)
}

func (s *CopyrightStore) SetSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, scenesCopyrightsTable, "scene_id", sceneID, copyrightIDs, true)
}

func (s *CopyrightStore) AddSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error {
	return setMediaCopyrights(ctx, scenesCopyrightsTable, "scene_id", sceneID, copyrightIDs, false)
}
