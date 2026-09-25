from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one anchor, found {count}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "pkg/sqlite/copyright.go",
    '''func replaceCopyrightRelations(ctx context.Context, id int, ids []int, parents bool) error {
\tcolumn := "child_id"
\tother := "parent_id"
\tif !parents {
\t\tcolumn, other = "parent_id", "child_id"
\t}
\tif _, err := dbWrapper.Exec(ctx, "DELETE FROM copyright_relations WHERE "+column+" = ?", id); err != nil {
\t\treturn err
\t}
\tfor _, related := range ids {
\t\tif related == id {
\t\t\treturn fmt.Errorf("a copyright cannot be its own parent or child")
\t\t}
\t\tquery := "INSERT OR IGNORE INTO copyright_relations (" + column + ", " + other + ") VALUES (?, ?)"
\t\tif _, err := dbWrapper.Exec(ctx, query, id, related); err != nil {
\t\t\treturn err
\t\t}
\t}
\treturn nil
}''',
    '''func replaceCopyrightRelations(ctx context.Context, id int, ids []int, parents bool) error {
\tcolumn := "child_id"
\tother := "parent_id"
\tif !parents {
\t\tcolumn, other = "parent_id", "child_id"
\t}

\t// Apply relationship changes as a delta instead of deleting and reinserting
\t// every edge. copyright_relation_order has an FK to the edge itself, so an
\t// unchanged edge must remain intact for its manual sibling position to
\t// survive ordinary Copyright edits.
\tdeduped := make([]int, 0, len(ids))
\tseen := make(map[int]struct{}, len(ids))
\tfor _, related := range ids {
\t\tif related == id {
\t\t\treturn fmt.Errorf("a copyright cannot be its own parent or child")
\t\t}
\t\tif _, ok := seen[related]; ok {
\t\t\tcontinue
\t\t}
\t\tseen[related] = struct{}{}
\t\tdeduped = append(deduped, related)
\t}

\tif len(deduped) == 0 {
\t\tif _, err := dbWrapper.Exec(ctx, "DELETE FROM copyright_relations WHERE "+column+" = ?", id); err != nil {
\t\t\treturn err
\t\t}
\t} else {
\t\targs := make([]interface{}, 0, len(deduped)+1)
\t\targs = append(args, id)
\t\tfor _, related := range deduped {
\t\t\targs = append(args, related)
\t\t}
\t\tquery := "DELETE FROM copyright_relations WHERE " + column + " = ? AND " + other + " NOT IN " + getInBinding(len(deduped))
\t\tif _, err := dbWrapper.Exec(ctx, query, args...); err != nil {
\t\t\treturn err
\t\t}
\t}

\tfor _, related := range deduped {
\t\tquery := "INSERT OR IGNORE INTO copyright_relations (" + column + ", " + other + ") VALUES (?, ?)"
\t\tif _, err := dbWrapper.Exec(ctx, query, id, related); err != nil {
\t\t\treturn err
\t\t}
\t}
\treturn nil
}''',
)

replace_once(
    "pkg/sqlite/copyright_taxonomy_test.go",
    '''\trequire.NoError(t, store.SetChildOrder(ctx, 1, []int{2, 3}))
\tchildren, err = store.FindOrderedChildren(ctx, 1)
\trequire.NoError(t, err)
\trequire.Equal(t, []int{2, 3}, []int{children[0].ID, children[1].ID})
\trequire.Error(t, store.SetChildOrder(ctx, 1, []int{2, 2}))''',
    '''\trequire.NoError(t, store.SetChildOrder(ctx, 1, []int{2, 3}))
\tchildren, err = store.FindOrderedChildren(ctx, 1)
\trequire.NoError(t, err)
\trequire.Equal(t, []int{2, 3}, []int{children[0].ID, children[1].ID})

\t// Saving unchanged relationships from either side must not delete the edge
\t// and cascade away the manual order metadata.
\tupdatedName := "Root renamed"
\t_, err = store.Update(ctx, models.CopyrightUpdateInput{ID: "1", Name: &updatedName, ChildIDs: []string{"2", "3"}})
\trequire.NoError(t, err)
\t_, err = store.Update(ctx, models.CopyrightUpdateInput{ID: "2", ParentIDs: []string{"1"}})
\trequire.NoError(t, err)
\tchildren, err = store.FindOrderedChildren(ctx, 1)
\trequire.NoError(t, err)
\trequire.Equal(t, []int{2, 3}, []int{children[0].ID, children[1].ID}, "ordinary hierarchy saves must preserve manual sibling order")

\trequire.Error(t, store.SetChildOrder(ctx, 1, []int{2, 2}))''',
)
