//go:build integration

package api

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func p08Ref(kind models.MediaKind, id int) models.MediaReference {
	return models.MediaReference{Kind: kind, ID: id}
}
func p08Members(refs ...models.MediaReference) []*models.VisualStackMemberInput {
	ret := make([]*models.VisualStackMemberInput, 0, len(refs))
	for _, ref := range refs {
		ret = append(ret, &models.VisualStackMemberInput{Media: ref})
	}
	return ret
}
func p08Write(t *testing.T, r models.Repository, fn func(context.Context) (*models.VisualStack, error)) *models.VisualStack {
	t.Helper()
	var ret *models.VisualStack
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error { var err error; ret, err = fn(ctx); return err }))
	return ret
}
func p08Read(t *testing.T, r models.Repository, id int) *models.VisualStack {
	t.Helper()
	var ret *models.VisualStack
	require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error { var err error; ret, err = r.VisualStack.Find(ctx, id); return err }))
	return ret
}
func p08Inputs(stack *models.VisualStack) []*models.VisualStackMemberInput {
	ret := make([]*models.VisualStackMemberInput, 0, len(stack.Members))
	for _, m := range stack.Members {
		ret = append(ret, &models.VisualStackMemberInput{Media: m.Media, Label: m.Label})
	}
	return ret
}
func p08Keys(stack *models.VisualStack) []string {
	ret := []string{}
	for _, m := range stack.Members {
		ret = append(ret, m.ID)
	}
	return ret
}

func TestP08LifecycleKeepsNativeMediaFilesAndGalleryOrder(t *testing.T) {
	r := p07Fixture(t)
	image1, video1, image2, video2 := p08Ref(models.MediaKindImage, 1), p08Ref(models.MediaKindVideo, 1), p08Ref(models.MediaKindImage, 2), p08Ref(models.MediaKindVideo, 2)
	gallery := models.NewGallery()
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		require.NoError(t, r.Gallery.Create(ctx, &models.CreateGalleryInput{Gallery: &gallery}))
		require.NoError(t, r.Gallery.AddImages(ctx, gallery.ID, 2, 1, 3))
		for _, id := range []models.FileID{1, 2} {
			files, err := r.File.Find(ctx, id)
			require.NoError(t, err)
			data := []byte(fmt.Sprintf("unchanged fixture bytes %d", id))
			require.NoError(t, os.WriteFile(files[0].Base().Path, data, 0o600))
			sum := fmt.Sprintf("%x", md5.Sum(data))
			require.NoError(t, r.File.ModifyFingerprints(ctx, id, []models.Fingerprint{{Type: "md5", Fingerprint: sum}}))
		}
		return nil
	}))
	snapshot := func() []byte {
		var result []byte
		require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
			images, err := r.Image.FindMany(ctx, []int{1, 2, 3})
			require.NoError(t, err)
			scenes, err := r.Scene.FindMany(ctx, []int{1, 2})
			require.NoError(t, err)
			files, err := r.File.Find(ctx, 1, 2)
			require.NoError(t, err)
			ids, err := r.Gallery.GetImageIDs(ctx, gallery.ID)
			require.NoError(t, err)
			var contents [][]byte
			for _, file := range files {
				data, err := os.ReadFile(file.Base().Path)
				require.NoError(t, err)
				contents = append(contents, data)
			}
			result, err = json.Marshal([]any{images, scenes, files, contents, ids})
			return err
		}))
		return result
	}
	before := snapshot()
	stack := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Title: "Variants", Members: p08Members(image1, video1, image2, video2), Representative: video1})
	})
	require.Equal(t, []string{"image:1", "scene:1", "image:2", "scene:2"}, p08Keys(stack))
	require.Equal(t, "scene:1", stack.Representative)
	p07GraphQL(t, r, `query { findImage(id:"1") { id title visual_stack { representative member_count members { id media { kind id } image { id } scene { id } } } } findScene(id:"1") { id title visual_stack { id } } }`, nil)
	members := p08Inputs(stack)
	members[0].Label = "Original"
	members[1].Label = "Converted copy"
	members[2], members[3] = members[3], members[2]
	stack = p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Title: stack.Title, Members: members, Representative: image2})
	})
	moved := []*models.MediaReference{&video1, &video2}
	other := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Split(ctx, models.VisualStackSplitInput{ID: stack.ID, Version: stack.Version, Members: moved, Representative: video1})
	})
	stack = p08Read(t, r, stack.ID)
	require.Equal(t, []string{"scene:1", "scene:2"}, p08Keys(other))
	require.Equal(t, "Converted copy", other.Members[0].Label)
	stack = p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Merge(ctx, models.VisualStackMergeInput{Stacks: []*models.VisualStackVersionInput{{ID: stack.ID, Version: stack.Version}, {ID: other.ID, Version: other.Version}}, Title: stack.Title, Representative: image2})
	})
	require.Nil(t, p08Read(t, r, other.ID))
	require.Equal(t, 4, stack.MemberCount)
	members = p08Inputs(stack)
	var kept []*models.VisualStackMemberInput
	for _, m := range members {
		if m.Media.Key() != image2.Key() {
			kept = append(kept, m)
		}
	}
	stack = p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Members: kept, Representative: image2})
	})
	require.Equal(t, "image:1", stack.Representative)
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		return r.VisualStack.Destroy(ctx, models.VisualStackVersionInput{ID: stack.ID, Version: stack.Version})
	}))
	require.Equal(t, before, snapshot(), "all grouping operations preserve bytes, fingerprints, native metadata and gallery order")
	stack = p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(video1, image1), Representative: video1})
	})
	prior := stack.Version
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error { return r.Scene.Destroy(ctx, 1) }))
	stack = p08Read(t, r, stack.ID)
	require.Equal(t, "image:1", stack.Representative)
	require.Greater(t, stack.Version, prior)
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error { return r.Image.Destroy(ctx, 1) }))
	require.Nil(t, p08Read(t, r, stack.ID))
}

func TestP08RejectsAmbiguousMembershipStaleWritesAndRollsBack(t *testing.T) {
	r := p07Fixture(t)
	a, b, c := p08Ref(models.MediaKindImage, 1), p08Ref(models.MediaKindVideo, 1), p08Ref(models.MediaKindImage, 2)
	stack := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(a, b, c), Representative: a})
	})
	for _, input := range []models.VisualStackCreateInput{
		{Members: p08Members(a, b), Representative: a},
		{Members: p08Members(c, c), Representative: c},
		{Members: p08Members(p08Ref(models.MediaKindImage, 999), p08Ref(models.MediaKindVideo, 2)), Representative: p08Ref(models.MediaKindImage, 999)},
	} {
		require.Error(t, r.WithTxn(context.Background(), func(ctx context.Context) error { _, err := r.VisualStack.Create(ctx, input); return err }))
	}
	require.ErrorContains(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		return r.VisualStack.Destroy(ctx, models.VisualStackVersionInput{ID: stack.ID, Version: 0})
	}), "stack changed")
	// Source update executes before destination validation. A failed split must
	// restore the complete source, including its version and representative.
	require.Error(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := r.VisualStack.Split(ctx, models.VisualStackSplitInput{ID: stack.ID, Version: stack.Version, Members: []*models.MediaReference{&b, &c}, Representative: a})
		return err
	}))
	require.Equal(t, stack, p08Read(t, r, stack.ID))
	require.Error(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := r.VisualStack.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Members: p08Inputs(stack), Representative: p08Ref("INVALID", 1)})
		return err
	}))
}

func TestP08CollapseBeforePaginationAnyMemberFiltersAndStablePosition(t *testing.T) {
	r := p07Fixture(t)
	a, b, c, d := p08Ref(models.MediaKindImage, 1), p08Ref(models.MediaKindVideo, 1), p08Ref(models.MediaKindImage, 2), p08Ref(models.MediaKindVideo, 2)
	stack := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(a, b, c, d), Representative: a})
	})
	collapse := true
	sort := "created_at"
	perPage := 2
	direction := models.SortDirectionEnumAsc
	var keys []string
	for page := 1; page <= 4; page++ {
		result := p07Query(t, r, &models.MediaFilterType{CollapseStacks: &collapse}, &models.FindFilterType{Sort: &sort, Direction: &direction, Page: &page, PerPage: &perPage})
		require.Equal(t, 7, result.Count)
		require.Equal(t, 10, result.MatchedCount)
		keys = append(keys, p07Keys(result)...)
	}
	require.Len(t, keys, 7)
	require.Equal(t, "image:1", keys[0])
	require.Len(t, keys, len(mapKeys(keys)))
	stack = p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Members: p08Inputs(stack), Representative: d})
	})
	result := p07Query(t, r, &models.MediaFilterType{CollapseStacks: &collapse}, &models.FindFilterType{Sort: &sort, Direction: &direction})
	after := p07Keys(result)
	keys[0] = "scene:2"
	require.Equal(t, keys, after, "representative changes retain the group's native matching sort position")
	result = p07Query(t, r, &models.MediaFilterType{CollapseStacks: &collapse, Title: &models.StringCriterionInput{Value: "Item 2", Modifier: models.CriterionModifierEquals}}, nil)
	require.Equal(t, 1, result.Count)
	require.Equal(t, 1, result.MatchedCount)
	require.Equal(t, []string{"scene:2"}, p07Keys(result))
	require.Equal(t, 1, result.Items[0].StackMatchCount)
	result = p07Query(t, r, &models.MediaFilterType{CollapseStacks: &collapse, MediaTypes: []models.MediaKind{models.MediaKindImage}}, nil)
	require.Equal(t, 5, result.MatchedCount)
	require.Equal(t, 4, result.Count)
	require.Equal(t, 1, result.VideoCount, "representative can be outside the type filter")
	p07GraphQL(t, r, `query { findMedia(media_filter:{collapse_stacks:true},filter:{per_page:2}) { count matched_count items { id stack_match_count image { visual_stack { member_count } } scene { visual_stack { member_count } } } } }`, nil)
}
func mapKeys(keys []string) map[string]bool {
	ret := map[string]bool{}
	for _, key := range keys {
		ret[key] = true
	}
	return ret
}

func TestP08ReviewedPairsRequireCurrentEvidenceAndNeverJoinTransitively(t *testing.T) {
	r := p07Fixture(t)
	refs := []*models.MediaReference{}
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		folder := &models.Folder{Path: t.TempDir()}
		require.NoError(t, r.Folder.Create(ctx, folder))
		for i := 1; i <= 3; i++ {
			path := filepath.Join(folder.Path, fmt.Sprintf("variant%d.png", i))
			require.NoError(t, os.WriteFile(path, []byte{byte(i)}, 0o600))
			stat, err := os.Stat(path)
			require.NoError(t, err)
			file := &models.ImageFile{BaseFile: &models.BaseFile{Path: path, Basename: filepath.Base(path), ParentFolderID: folder.ID, Size: stat.Size(), DirEntry: models.DirEntry{ModTime: stat.ModTime()}, Fingerprints: models.Fingerprints{{Type: "md5", Fingerprint: fmt.Sprintf("hash-%d", i)}, {Type: "phash", Fingerprint: int64(42)}}}, Format: "png", Width: 8, Height: 8}
			require.NoError(t, r.File.Create(ctx, file))
			require.NoError(t, r.Image.AddFileID(ctx, i, file.ID))
			_, err = r.Image.UpdatePartial(ctx, i, models.ImagePartial{PrimaryFileID: &file.ID})
			require.NoError(t, err)
			vector := make([]float32, sqlite.VisualEmbeddingDimensions)
			angle := float64(i-1) * math.Pi / 18
			vector[0] = float32(math.Cos(angle))
			vector[1] = float32(math.Sin(angle))
			require.NoError(t, sqlite.VisualEmbeddings.UpsertImage(ctx, i, vector, fmt.Sprintf("%s:%d:%d", path, stat.Size(), stat.ModTime().UnixNano())))
			ref := p08Ref(models.MediaKindImage, i)
			refs = append(refs, &ref)
		}
		return nil
	}))
	propose := func() []*models.VisualStackProposal {
		var ret []*models.VisualStackProposal
		require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
			var err error
			ret, err = r.VisualStack.Propose(ctx, refs)
			return err
		}))
		return ret
	}
	proposals := propose()
	require.Len(t, proposals, 2)
	require.Equal(t, []*models.MediaReference{refs[0], refs[1]}, proposals[0].Members)
	require.Equal(t, []*models.MediaReference{refs[1], refs[2]}, proposals[1].Members)
	require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
		for _, ref := range refs {
			stack, err := r.VisualStack.FindByMedia(ctx, *ref)
			require.NoError(t, err)
			require.Nil(t, stack)
		}
		return nil
	}))
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		vector := make([]float32, sqlite.VisualEmbeddingDimensions)
		vector[0] = 1
		return sqlite.VisualEmbeddings.UpsertImage(ctx, 2, vector, "stale-source")
	}))
	require.Empty(t, propose())
	p07GraphQL(t, r, `query { visualStackProposals(media:[{kind:IMAGE,id:"1"},{kind:IMAGE,id:"2"}]) { evidence members { kind id } } }`, nil)
}

func TestP08MigrationUpgrades95AndPreservesNativeRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.sqlite")
	database := sqlite.NewDatabase()
	require.NoError(t, database.Open(path))
	r := database.Repository()
	image := models.NewImage()
	image.Title = "Preserved native Image"
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error { return r.Image.Create(ctx, &models.CreateImageInput{Image: &image}) }))
	require.NoError(t, database.Close())
	// Reconstruct schema 95 by removing exactly the P08 migration's objects.
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`DROP TRIGGER visual_stack_member_deleted; DROP TABLE visual_stack_members; DROP TABLE visual_stacks; UPDATE schema_migrations SET version=95,dirty=0;`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	database = sqlite.NewDatabase()
	require.Error(t, database.Open(path))
	migrator, err := sqlite.NewMigrator(database)
	require.NoError(t, err)
	require.Equal(t, uint(96), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 96))
	migrator.Close()
	require.NoError(t, database.Open(path))
	defer database.Close()
	r = database.Repository()
	require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
		found, err := r.Image.Find(ctx, image.ID)
		require.NoError(t, err)
		require.Equal(t, image.Title, found.Title)
		stack, err := r.VisualStack.FindByMedia(ctx, p08Ref(models.MediaKindImage, image.ID))
		require.Nil(t, stack)
		return err
	}))
}

func TestP08HashAndDerivationPairsRequireReviewAndRespectBounds(t *testing.T) {
	r := p07Fixture(t)
	a, b := p08Ref(models.MediaKindImage, 1), p08Ref(models.MediaKindVideo, 1)
	refs := []*models.MediaReference{&a, &b}
	propose := func() []*models.VisualStackProposal {
		var ret []*models.VisualStackProposal
		require.NoError(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
			var err error
			ret, err = r.VisualStack.Propose(ctx, refs)
			return err
		}))
		return ret
	}
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		for _, id := range []models.FileID{1, 2} {
			require.NoError(t, r.File.ModifyFingerprints(ctx, id, []models.Fingerprint{{Type: "md5", Fingerprint: "same-active-hash"}}))
		}
		return nil
	}))
	proposals := propose()
	require.Len(t, proposals, 1)
	require.Contains(t, proposals[0].Evidence, "Identical")
	require.Equal(t, refs, proposals[0].Members)
	require.NoError(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		return r.File.ModifyFingerprints(ctx, 2, []models.Fingerprint{{Type: "md5", Fingerprint: "new-active-hash"}, {Type: "source_md5", Fingerprint: "same-active-hash"}})
	}))
	proposals = propose()
	require.Len(t, proposals, 1)
	require.Contains(t, proposals[0].Evidence, "Recorded source")
	p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(a, b), Representative: a})
	})
	require.Empty(t, propose(), "already grouped pairs are not proposed again")
	require.ErrorContains(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		members := make([]*models.VisualStackMemberInput, 201)
		_, err := r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: members, Representative: a})
		return err
	}), "200")
	require.ErrorContains(t, r.WithReadTxn(context.Background(), func(ctx context.Context) error {
		_, err := r.VisualStack.Propose(ctx, make([]*models.MediaReference, 101))
		return err
	}), "100")
}

func TestP08MergeValidatesVersionsAndRollsBackTransfers(t *testing.T) {
	r := p07Fixture(t)
	a, b, c, d := p08Ref(models.MediaKindImage, 1), p08Ref(models.MediaKindVideo, 1), p08Ref(models.MediaKindImage, 2), p08Ref(models.MediaKindVideo, 2)
	first := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(a, b), Representative: a})
	})
	second := p08Write(t, r, func(ctx context.Context) (*models.VisualStack, error) {
		return r.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: p08Members(c, d), Representative: c})
	})
	input := models.VisualStackMergeInput{Stacks: []*models.VisualStackVersionInput{{ID: first.ID, Version: first.Version}, {ID: second.ID, Version: second.Version}}, Representative: p08Ref(models.MediaKindImage, 999)}
	require.Error(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := r.VisualStack.Merge(ctx, input)
		return err
	}))
	require.Equal(t, first, p08Read(t, r, first.ID))
	require.Equal(t, second, p08Read(t, r, second.ID))
	input.Representative = a
	input.Stacks[1].Version = 0
	require.ErrorContains(t, r.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := r.VisualStack.Merge(ctx, input)
		return err
	}), "stack changed")
	require.Equal(t, first, p08Read(t, r, first.ID))
	require.Equal(t, second, p08Read(t, r, second.ID))
}
