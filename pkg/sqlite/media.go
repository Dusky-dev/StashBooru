package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

type MediaStore struct{ stores *storeRepository }

func (s *MediaStore) Query(ctx context.Context, filter *models.MediaFilterType, find *models.FindFilterType) (*models.MediaQueryResult, error) {
	if filter == nil {
		filter = &models.MediaFilterType{}
	}
	if find == nil {
		find = &models.FindFilterType{}
	}
	if find.IsGetAll() || find.GetPageSize() < 1 || find.GetPageSize() > 500 {
		return nil, fmt.Errorf("media page size must be between 1 and 500")
	}
	page, size := find.GetPage(), find.GetPageSize()
	if page > int(^uint(0)>>1)/size {
		return nil, fmt.Errorf("media page is too large")
	}
	includeImage, includeVideo := len(filter.MediaTypes) == 0, len(filter.MediaTypes) == 0
	for _, k := range filter.MediaTypes {
		switch k {
		case models.MediaKindImage:
			includeImage = true
		case models.MediaKindVideo:
			includeVideo = true
		default:
			return nil, fmt.Errorf("invalid media kind: %s", k)
		}
	}
	if filter.Duration != nil {
		includeImage = false
	}

	imageFilter := &models.ImageFilterType{Title: filter.Title, Details: filter.Details, Path: filter.Path,
		Rating100: filter.Rating100, Date: filter.Date, CreatedAt: filter.CreatedAt, UpdatedAt: filter.UpdatedAt,
		Organized: filter.Organized, PerformerFavorite: filter.PerformerFavorite, Tags: filter.Tags,
		Performers: filter.Performers, Studios: filter.Studios, Copyrights: filter.Copyrights}
	sceneFilter := &models.SceneFilterType{Title: filter.Title, Details: filter.Details, Path: filter.Path,
		Rating100: filter.Rating100, Date: filter.Date, CreatedAt: filter.CreatedAt, UpdatedAt: filter.UpdatedAt,
		Organized: filter.Organized, PerformerFavorite: filter.PerformerFavorite, Tags: filter.Tags,
		Performers: filter.Performers, Studios: filter.Studios, Copyrights: filter.Copyrights, Duration: filter.Duration}
	// Native filters execute inside the same read transaction. They select IDs
	// without pagination; one UNION applies the global ordering and page boundary.
	search := &models.FindFilterType{Q: find.Q}
	iq, err := s.stores.Image.makeQuery(ctx, imageFilter, search)
	if err != nil {
		return nil, err
	}
	sq, err := s.stores.Scene.makeQuery(ctx, sceneFilter, search)
	if err != nil {
		return nil, err
	}
	if !includeImage {
		iq.addWhere("0")
	}
	if !includeVideo {
		sq.addWhere("0")
	}
	args := append(iq.allArgs(), sq.allArgs()...)
	sort := find.GetSort("created_at")
	allowed := map[string]bool{"created_at": true, "updated_at": true, "date": true, "title": true, "rating": true, "filesize": true, "path": true}
	if !allowed[sort] {
		return nil, fmt.Errorf("unsupported media sort: %s", sort)
	}
	if find.Direction != nil && !find.Direction.IsValid() {
		return nil, fmt.Errorf("invalid sort direction")
	}
	projection := func(table, kind, extra string) string {
		return fmt.Sprintf(`SELECT '%s' AS kind, m.id%s FROM %s m WHERE m.id IN (SELECT id FROM matched_%s)`, kind, extra, table, table)
	}
	sortExpression := func(links, fk string) string {
		switch sort {
		case "filesize":
			return fmt.Sprintf(`(SELECT f.size FROM %s l JOIN files f ON f.id=l.file_id WHERE l.%s=m.id AND l."primary"=1)`, links, fk)
		case "path":
			return fmt.Sprintf(`(SELECT d.path || '/' || f.basename FROM %s l JOIN files f ON f.id=l.file_id
				LEFT JOIN folders d ON d.id=f.parent_folder_id WHERE l.%s=m.id AND l."primary"=1)`, links, fk)
		case "title", "date":
			return fmt.Sprintf("NULLIF(m.%s, '')", sort)
		default:
			return "m." + sort
		}
	}
	matched := "WITH matched_images AS (" + iq.toSQL(false) + "), matched_scenes AS (" + sq.toSQL(false) + "), media AS ("
	// Count only identities. SQLite evaluates unused UNION projections too, so
	// file/path lookups must not run for every item just to count the library.
	countWith := matched + projection(imageTable, string(models.MediaKindImage), "") + " UNION ALL " +
		projection(sceneTable, string(models.MediaKindVideo), "") + ") "
	pageWith := matched + projection(imageTable, string(models.MediaKindImage), ", "+sortExpression(imagesFilesTable, imageIDColumn)+" AS sort_value") + " UNION ALL " +
		projection(sceneTable, string(models.MediaKindVideo), ", "+sortExpression(scenesFilesTable, sceneIDColumn)+" AS sort_value") + ") "
	collation := ""
	if sort == "title" || sort == "path" {
		collation = " COLLATE NATURAL_CI"
	}
	order := fmt.Sprintf(" ORDER BY sort_value IS NULL ASC, sort_value%s %s, kind ASC, id ASC", collation, find.GetDirection())
	result := &models.MediaQueryResult{Items: []models.MediaReference{}}
	countSQL := countWith + `SELECT COUNT(*) AS matched_count, COUNT(*) AS count,
  COALESCE(SUM(kind='IMAGE'),0) AS image_count, COALESCE(SUM(kind='VIDEO'),0) AS video_count FROM media`
	pageSQL := pageWith + "SELECT kind,id,1 AS stack_match_count FROM media" + order + " LIMIT ? OFFSET ?"
	if filter.CollapseStacks != nil && *filter.CollapseStacks {
		grouped := func(with string) string {
			return strings.TrimSpace(with) + `, grouped AS (SELECT media.*,
    CASE WHEN sm.stack_id IS NULL THEN media.kind || ':' || media.id ELSE 'stack:' || sm.stack_id END AS group_key,
    CASE WHEN rep.scene_id IS NOT NULL THEN 'VIDEO' WHEN rep.image_id IS NOT NULL THEN 'IMAGE' ELSE media.kind END AS display_kind,
    COALESCE(rep.image_id,rep.scene_id,media.id) AS display_id
    FROM media LEFT JOIN visual_stack_members sm ON
      (media.kind='IMAGE' AND sm.image_id=media.id) OR (media.kind='VIDEO' AND sm.scene_id=media.id)
    LEFT JOIN visual_stack_members rep ON rep.stack_id=sm.stack_id AND rep.representative=1) `
		}
		countSQL = grouped(countWith) + `, cards AS (SELECT group_key,MAX(display_kind) AS kind FROM grouped GROUP BY group_key)
   SELECT (SELECT COUNT(*) FROM media) AS matched_count,COUNT(*) AS count,
   COALESCE(SUM(kind='IMAGE'),0) AS image_count,COALESCE(SUM(kind='VIDEO'),0) AS video_count FROM cards`
		rankOrder := fmt.Sprintf("sort_value IS NULL ASC,sort_value%s %s,kind ASC,id ASC", collation, find.GetDirection())
		pageSQL = grouped(pageWith) + `, ranked AS (SELECT display_kind,display_id,sort_value,kind,id,
   COUNT(*) OVER(PARTITION BY group_key) AS stack_match_count,
   ROW_NUMBER() OVER(PARTITION BY group_key ORDER BY ` + rankOrder + `) AS group_rank FROM grouped)
   SELECT display_kind AS kind,display_id AS id,stack_match_count FROM ranked WHERE group_rank=1 ORDER BY ` + strings.ReplaceAll(strings.ReplaceAll(rankOrder, "kind ASC", "ranked.kind ASC"), "id ASC", "ranked.id ASC") + " LIMIT ? OFFSET ?"
	}
	if err := dbWrapper.Get(ctx, result, countSQL, args...); err != nil {
		return nil, err
	}
	args = append(args, size, (page-1)*size)
	if err := dbWrapper.Select(ctx, &result.Items, pageSQL, args...); err != nil {
		return nil, err
	}
	return result, nil
}
