package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

const maxVisualStackMembers = 200

type VisualStackStore struct{ files models.FileReader }

func stackMediaColumn(ref models.MediaReference) (string, string, error) {
	if !ref.Kind.IsValid() || ref.ID < 1 {
		return "", "", fmt.Errorf("invalid media reference")
	}
	if ref.Kind == models.MediaKindImage {
		return "image_id", "images", nil
	}
	return "scene_id", "scenes", nil
}

func (s *VisualStackStore) summary(ctx context.Context, id int) (*models.VisualStack, error) {
	var row struct {
		ID          int           `db:"id"`
		Title       string        `db:"title"`
		Version     int           `db:"version"`
		MemberCount int           `db:"member_count"`
		Image       sql.NullInt64 `db:"image_id"`
		Scene       sql.NullInt64 `db:"scene_id"`
	}
	err := dbWrapper.Get(ctx, &row, `SELECT s.id,s.title,s.version,(SELECT COUNT(*) FROM visual_stack_members WHERE stack_id=s.id) AS member_count,
 r.image_id,r.scene_id FROM visual_stacks s LEFT JOIN visual_stack_members r ON r.stack_id=s.id AND r.representative=1 WHERE s.id=?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ref := models.MediaReference{Kind: models.MediaKindImage, ID: int(row.Image.Int64)}
	if row.Scene.Valid {
		ref = models.MediaReference{Kind: models.MediaKindVideo, ID: int(row.Scene.Int64)}
	}
	return &models.VisualStack{ID: row.ID, Title: row.Title, Version: row.Version, MemberCount: row.MemberCount, Representative: ref.Key()}, nil
}

func (s *VisualStackStore) Find(ctx context.Context, id int) (*models.VisualStack, error) {
	ret, err := s.summary(ctx, id)
	if err != nil || ret == nil {
		return ret, err
	}
	var rows []struct {
		Image          sql.NullInt64 `db:"image_id"`
		Scene          sql.NullInt64 `db:"scene_id"`
		Position       int           `db:"position"`
		Label          string        `db:"label"`
		Representative bool          `db:"representative"`
	}
	if err = dbWrapper.Select(ctx, &rows, `SELECT image_id,scene_id,position,label,representative FROM visual_stack_members WHERE stack_id=? ORDER BY position`, id); err != nil {
		return nil, err
	}
	ret.Members = []*models.VisualStackMember{}
	for _, row := range rows {
		ref := models.MediaReference{Kind: models.MediaKindImage, ID: int(row.Image.Int64)}
		if row.Scene.Valid {
			ref = models.MediaReference{Kind: models.MediaKindVideo, ID: int(row.Scene.Int64)}
		}
		ret.Members = append(ret.Members, &models.VisualStackMember{ID: ref.Key(), Media: ref, Position: row.Position, Label: row.Label, Representative: row.Representative})
	}
	return ret, nil
}

func (s *VisualStackStore) FindByMedia(ctx context.Context, ref models.MediaReference) (*models.VisualStack, error) {
	column, _, err := stackMediaColumn(ref)
	if err != nil {
		return nil, err
	}
	var id int
	err = dbWrapper.Get(ctx, &id, "SELECT stack_id FROM visual_stack_members WHERE "+column+"=?", ref.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.summary(ctx, id)
}

func (s *VisualStackStore) checked(ctx context.Context, id, version int) (*models.VisualStack, error) {
	stack, err := s.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, fmt.Errorf("stack does not exist")
	}
	if version != stack.Version {
		return nil, fmt.Errorf("stack changed; reload before editing")
	}
	return stack, nil
}

func (s *VisualStackStore) validate(ctx context.Context, title string, members []*models.VisualStackMemberInput, rep models.MediaReference, allowedID int) error {
	if len(strings.TrimSpace(title)) > 200 {
		return fmt.Errorf("stack title must be at most 200 bytes")
	}
	if len(members) < 1 || len(members) > maxVisualStackMembers {
		return fmt.Errorf("stack must have between 1 and 200 members")
	}
	if _, _, err := stackMediaColumn(rep); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, member := range members {
		if member == nil {
			return fmt.Errorf("invalid stack member")
		}
		_, table, err := stackMediaColumn(member.Media)
		if err != nil {
			return err
		}
		key := member.Media.Key()
		if seen[key] {
			return fmt.Errorf("duplicate stack member %s", key)
		}
		seen[key] = true
		if len(member.Label) > 80 {
			return fmt.Errorf("relationship label must be at most 80 bytes")
		}
		var exists bool
		if err := dbWrapper.Get(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE id=?)", member.Media.ID); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("member %s does not exist", key)
		}
		stack, err := s.FindByMedia(ctx, member.Media)
		if err != nil {
			return err
		}
		if stack != nil && stack.ID != allowedID {
			return fmt.Errorf("member %s already belongs to stack %d; merge stacks instead", key, stack.ID)
		}
	}
	if !seen[rep.Key()] {
		return fmt.Errorf("representative must be a stack member")
	}
	return nil
}

// Upsert retained memberships rather than deleting/recreating them: the native
// deletion trigger must not remove a stack while an edit is being assembled.
func (s *VisualStackStore) write(ctx context.Context, id, version int, title string, members []*models.VisualStackMemberInput, rep models.MediaReference) (*models.VisualStack, error) {
	var offset int
	if err := dbWrapper.Get(ctx, &offset, "SELECT COALESCE(MAX(position),0)+1 FROM visual_stack_members WHERE stack_id=?", id); err != nil {
		return nil, err
	}
	if _, err := dbWrapper.Exec(ctx, "UPDATE visual_stack_members SET position=position+?,representative=0 WHERE stack_id=?", offset, id); err != nil {
		return nil, err
	}
	keep := map[string]bool{}
	for i, m := range members {
		column, _, err := stackMediaColumn(m.Media)
		if err != nil {
			return nil, err
		}
		keep[m.Media.Key()] = true
		q := fmt.Sprintf(`INSERT INTO visual_stack_members(stack_id,%s,position,label,representative) VALUES(?,?,?,?,?)
  ON CONFLICT(%s) DO UPDATE SET position=excluded.position,label=excluded.label,representative=excluded.representative`, column, column)
		if _, err := dbWrapper.Exec(ctx, q, id, m.Media.ID, i, m.Label, m.Media.Key() == rep.Key()); err != nil {
			return nil, err
		}
	}
	existing, err := s.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, m := range existing.Members {
		if !keep[m.ID] {
			column, _, _ := stackMediaColumn(m.Media)
			if _, err := dbWrapper.Exec(ctx, "DELETE FROM visual_stack_members WHERE stack_id=? AND "+column+"=?", id, m.Media.ID); err != nil {
				return nil, err
			}
		}
	}
	if _, err := dbWrapper.Exec(ctx, "UPDATE visual_stacks SET title=?,version=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", strings.TrimSpace(title), version+1, id); err != nil {
		return nil, err
	}
	return s.Find(ctx, id)
}

func (s *VisualStackStore) Create(ctx context.Context, input models.VisualStackCreateInput) (*models.VisualStack, error) {
	if len(input.Members) < 2 {
		return nil, fmt.Errorf("select at least two members")
	}
	if err := s.validate(ctx, input.Title, input.Members, input.Representative, 0); err != nil {
		return nil, err
	}
	result, err := dbWrapper.Exec(ctx, "INSERT INTO visual_stacks(title) VALUES(?)", strings.TrimSpace(input.Title))
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.write(ctx, int(id), 0, input.Title, input.Members, input.Representative)
}

func (s *VisualStackStore) Update(ctx context.Context, input models.VisualStackUpdateInput) (*models.VisualStack, error) {
	stack, err := s.checked(ctx, input.ID, input.Version)
	if err != nil {
		return nil, err
	}
	if _, _, err := stackMediaColumn(input.Representative); err != nil {
		return nil, err
	}
	found := false
	for _, m := range input.Members {
		if m != nil && m.Media.Key() == input.Representative.Key() {
			found = true
		}
	}
	// Removing the current representative deterministically selects a survivor.
	if !found && len(input.Members) > 0 && input.Members[0] != nil && input.Representative.Key() == stack.Representative {
		input.Representative = input.Members[0].Media
	}
	if err := s.validate(ctx, input.Title, input.Members, input.Representative, input.ID); err != nil {
		return nil, err
	}
	return s.write(ctx, input.ID, input.Version, input.Title, input.Members, input.Representative)
}

func stackInputs(stack *models.VisualStack) []*models.VisualStackMemberInput {
	ret := make([]*models.VisualStackMemberInput, 0, len(stack.Members))
	for _, m := range stack.Members {
		ret = append(ret, &models.VisualStackMemberInput{Media: m.Media, Label: m.Label})
	}
	return ret
}

func (s *VisualStackStore) Split(ctx context.Context, input models.VisualStackSplitInput) (*models.VisualStack, error) {
	stack, err := s.checked(ctx, input.ID, input.Version)
	if err != nil {
		return nil, err
	}
	if len(input.Members) < 2 || len(input.Members) >= len(stack.Members) {
		return nil, fmt.Errorf("split at least two members and leave at least one")
	}
	selected := map[string]bool{}
	for _, m := range input.Members {
		if m == nil {
			return nil, fmt.Errorf("invalid split member")
		}
		if _, _, err := stackMediaColumn(*m); err != nil {
			return nil, err
		}
		if selected[m.Key()] {
			return nil, fmt.Errorf("duplicate split member")
		}
		selected[m.Key()] = true
	}
	var moved, kept []*models.VisualStackMemberInput
	for _, m := range stackInputs(stack) {
		if selected[m.Media.Key()] {
			moved = append(moved, m)
		} else {
			kept = append(kept, m)
		}
	}
	if len(moved) != len(selected) {
		return nil, fmt.Errorf("split members must belong to this stack")
	}
	rep := kept[0].Media
	for _, m := range kept {
		if m.Media.Key() == stack.Representative {
			rep = m.Media
		}
	}
	if _, err := s.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Title: stack.Title, Members: kept, Representative: rep}); err != nil {
		return nil, err
	}
	// The API owns the transaction, so destination validation also rolls back
	// the source edit when creation fails.
	return s.Create(ctx, models.VisualStackCreateInput{Title: input.Title, Members: moved, Representative: input.Representative})
}

func (s *VisualStackStore) Merge(ctx context.Context, input models.VisualStackMergeInput) (*models.VisualStack, error) {
	if len(input.Stacks) < 2 || len(input.Stacks) > 100 {
		return nil, fmt.Errorf("merge between 2 and 100 stacks")
	}
	seen := map[int]bool{}
	var stacks []*models.VisualStack
	var members []*models.VisualStackMemberInput
	for _, ref := range input.Stacks {
		if ref == nil || seen[ref.ID] {
			return nil, fmt.Errorf("duplicate or invalid stack")
		}
		seen[ref.ID] = true
		stack, err := s.checked(ctx, ref.ID, ref.Version)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, stack)
		members = append(members, stackInputs(stack)...)
	}
	if len(members) > maxVisualStackMembers {
		return nil, fmt.Errorf("merged stack exceeds 200 members")
	}
	target := stacks[0]
	if _, err := dbWrapper.Exec(ctx, "UPDATE visual_stack_members SET representative=0 WHERE stack_id=?", target.ID); err != nil {
		return nil, err
	}
	var offset int
	if err := dbWrapper.Get(ctx, &offset, "SELECT COALESCE(MAX(position),0)+1 FROM visual_stack_members WHERE stack_id=?", target.ID); err != nil {
		return nil, err
	}
	for _, stack := range stacks[1:] {
		for _, member := range stack.Members {
			column, _, _ := stackMediaColumn(member.Media)
			if _, err := dbWrapper.Exec(ctx, "UPDATE visual_stack_members SET stack_id=?,representative=0,position=? WHERE stack_id=? AND "+column+"=?", target.ID, offset, stack.ID, member.Media.ID); err != nil {
				return nil, err
			}
			offset++
		}
		if _, err := dbWrapper.Exec(ctx, "DELETE FROM visual_stacks WHERE id=?", stack.ID); err != nil {
			return nil, err
		}
	}
	if err := s.validate(ctx, input.Title, members, input.Representative, target.ID); err != nil {
		return nil, err
	}
	return s.write(ctx, target.ID, target.Version, input.Title, members, input.Representative)
}

func (s *VisualStackStore) Destroy(ctx context.Context, input models.VisualStackVersionInput) error {
	if _, err := s.checked(ctx, input.ID, input.Version); err != nil {
		return err
	}
	_, err := dbWrapper.Exec(ctx, "DELETE FROM visual_stacks WHERE id=?", input.ID)
	return err
}

func (s *VisualStackStore) Propose(ctx context.Context, refs []*models.MediaReference) ([]*models.VisualStackProposal, error) {
	if len(refs) < 2 || len(refs) > 100 {
		return nil, fmt.Errorf("review between 2 and 100 selected members")
	}
	type evidence struct {
		ref         models.MediaReference
		md5, source string
		phash       *int64
		fresh       bool
		stack       int
	}
	items := make([]evidence, 0, len(refs))
	seen := map[string]bool{}
	for _, pointer := range refs {
		if pointer == nil {
			return nil, fmt.Errorf("invalid selected member")
		}
		ref := *pointer
		column, _, err := stackMediaColumn(ref)
		if err != nil {
			return nil, err
		}
		if seen[ref.Key()] {
			return nil, fmt.Errorf("duplicate selected member")
		}
		seen[ref.Key()] = true
		table := "images_files"
		if ref.Kind == models.MediaKindVideo {
			table = "scenes_files"
		}
		var fid models.FileID
		err = dbWrapper.Get(ctx, &fid, "SELECT file_id FROM "+table+" WHERE "+column+"=? AND \"primary\"=1", ref.ID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files, err := s.files.Find(ctx, fid)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}
		file := files[0].Base()
		item := evidence{ref: ref, md5: file.Fingerprints.GetString("md5"), source: file.Fingerprints.GetString("source_md5")}
		if phash := file.Fingerprints.For("phash"); phash != nil {
			value := phash.Int64()
			item.phash = &value
		}
		if stack, err := s.FindByMedia(ctx, ref); err != nil {
			return nil, err
		} else if stack != nil {
			item.stack = stack.ID
		}
		if ref.Kind == models.MediaKindImage && item.phash != nil {
			if stat, err := os.Stat(file.Path); err == nil {
				key := fmt.Sprintf("%s:%d:%d", file.Path, stat.Size(), stat.ModTime().UnixNano())
				item.fresh, err = VisualEmbeddings.HasCurrentImage(ctx, ref.ID, key)
				if err != nil {
					return nil, err
				}
			}
		}
		items = append(items, item)
	}
	ret := []*models.VisualStackProposal{}
	for i, a := range items {
		for _, b := range items[i+1:] {
			if a.stack != 0 && a.stack == b.stack {
				continue
			}
			proposal := &models.VisualStackProposal{Members: []*models.MediaReference{&a.ref, &b.ref}}
			switch {
			case a.md5 != "" && a.md5 == b.md5:
				proposal.Evidence = "Identical active-file MD5"
			case (a.source != "" && a.source == b.md5) || (b.source != "" && b.source == a.md5):
				proposal.Evidence = "Recorded source MD5 matches the other active file"
			case a.fresh && b.fresh && bits.OnesCount64(uint64(*a.phash)^uint64(*b.phash)) <= 4:
				var distance float64
				err := dbWrapper.Get(ctx, &distance, `SELECT vec_distance_cosine(a.embedding,b.embedding) FROM image_embedding_vectors a,image_embedding_vectors b WHERE a.rowid=? AND b.rowid=?`, a.ref.ID, b.ref.ID)
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				if err != nil {
					return nil, err
				}
				if distance > 0.02 {
					continue
				}
				score := 1 - distance
				proposal.Score = &score
				proposal.Evidence = "pHash distance ≤ 4 and current EVA02 cosine ≥ 0.98"
			default:
				continue
			}
			ret = append(ret, proposal)
		}
	}
	return ret, nil
}
