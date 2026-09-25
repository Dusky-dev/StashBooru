package sqlite

import (
	"context"
	"fmt"
	"strings"
)

// Validate the final graph before changing metadata or relations. Nil leaves
// that side unchanged; an empty slice removes it. Considering both sides at
// once allows valid moves that would look cyclic halfway through an update.
func validateCopyrightHierarchy(ctx context.Context, id int, parents, children []int) error {
	if parents == nil && children == nil {
		return nil
	}
	seen := map[int]bool{}
	var targetArgs []interface{}
	for _, ids := range [][]int{parents, children} {
		for _, related := range ids {
			if related == id {
				return fmt.Errorf("a copyright cannot be its own parent or child")
			}
			if !seen[related] {
				seen[related] = true
				targetArgs = append(targetArgs, related)
			}
		}
	}
	if len(targetArgs) > 0 {
		var count int
		if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM copyrights WHERE id IN "+getInBinding(len(targetArgs)), targetArgs...); err != nil {
			return err
		}
		if count != len(targetArgs) {
			return fmt.Errorf("one or more parent or child copyrights do not exist")
		}
	}

	edges := "SELECT parent_id, child_id FROM copyright_relations WHERE 1 = 1"
	var args []interface{}
	if parents != nil {
		edges += " AND child_id != ?"
		args = append(args, id)
	}
	if children != nil {
		edges += " AND parent_id != ?"
		args = append(args, id)
	}
	var values []string
	for _, parent := range parents {
		values = append(values, "(?, ?)")
		args = append(args, parent, id)
	}
	for _, child := range children {
		values = append(values, "(?, ?)")
		args = append(args, id, child)
	}
	if len(values) > 0 {
		edges += " UNION SELECT * FROM (VALUES " + strings.Join(values, ", ") + ")"
	}
	query := `WITH RECURSIVE final_edges(parent_id, child_id) AS (` + edges + `),
reachable(id) AS (
 SELECT child_id FROM final_edges WHERE parent_id = ?
 UNION
 SELECT e.child_id FROM final_edges e JOIN reachable r ON e.parent_id = r.id
)
SELECT COUNT(*) FROM reachable WHERE id = ?`
	args = append(args, id, id)
	var cycles int
	if err := dbWrapper.Get(ctx, &cycles, query, args...); err != nil {
		return fmt.Errorf("validating copyright hierarchy: %w", err)
	}
	if cycles != 0 {
		return fmt.Errorf("copyright hierarchy would contain a cycle; a series cannot be its own ancestor")
	}
	return nil
}
