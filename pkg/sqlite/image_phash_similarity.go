package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

// Only the primary file's current pHash is evidence about the displayed image.
// Do not fall back to source_phash or a secondary file after conversion/upscaling.
const imagePrimaryPHashSQL = `SELECT fp.fingerprint
FROM images_files AS image_file
JOIN files_fingerprints AS fp ON fp.file_id = image_file.file_id AND fp.type = 'phash'
WHERE image_file.image_id = ? AND image_file."primary" = 1
ORDER BY image_file.file_id, fp.fingerprint LIMIT 1`

func imagePHashMatchesSQL(candidateSQL string) string {
	return fmt.Sprintf(`SELECT candidates.id, MIN(phash_distance(fp.fingerprint, ?)) AS distance
FROM (%s) AS candidates
JOIN images_files AS image_file ON image_file.image_id = candidates.id AND image_file."primary" = 1
JOIN files_fingerprints AS fp ON fp.file_id = image_file.file_id AND fp.type = 'phash'
WHERE candidates.id != ? AND fp.fingerprint IS NOT NULL
AND phash_distance(fp.fingerprint, ?) <= ?
GROUP BY candidates.id`, candidateSQL)
}

func findImagePHashSimilarityIDs(ctx context.Context, query queryBuilder, findFilter *models.FindFilterType, referenceID, distance int) ([]int, int, error) {
	return findImagePHashSimilarityIDsWithCollapse(ctx, query, findFilter, referenceID, distance, false)
}

func findImagePHashSimilarityIDsCollapsed(ctx context.Context, query queryBuilder, findFilter *models.FindFilterType, referenceID, distance int) ([]int, int, error) {
	return findImagePHashSimilarityIDsWithCollapse(ctx, query, findFilter, referenceID, distance, true)
}

func findImagePHashSimilarityIDsWithCollapse(ctx context.Context, query queryBuilder, findFilter *models.FindFilterType, referenceID, distance int, collapse bool) ([]int, int, error) {
	// Load the reference separately: it need not pass the result filters.
	var reference sql.NullInt64
	if err := dbWrapper.Get(ctx, &reference, imagePrimaryPHashSQL, referenceID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, fmt.Errorf("reading image %d pHash: %w", referenceID, err)
	}
	if !reference.Valid {
		return nil, 0, fmt.Errorf("image %d has no current primary-file pHash; generate image perceptual hashes in Tasks, or choose Related content", referenceID)
	}

	matchesSQL := imagePHashMatchesSQL(query.toSQL(false))
	if collapse {
		// Match variants first, then choose their actual stack representative.
		// The reference itself is excluded from matches; a matching variant
		// must still yield its representative even when that is the reference.
		// Mixed stacks represented by a Video have no Image card in Images.
		matchesSQL = `WITH matched AS (` + matchesSQL + `),
  grouped AS (
    SELECT COALESCE('stack:' || sm.stack_id, 'image:' || matched.id) AS group_key,
      CASE WHEN sm.stack_id IS NULL THEN matched.id ELSE rep.image_id END AS display_id,
      matched.distance
    FROM matched
    LEFT JOIN visual_stack_members sm ON sm.image_id = matched.id
    LEFT JOIN visual_stack_members rep ON rep.stack_id = sm.stack_id AND rep.representative = 1
  )
  SELECT MAX(display_id) AS id, MIN(distance) AS distance
  FROM grouped GROUP BY group_key HAVING MAX(display_id) IS NOT NULL`
	}
	args := []interface{}{reference.Int64}
	args = append(args, query.allArgs()...)
	args = append(args, referenceID, reference.Int64, distance)
	var total int
	if err := dbWrapper.Get(ctx, &total, "SELECT COUNT(*) FROM ("+matchesSQL+")", args...); err != nil {
		return nil, 0, fmt.Errorf("counting image pHash matches: %w", err)
	}

	// Filter and paginate in SQLite rather than loading every candidate into Go.
	ids := []int{}
	idSQL := "SELECT id FROM (" + matchesSQL + ") ORDER BY distance ASC, id ASC" + getPagination(findFilter)
	if err := dbWrapper.Select(ctx, &ids, idSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("querying image pHash matches: %w", err)
	}
	return ids, total, nil
}

// collapseSortedImageIDs retains the first ranked hit for each visual stack
// but displays its actual Image representative. A Video representative has
// no Images-only card, so matching Images in that stack are omitted here.
func collapseSortedImageIDs(ctx context.Context, ranked []int, findFilter *models.FindFilterType) ([]int, int, error) {
	var members []struct {
		ImageID   int           `db:"image_id"`
		DisplayID sql.NullInt64 `db:"display_id"`
	}
	err := dbWrapper.Select(ctx, &members, `SELECT sm.image_id AS image_id, rep.image_id AS display_id
  FROM visual_stack_members sm
  LEFT JOIN visual_stack_members rep ON rep.stack_id = sm.stack_id AND rep.representative = 1
  WHERE sm.image_id IS NOT NULL`)
	if err != nil {
		return nil, 0, fmt.Errorf("looking up ranked Image stack representatives: %w", err)
	}
	representatives := make(map[int]int, len(members))
	for _, member := range members {
		if member.DisplayID.Valid {
			representatives[member.ImageID] = int(member.DisplayID.Int64)
		} else {
			representatives[member.ImageID] = 0
		}
	}
	collapsed := make([]int, 0, len(ranked))
	seen := make(map[int]bool)
	for _, id := range ranked {
		if representative, grouped := representatives[id]; grouped {
			id = representative
		}
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		collapsed = append(collapsed, id)
	}
	return paginatePHashSimilarityIDs(collapsed, findFilter), len(collapsed), nil
}
