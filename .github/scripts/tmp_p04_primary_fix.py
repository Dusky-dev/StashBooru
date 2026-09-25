from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one anchor, found {count}: {old[:80]!r}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "pkg/sqlite/copyright_taxonomy.go",
    '''func setPrimaryCopyrightID(ctx context.Context, table, mediaColumn string, mediaID int, copyrightID *int) error {
\tif _, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE "+mediaColumn+" = ?", mediaID); err != nil {
\t\treturn err
\t}
\tif copyrightID == nil {
\t\treturn nil
\t}
\t_, err := dbWrapper.Exec(ctx, "INSERT INTO "+table+" ("+mediaColumn+", copyright_id) VALUES (?, ?)", mediaID, *copyrightID)
\treturn err
}''',
    '''func setPrimaryCopyrightID(ctx context.Context, table, mediaColumn string, mediaID int, copyrightID *int) error {
\tif copyrightID == nil {
\t\t_, err := dbWrapper.Exec(ctx, "DELETE FROM "+table+" WHERE "+mediaColumn+" = ?", mediaID)
\t\treturn err
\t}

\t// Replace the primary atomically. If the composite foreign key rejects the
\t// requested Copyright because it is not associated with the media item, the
\t// existing primary remains untouched.
\t_, err := dbWrapper.Exec(ctx,
\t\t"INSERT INTO "+table+" ("+mediaColumn+", copyright_id) VALUES (?, ?) "+
\t\t\t"ON CONFLICT("+mediaColumn+") DO UPDATE SET copyright_id = excluded.copyright_id",
\t\tmediaID, *copyrightID)
\treturn err
}''',
)

replace_once(
    "pkg/sqlite/copyright.go",
    '''func (s *CopyrightStore) SetImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error {
\treturn setMediaCopyrights(ctx, imagesCopyrightsTable, "image_id", imageID, copyrightIDs, true)
}''',
    '''func containsCopyrightID(ids []int, wanted int) bool {
\tfor _, id := range ids {
\t\tif id == wanted {
\t\t\treturn true
\t\t}
\t}
\treturn false
}

func (s *CopyrightStore) SetImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error {
\tprimaryID, err := s.PrimaryImageCopyrightID(ctx, imageID)
\tif err != nil {
\t\treturn err
\t}
\tif err := setMediaCopyrights(ctx, imagesCopyrightsTable, "image_id", imageID, copyrightIDs, true); err != nil {
\t\treturn err
\t}
\tif primaryID != nil && containsCopyrightID(copyrightIDs, *primaryID) {
\t\treturn s.SetPrimaryImageCopyright(ctx, imageID, primaryID)
\t}
\treturn nil
}''',
)

replace_once(
    "pkg/sqlite/copyright.go",
    '''func (s *CopyrightStore) SetSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error {
\treturn setMediaCopyrights(ctx, scenesCopyrightsTable, "scene_id", sceneID, copyrightIDs, true)
}''',
    '''func (s *CopyrightStore) SetSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error {
\tprimaryID, err := s.PrimarySceneCopyrightID(ctx, sceneID)
\tif err != nil {
\t\treturn err
\t}
\tif err := setMediaCopyrights(ctx, scenesCopyrightsTable, "scene_id", sceneID, copyrightIDs, true); err != nil {
\t\treturn err
\t}
\tif primaryID != nil && containsCopyrightID(copyrightIDs, *primaryID) {
\t\treturn s.SetPrimarySceneCopyright(ctx, sceneID, primaryID)
\t}
\treturn nil
}''',
)

replace_once(
    "pkg/sqlite/copyright_taxonomy_test.go",
    '''\tinvalidPrimary := 4
\trequire.Error(t, store.SetPrimaryImageCopyright(ctx, 20, &invalidPrimary), "primary Copyright must already be associated with the image")
\tstoredPrimary, err := store.PrimaryImageCopyrightID(ctx, 20)
\trequire.NoError(t, err)
\trequire.Nil(t, storedPrimary, "failed replacement must not leave a primary Copyright behind")

\tprimary = 3
\trequire.NoError(t, store.SetPrimarySceneCopyright(ctx, 30, &primary))
\torderedScenes, err := store.FindBySceneIDOrdered(ctx, 30)
\trequire.NoError(t, err)
\trequire.Equal(t, []int{3, 2}, []int{orderedScenes[0].ID, orderedScenes[1].ID})

\trequire.NoError(t, store.SetImageCopyrights(ctx, 20, []int{2}))
\tstoredPrimary, err = store.PrimaryImageCopyrightID(ctx, 20)
\trequire.NoError(t, err)
\trequire.Nil(t, storedPrimary, "removing an association must cascade-delete the matching primary row")''',
    '''\tinvalidPrimary := 4
\trequire.Error(t, store.SetPrimaryImageCopyright(ctx, 20, &invalidPrimary), "primary Copyright must already be associated with the image")
\tstoredPrimary, err := store.PrimaryImageCopyrightID(ctx, 20)
\trequire.NoError(t, err)
\trequire.NotNil(t, storedPrimary)
\trequire.Equal(t, 2, *storedPrimary, "a rejected replacement must preserve the previous primary")

\t// Replacing the association set temporarily deletes/reinserts join rows. The
\t// primary should survive when it is still part of the final set.
\trequire.NoError(t, store.SetImageCopyrights(ctx, 20, []int{3, 2}))
\tstoredPrimary, err = store.PrimaryImageCopyrightID(ctx, 20)
\trequire.NoError(t, err)
\trequire.NotNil(t, storedPrimary)
\trequire.Equal(t, 2, *storedPrimary)

\tprimary = 3
\trequire.NoError(t, store.SetPrimarySceneCopyright(ctx, 30, &primary))
\torderedScenes, err := store.FindBySceneIDOrdered(ctx, 30)
\trequire.NoError(t, err)
\trequire.Equal(t, []int{3, 2}, []int{orderedScenes[0].ID, orderedScenes[1].ID})
\trequire.NoError(t, store.SetSceneCopyrights(ctx, 30, []int{2, 3}))
\tstoredScenePrimary, err := store.PrimarySceneCopyrightID(ctx, 30)
\trequire.NoError(t, err)
\trequire.NotNil(t, storedScenePrimary)
\trequire.Equal(t, 3, *storedScenePrimary)

\trequire.NoError(t, store.SetImageCopyrights(ctx, 20, []int{3}))
\tstoredPrimary, err = store.PrimaryImageCopyrightID(ctx, 20)
\trequire.NoError(t, err)
\trequire.Nil(t, storedPrimary, "removing the primary association must clear the primary row")''',
)

replace_once(
    "ui/v2.5/src/components/Images/ImageDetails/Image.tsx",
    'copyrights={image.copyrights}',
    'copyrights={image.ordered_copyrights ?? image.copyrights}',
)
