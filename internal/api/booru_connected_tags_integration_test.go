//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

// Paheal sends every tag uncategorised. A plain tag connected to a Character
// is listed as that Character while browsing and joins it on download.
func TestConnectedPlainTagsActAsTheirKind(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	paheal := booru.Source{ID: "paheal", Name: "Paheal", Engine: "shimmie", URL: "https://rule34.paheal.net"}
	row := identityTestObserve(t, rs, paheal, "character", "Hatsune_Miku")
	character := identityTestDecide(t, rs, "create", "character", []models.IdentityDecision{{ID: row.ID, Version: row.Version}}, "Hatsune Miku", 200)[0].EntityID

	posts := []booru.Post{{ID: "1", Tags: []booru.Tag{
		{Name: "Hatsune_Miku", Category: "unknown"},
		{Name: "smile", Category: "unknown"},
	}}}
	rs.captureIdentityPosts(context.Background(), paheal, posts)
	require.Equal(t, "character", posts[0].Tags[0].Category, "a connected plain tag is listed as its kind")
	require.Equal(t, "Hatsune Miku", posts[0].Tags[0].CanonicalName)
	require.NotEmpty(t, posts[0].Tags[0].IdentityID)
	require.Equal(t, "unknown", posts[0].Tags[1].Category, "other plain tags stay plain")

	var resolved taggingResolvedEntities
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		var err error
		resolved, err = ensureBooruImportMetadata(ctx, rs.repository, paheal, []booru.Tag{
			{Name: "Hatsune_Miku", Category: "unknown"},
			{Name: "smile", Category: "unknown"},
		})
		return err
	}))
	require.Equal(t, []int{character}, resolved.CharacterIDs, "the download joins the connected Character")
	require.Zero(t, resolved.CreatedCharacters)
	require.Len(t, resolved.TagIDs, 1, "the plain tag stays a Tag")
}
