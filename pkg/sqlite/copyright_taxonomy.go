package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

func copyrightDescendantIDs(ctx context.Context, id, depth int) ([]int, error) {
	if depth == 0 {
		return []int{id}, nil
	}

	depthClause := ""
	args := []interface{}{id}
	if depth > 0 {
		depthClause = "WHERE d.depth < ?"
		args = append(args, depth)
	}

	query := `WITH RECURSIVE descendants(id, depth) AS (
SELECT ?, 0
UNION
SELECT r.child_id, d.depth + 1
FROM copyright_relations r
INNER JOIN descendants d ON r.parent_id = d.id
` + depthClause + `
)
SELECT DISTINCT id FROM descendants ORDER BY id`

	var ids []int
	if err := dbWrapper.Select(ctx, &ids, query, args...); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return ids, nil
}

func copyrightRelationCountDepth(ctx context.Context, table, mediaColumn string, id, depth int) (int, error) {
	ids, err := copyrightDescendantIDs(ctx, id, depth)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	args := make([]interface{}, len(ids))
	for i, copyrightID := range ids {
		args[i] = copyrightID
	}
	var count int
	query := fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s WHERE copyright_id IN %s", mediaColumn, table, getInBinding(len(ids)))
	if err := dbWrapper.Get(ctx, &count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *CopyrightStore) ImageCountDepth(ctx context.Context, id, depth int) (int, error) {
	return copyrightRelationCountDepth(ctx, imagesCopyrightsTable, "image_id", id, depth)
}

func (s *CopyrightStore) SceneCountDepth(ctx context.Context, id, depth int) (int, error) {
	return copyrightRelationCountDepth(ctx, scenesCopyrightsTable, "scene_id", id, depth)
}

func (s *CopyrightStore) PerformerCountDepth(ctx context.Context, id, depth int) (int, error) {
	return copyrightRelationCountDepth(ctx, performersCopyrightsTable, "performer_id", id, depth)
}

func findCopyrightMediaIDsDepth(ctx context.Context, table, mediaColumn string, copyrightID, depth int) ([]int, error) {
	ids, err := copyrightDescendantIDs(ctx, copyrightID, depth)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []int{}, nil
	}

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE copyright_id IN %s ORDER BY %s", mediaColumn, table, getInBinding(len(ids)), mediaColumn)
	var mediaIDs []int
	if err := dbWrapper.Select(ctx, &mediaIDs, query, args...); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return mediaIDs, nil
}

func (s *CopyrightStore) FindImageIDsDepth(ctx context.Context, copyrightID, depth int) ([]int, error) {
	return findCopyrightMediaIDsDepth(ctx, imagesCopyrightsTable, "image_id", copyrightID, depth)
}

func (s *CopyrightStore) FindSceneIDsDepth(ctx context.Context, copyrightID, depth int) ([]int, error) {
	return findCopyrightMediaIDsDepth(ctx, scenesCopyrightsTable, "scene_id", copyrightID, depth)
}

func (s *CopyrightStore) FindPerformerIDsDepth(ctx context.Context, copyrightID, depth int) ([]int, error) {
	return findCopyrightMediaIDsDepth(ctx, performersCopyrightsTable, "performer_id", copyrightID, depth)
}

// copyrightSubtreeCountSortExpression returns a correlated scalar expression for
// Copyright directory count sorting. UNION de-duplicates nodes reached through
// multiple parent paths, while COUNT(DISTINCT ...) de-duplicates media linked to
// more than one node in the same branch.
func copyrightSubtreeCountSortExpression(joinTable, mediaColumn string) string {
	return `(WITH RECURSIVE descendants(id) AS (
SELECT c.id
UNION
SELECT r.child_id FROM copyright_relations r
INNER JOIN descendants d ON r.parent_id = d.id
)
SELECT COUNT(DISTINCT m.` + mediaColumn + `)
FROM ` + joinTable + ` m
INNER JOIN descendants d ON d.id = m.copyright_id)`
}

func (s *CopyrightStore) SetChildOrder(ctx context.Context, parentID int, childIDs []int) error {
	var direct []int
	if err := dbWrapper.Select(ctx, &direct, "SELECT child_id FROM copyright_relations WHERE parent_id = ? ORDER BY child_id", parentID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if len(direct) != len(childIDs) {
		return fmt.Errorf("child order must contain every direct child exactly once")
	}
	wanted := make(map[int]struct{}, len(direct))
	for _, id := range direct {
		wanted[id] = struct{}{}
	}
	seen := make(map[int]struct{}, len(childIDs))
	for _, id := range childIDs {
		if _, ok := wanted[id]; !ok {
			return fmt.Errorf("copyright %d is not a direct child of %d", id, parentID)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("copyright %d appears more than once in child order", id)
		}
		seen[id] = struct{}{}
	}

	if _, err := dbWrapper.Exec(ctx, "DELETE FROM copyright_relation_order WHERE parent_id = ?", parentID); err != nil {
		return err
	}
	for position, childID := range childIDs {
		if _, err := dbWrapper.Exec(ctx, "INSERT INTO copyright_relation_order (parent_id, child_id, position) VALUES (?, ?, ?)", parentID, childID, position); err != nil {
			return err
		}
	}
	return nil
}

func (s *CopyrightStore) FindOrderedChildren(ctx context.Context, id int) ([]*models.Copyright, error) {
	query := `SELECT c.id, c.name, c.sort_name, c.description, c.favorite, c.created_at, c.updated_at
FROM copyrights c
INNER JOIN copyright_relations r ON c.id = r.child_id
LEFT JOIN copyright_relation_order o ON o.parent_id = r.parent_id AND o.child_id = r.child_id
WHERE r.parent_id = ?
ORDER BY CASE WHEN o.position IS NULL THEN 1 ELSE 0 END, o.position ASC,
COALESCE(NULLIF(c.sort_name, ''), c.name) COLLATE NOCASE, c.id ASC`
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

func structuralRole(ctx context.Context, table, idColumn string, id int) (string, error) {
	var role string
	err := dbWrapper.Get(ctx, &role, "SELECT role FROM "+table+" WHERE "+idColumn+" = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

func setStructuralRole(ctx context.Context, table, idColumn string, id int, role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		_, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE "+idColumn+" = ?", id)
		return err
	}
	_, err := dbWrapper.Exec(ctx, "INSERT INTO "+table+" ("+idColumn+", role) VALUES (?, ?) ON CONFLICT("+idColumn+") DO UPDATE SET role = excluded.role", id, role)
	return err
}

func (s *CopyrightStore) StructuralRole(ctx context.Context, id int) (string, error) {
	return structuralRole(ctx, "copyright_structural_roles", "copyright_id", id)
}

func (s *CopyrightStore) SetStructuralRole(ctx context.Context, id int, role string) error {
	return setStructuralRole(ctx, "copyright_structural_roles", "copyright_id", id, role)
}

func TagStructuralRole(ctx context.Context, id int) (string, error) {
	return structuralRole(ctx, "tag_structural_roles", "tag_id", id)
}

func SetTagStructuralRole(ctx context.Context, id int, role string) error {
	return setStructuralRole(ctx, "tag_structural_roles", "tag_id", id, role)
}

func (s *CopyrightStore) Breadcrumb(ctx context.Context, id int) ([]*models.Copyright, error) {
	current, err := s.Find(ctx, id)
	if err != nil || current == nil {
		return nil, err
	}
	path := []*models.Copyright{current}
	visited := map[int]struct{}{id: {}}
	for {
		parents, err := s.FindParents(ctx, current.ID)
		if err != nil {
			return nil, err
		}
		if len(parents) == 0 {
			break
		}
		current = parents[0]
		if _, ok := visited[current.ID]; ok {
			return nil, fmt.Errorf("cycle encountered while resolving Copyright breadcrumb")
		}
		visited[current.ID] = struct{}{}
		path = append(path, current)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path, nil
}

func (s *CopyrightStore) branchKey(ctx context.Context, id int) (string, error) {
	path, err := s.Breadcrumb(ctx, id)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(path))
	for _, item := range path {
		name := item.SortName
		if name == "" {
			name = item.Name
		}
		parts = append(parts, strings.ToLower(name))
	}
	return strings.Join(parts, "\x00"), nil
}

func primaryCopyrightID(ctx context.Context, table, mediaColumn string, mediaID int) (*int, error) {
	var id int
	err := dbWrapper.Get(ctx, &id, "SELECT copyright_id FROM "+table+" WHERE "+mediaColumn+" = ?", mediaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func setPrimaryCopyrightID(ctx context.Context, table, mediaColumn string, mediaID int, copyrightID *int) error {
	if _, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE "+mediaColumn+" = ?", mediaID); err != nil {
		return err
	}
	if copyrightID == nil {
		return nil
	}
	_, err := dbWrapper.Exec(ctx, "INSERT INTO "+table+" ("+mediaColumn+", copyright_id) VALUES (?, ?)", mediaID, *copyrightID)
	return err
}

func (s *CopyrightStore) PrimaryImageCopyrightID(ctx context.Context, imageID int) (*int, error) {
	return primaryCopyrightID(ctx, "image_primary_copyrights", "image_id", imageID)
}

func (s *CopyrightStore) PrimarySceneCopyrightID(ctx context.Context, sceneID int) (*int, error) {
	return primaryCopyrightID(ctx, "scene_primary_copyrights", "scene_id", sceneID)
}

func (s *CopyrightStore) SetPrimaryImageCopyright(ctx context.Context, imageID int, copyrightID *int) error {
	return setPrimaryCopyrightID(ctx, "image_primary_copyrights", "image_id", imageID, copyrightID)
}

func (s *CopyrightStore) SetPrimarySceneCopyright(ctx context.Context, sceneID int, copyrightID *int) error {
	return setPrimaryCopyrightID(ctx, "scene_primary_copyrights", "scene_id", sceneID, copyrightID)
}

func (s *CopyrightStore) findByMediaIDOrdered(ctx context.Context, table, mediaColumn string, mediaID int, primaryTable string) ([]*models.Copyright, error) {
	items, err := s.findForMedia(ctx, table, mediaColumn, mediaID)
	if err != nil || len(items) < 2 {
		return items, err
	}
	primaryID, err := primaryCopyrightID(ctx, primaryTable, mediaColumn, mediaID)
	if err != nil {
		return nil, err
	}
	keys := make(map[int]string, len(items))
	for _, item := range items {
		key, err := s.branchKey(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		keys[item.ID] = key
	}
	sort.SliceStable(items, func(i, j int) bool {
		if primaryID != nil {
			if items[i].ID == *primaryID && items[j].ID != *primaryID {
				return true
			}
			if items[j].ID == *primaryID && items[i].ID != *primaryID {
				return false
			}
		}
		if keys[items[i].ID] == keys[items[j].ID] {
			return items[i].ID < items[j].ID
		}
		return keys[items[i].ID] < keys[items[j].ID]
	})
	return items, nil
}

func (s *CopyrightStore) FindByImageIDOrdered(ctx context.Context, imageID int) ([]*models.Copyright, error) {
	return s.findByMediaIDOrdered(ctx, imagesCopyrightsTable, "image_id", imageID, "image_primary_copyrights")
}

func (s *CopyrightStore) FindBySceneIDOrdered(ctx context.Context, sceneID int) ([]*models.Copyright, error) {
	return s.findByMediaIDOrdered(ctx, scenesCopyrightsTable, "scene_id", sceneID, "scene_primary_copyrights")
}
