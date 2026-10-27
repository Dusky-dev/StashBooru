package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
)

// Metadata merges are called inside the repository transaction. SQL transfers
// deliberately happen before deletion, so any constraint/conflict rolls back
// the whole merge instead of leaving media without its original association.
func mergeExec(ctx context.Context, query string, args ...interface{}) error {
	_, err := dbWrapper.Exec(ctx, query, args...)
	return err
}

func mergeConflictCount(ctx context.Context, query string, args ...interface{}) (bool, error) {
	var count int
	if err := dbWrapper.Get(ctx, &count, query, args...); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *StudioStore) Merge(ctx context.Context, sourceID, destinationID int) error {
	if sourceID <= 0 || destinationID <= 0 || sourceID == destinationID {
		return fmt.Errorf("choose two distinct Artists")
	}
	source, err := s.Find(ctx, sourceID)
	if err != nil {
		return err
	}
	dest, err := s.Find(ctx, destinationID)
	if err != nil {
		return err
	}
	if source == nil || dest == nil {
		return fmt.Errorf("source or destination Artist was not found")
	}
	if source.Details != "" && dest.Details != "" && source.Details != dest.Details {
		return fmt.Errorf("Artist descriptions conflict; edit one profile before merging")
	}
	if source.Rating != nil && dest.Rating != nil && *source.Rating != *dest.Rating {
		return fmt.Errorf("Artist ratings conflict; edit one profile before merging")
	}
	if source.ParentID != nil && *source.ParentID != destinationID &&
		dest.ParentID != nil && *dest.ParentID != sourceID &&
		*source.ParentID != *dest.ParentID {
		return fmt.Errorf("Artist parent relationships conflict; resolve them before merging")
	}
	conflict, err := mergeConflictCount(ctx, `SELECT COUNT(*) FROM studio_custom_fields src
JOIN studio_custom_fields dst ON dst.studio_id=? AND dst.field=src.field
WHERE src.studio_id=? AND src.value!=dst.value`, destinationID, sourceID)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("Artist custom fields conflict; resolve them before merging")
	}
	sourceImage, err := s.GetImage(ctx, sourceID)
	if err != nil {
		return err
	}
	destImage, err := s.GetImage(ctx, destinationID)
	if err != nil {
		return err
	}
	if len(sourceImage) > 0 && len(destImage) > 0 && !bytes.Equal(sourceImage, destImage) {
		return fmt.Errorf("Artist portraits conflict; resolve them before merging")
	}
	if len(sourceImage) > 0 && len(destImage) == 0 {
		if err := s.UpdateImage(ctx, destinationID, sourceImage); err != nil {
			return err
		}
	}
	// Preserve the destination's primary metadata. Fill missing fields and
	// combine non-conflicting boolean flags rather than overwriting details.
	if err := mergeExec(ctx, `UPDATE studios SET
details=CASE WHEN details='' THEN ? ELSE details END,
rating=COALESCE(rating, ?),
favorite=(favorite OR ?), organized=(organized OR ?),
ignore_auto_tag=(ignore_auto_tag OR ?),
created_at=MIN(created_at, ?), updated_at=?
WHERE id=?`, source.Details, source.Rating, source.Favorite, source.Organized,
		source.IgnoreAutoTag, source.CreatedAt, time.Now(), destinationID); err != nil {
		return err
	}
	if dest.ParentID != nil && *dest.ParentID == sourceID {
		// Destination was a child of the entity being removed.
		var newParent interface{}
		if source.ParentID != nil && *source.ParentID != destinationID {
			newParent = *source.ParentID
		}
		if err := mergeExec(ctx, "UPDATE studios SET parent_id=? WHERE id=?", newParent, destinationID); err != nil {
			return err
		}
	} else if dest.ParentID == nil && source.ParentID != nil && *source.ParentID != destinationID {
		if err := mergeExec(ctx, "UPDATE studios SET parent_id=? WHERE id=?", *source.ParentID, destinationID); err != nil {
			return err
		}
	}
	// Prevent losing associations from legacy single-Artist fields and the
	// newer ordered many-to-many Artist links.
	for _, query := range []string{
		"UPDATE images SET studio_id=? WHERE studio_id=?",
		"UPDATE scenes SET studio_id=? WHERE studio_id=?",
		"UPDATE galleries SET studio_id=? WHERE studio_id=?",
		"UPDATE groups SET studio_id=? WHERE studio_id=?",
		"UPDATE studios SET parent_id=? WHERE parent_id=? AND id!=?",
	} {
		if strings.Contains(query, "AND id!=?") {
			if err := mergeExec(ctx, query, destinationID, sourceID, destinationID); err != nil {
				return err
			}
		} else if err := mergeExec(ctx, query, destinationID, sourceID); err != nil {
			return err
		}
	}
	for _, table := range []string{"images_artists", "scenes_artists"} {
		mediaColumn := "image_id"
		if table == "scenes_artists" {
			mediaColumn = "scene_id"
		}
		query := fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, studio_id, position)
SELECT %s, ?, position FROM %s WHERE studio_id=?`, table, mediaColumn, mediaColumn, table)
		if err := mergeExec(ctx, query, destinationID, sourceID); err != nil {
			return err
		}
	}
	for _, query := range []string{
		`INSERT OR IGNORE INTO studios_tags (studio_id,tag_id)
SELECT ?,tag_id FROM studios_tags WHERE studio_id=?`,
		`INSERT OR IGNORE INTO studio_custom_fields (studio_id,field,value)
SELECT ?,field,value FROM studio_custom_fields WHERE studio_id=?`,
		`INSERT INTO studio_stash_ids (studio_id,endpoint,stash_id)
SELECT ?,endpoint,stash_id FROM studio_stash_ids s
WHERE studio_id=? AND NOT EXISTS (SELECT 1 FROM studio_stash_ids d
WHERE d.studio_id=? AND d.endpoint=s.endpoint AND d.stash_id=s.stash_id)`,
	} {
		if strings.Contains(query, "studio_stash_ids") {
			err = mergeExec(ctx, query, destinationID, sourceID, destinationID)
		} else {
			err = mergeExec(ctx, query, destinationID, sourceID)
		}
		if err != nil {
			return err
		}
	}
	var maxURL int
	if err := dbWrapper.Get(ctx, &maxURL, "SELECT COALESCE(MAX(position),-1) FROM studio_urls WHERE studio_id=?", destinationID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT INTO studio_urls (studio_id,position,url)
SELECT ?, position+?, url FROM studio_urls s WHERE studio_id=?
AND NOT EXISTS (SELECT 1 FROM studio_urls d WHERE d.studio_id=? AND d.url=s.url)`,
		destinationID, maxURL+1, sourceID, destinationID); err != nil {
		return err
	}
	// An alias can only belong to one Studio. Redundant aliases already on
	// the destination are harmless, everything else must transfer or fail.
	if err := mergeExec(ctx, `DELETE FROM studio_aliases WHERE studio_id=?
AND alias IN (SELECT alias FROM studio_aliases WHERE studio_id=?)`, sourceID, destinationID); err != nil {
		return err
	}
	if err := mergeExec(ctx, "UPDATE studio_aliases SET studio_id=? WHERE studio_id=?", destinationID, sourceID); err != nil {
		return err
	}
	if source.Name != dest.Name {
		if err := mergeExec(ctx, `INSERT OR IGNORE INTO studio_aliases(studio_id,alias)
VALUES (?,?)`, destinationID, source.Name); err != nil {
			return err
		}
	}
	if err := mergeExec(ctx, "UPDATE provider_identities SET studio_id=? WHERE studio_id=?", destinationID, sourceID); err != nil {
		return err
	}
	return s.Destroy(ctx, sourceID)
}

func (s *CopyrightStore) Merge(ctx context.Context, sourceID, destinationID int) error {
	if sourceID <= 0 || destinationID <= 0 || sourceID == destinationID {
		return fmt.Errorf("choose two distinct Copyrights")
	}
	src, err := s.Find(ctx, sourceID)
	if err != nil {
		return err
	}
	dst, err := s.Find(ctx, destinationID)
	if err != nil {
		return err
	}
	if src == nil || dst == nil {
		return fmt.Errorf("source or destination Copyright was not found")
	}
	if src.Description != "" && dst.Description != "" && src.Description != dst.Description {
		return fmt.Errorf("Copyright descriptions conflict; edit one before merging")
	}
	if src.SortName != "" && dst.SortName != "" && src.SortName != dst.SortName {
		return fmt.Errorf("Copyright sort names conflict; edit one before merging")
	}
	srcImage, err := s.GetImage(ctx, sourceID)
	if err != nil {
		return err
	}
	dstImage, err := s.GetImage(ctx, destinationID)
	if err != nil {
		return err
	}
	if len(srcImage) > 0 && len(dstImage) > 0 && !bytes.Equal(srcImage, dstImage) {
		return fmt.Errorf("Copyright portraits conflict; edit one before merging")
	}
	if len(srcImage) > 0 && len(dstImage) == 0 {
		if err := s.UpdateImage(ctx, destinationID, srcImage); err != nil {
			return err
		}
	}
	conflict, err := mergeConflictCount(ctx, `SELECT COUNT(*)
FROM copyright_structural_roles a JOIN copyright_structural_roles b ON b.copyright_id=?
WHERE a.copyright_id=? AND a.role!='' AND b.role!='' AND a.role!=b.role`, destinationID, sourceID)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("Copyright structural roles conflict")
	}
	if err := mergeExec(ctx, `UPDATE copyrights SET
description=CASE WHEN description='' THEN ? ELSE description END,
sort_name=CASE WHEN sort_name='' THEN ? ELSE sort_name END,
favorite=(favorite OR ?), created_at=MIN(created_at,?), updated_at=?
WHERE id=?`, src.Description, src.SortName, src.Favorite, src.CreatedAt, time.Now(), destinationID); err != nil {
		return err
	}
	for _, row := range []struct{ table, column string }{
		{"images_copyrights", "image_id"},
		{"scenes_copyrights", "scene_id"},
		{"performers_copyrights", "performer_id"},
		{"copyrights_tags", "tag_id"},
	} {
		query := fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s,copyright_id)
SELECT %s,? FROM %s WHERE copyright_id=?`, row.table, row.column, row.column, row.table)
		if row.table == "copyrights_tags" {
			query = `INSERT OR IGNORE INTO copyrights_tags (tag_id,copyright_id)
SELECT tag_id,? FROM copyrights_tags WHERE copyright_id=?`
		}
		if err := mergeExec(ctx, query, destinationID, sourceID); err != nil {
			return err
		}
	}
	for _, table := range []string{"image_primary_copyrights", "scene_primary_copyrights"} {
		if err := mergeExec(ctx, "UPDATE "+table+" SET copyright_id=? WHERE copyright_id=?", destinationID, sourceID); err != nil {
			return err
		}
	}
	// Keep each parent and child; omit the merged-away edge to itself.
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_relations(parent_id,child_id)
SELECT ?,child_id FROM copyright_relations WHERE parent_id=? AND child_id NOT IN (?,?)`,
		destinationID, sourceID, destinationID, sourceID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_relations(parent_id,child_id)
SELECT parent_id,? FROM copyright_relations WHERE child_id=? AND parent_id NOT IN (?,?)`,
		destinationID, sourceID, destinationID, sourceID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_relation_order(parent_id,child_id,position)
SELECT ?,child_id,position FROM copyright_relation_order WHERE parent_id=? AND child_id!=?`,
		destinationID, sourceID, destinationID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_relation_order(parent_id,child_id,position)
SELECT parent_id,?,position FROM copyright_relation_order WHERE child_id=? AND parent_id!=?`,
		destinationID, sourceID, destinationID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_structural_roles(copyright_id,role)
SELECT ?,role FROM copyright_structural_roles WHERE copyright_id=?`, destinationID, sourceID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `UPDATE copyright_structural_roles SET role=(SELECT role FROM copyright_structural_roles WHERE copyright_id=?)
WHERE copyright_id=? AND role='' AND EXISTS (SELECT 1 FROM copyright_structural_roles WHERE copyright_id=? AND role!='')`,
		sourceID,destinationID,sourceID); err != nil {
		return err
	}
	if err := mergeExec(ctx, `INSERT OR IGNORE INTO copyright_aliases(copyright_id,alias)
SELECT ?,alias FROM copyright_aliases WHERE copyright_id=?`, destinationID, sourceID); err != nil {
		return err
	}
	if src.Name != dst.Name {
		if err := mergeExec(ctx, "INSERT OR IGNORE INTO copyright_aliases(copyright_id,alias) VALUES (?,?)", destinationID, src.Name); err != nil {
			return err
		}
	}
	if err := mergeExec(ctx, "UPDATE provider_identities SET copyright_id=? WHERE copyright_id=?", destinationID, sourceID); err != nil {
		return err
	}
	if err := s.Destroy(ctx, sourceID); err != nil {
		return err
	}
	// The final Copyright graph must be acyclic. Recheck after the source's
	// old edges have been removed, before the enclosing transaction commits.
	var cyclic int
	if err := dbWrapper.Get(ctx, &cyclic, `WITH RECURSIVE reachable(id,path) AS (
SELECT parent_id,printf('/%d/',parent_id) FROM copyright_relations
UNION ALL
SELECT edge.child_id,reachable.path || edge.child_id || '/' FROM copyright_relations edge
JOIN reachable ON edge.parent_id=reachable.id
WHERE instr(reachable.path,printf('/%d/',edge.child_id))=0
)
SELECT EXISTS(SELECT 1 FROM copyright_relations edge JOIN reachable ON edge.parent_id=reachable.id
WHERE instr(reachable.path,printf('/%d/',edge.child_id))>0)`); err != nil {
		return err
	}
	if cyclic != 0 {
		return fmt.Errorf("Copyright merge would create a hierarchy cycle")
	}
	return nil
}
