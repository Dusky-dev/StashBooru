package sqlite

import (
	"context"
	"fmt"
)

// StorageEstimateVisualStackIDs reads a bounded set of pre-existing,
// image-only Visual Stacks. It does not create, edit or imply storage groups.
func StorageEstimateVisualStackIDs(ctx context.Context, limit int) ([]int, error) {
	if limit < 1 || limit > 64 {
		return nil, fmt.Errorf("invalid storage estimate sample limit")
	}
	ids := []int{}
	const query = `SELECT s.id
FROM visual_stacks AS s
JOIN visual_stack_members AS m ON m.stack_id = s.id
GROUP BY s.id
HAVING COUNT(*) BETWEEN 2 AND 16
AND SUM(CASE WHEN m.scene_id IS NOT NULL THEN 1 ELSE 0 END) = 0
ORDER BY s.id
LIMIT ?`
	if err := dbWrapper.Select(ctx, &ids, query, limit); err != nil {
		return nil, fmt.Errorf("listing image-only Visual Stacks for estimate: %w", err)
	}
	return ids, nil
}
