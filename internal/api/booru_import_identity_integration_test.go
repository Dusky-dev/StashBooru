//go:build integration

package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

// A download whose Artist/Character/Copyright source names are connected to a
// native record joins that record instead of creating one per spelling.
func TestBooruImportUsesConnectedSourceNames(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	paw := booru.Source{ID: "paw-test", Name: "Pawchive", Engine: "pawchive", URL: "https://pawchive.pw"}
	data, err := json.Marshal([]booru.Source{paw})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600))

	connect := func(kind, name string, first models.ProviderIdentity, others ...models.ProviderIdentity) int {
		created := identityTestDecide(t, rs, "create", kind, []models.IdentityDecision{{ID: first.ID, Version: first.Version}}, name, 200)[0]
		for _, other := range others {
			identityTestDecide(t, rs, "link", kind, []models.IdentityDecision{{ID: other.ID, Version: other.Version, EntityID: created.EntityID}}, "", 200)
		}
		return created.EntityID
	}
	artist := connect("artist", "aki99",
		identityTestObserve(t, rs, dan, "artist", "aki99"),
		identityTestObserve(t, rs, gel, "artist", "aki99_(artist)"),
		identityTestObserve(t, rs, paw, "artist", "creator:patreon:4534453"))
	character := connect("character", "Hatsune Miku",
		identityTestObserve(t, rs, dan, "character", "hatsune_miku"),
		identityTestObserve(t, rs, gel, "character", "miku_(vocaloid)"))
	copyright := connect("copyright", "Vocaloid",
		identityTestObserve(t, rs, dan, "copyright", "vocaloid"),
		identityTestObserve(t, rs, gel, "copyright", "vocaloid_(series)"))

	countArtists := func() int {
		var count int
		require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
			count, err = rs.repository.Studio.Count(ctx)
			return err
		}))
		return count
	}
	before := countArtists()
	importTags := func(source booru.Source, tags ...booru.Tag) taggingResolvedEntities {
		var resolved taggingResolvedEntities
		require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
			resolved, err = ensureBooruImportMetadata(ctx, rs.repository, source, tags)
			return err
		}))
		return resolved
	}

	got := importTags(gel,
		booru.Tag{Name: "aki99_(artist)", Category: "artist"},
		booru.Tag{Name: "aki99", Category: "artist"},
		booru.Tag{Name: "miku_(vocaloid)", Category: "character"},
		booru.Tag{Name: "hatsune_miku", Category: "character"},
		booru.Tag{Name: "vocaloid_(series)", Category: "copyright"})
	require.Equal(t, []int{artist}, got.ArtistIDs, "connected and matching names share one Artist")
	require.Equal(t, []int{character}, got.CharacterIDs)
	// Every spelling keeps its prediction for Character–Copyright linking.
	require.Len(t, got.CharacterPredictions, 2)
	require.Equal(t, []int{character, character}, got.CharacterPredictionIDs)
	require.Equal(t, []int{copyright}, got.CopyrightIDs)
	require.Zero(t, got.CreatedArtists+got.CreatedCharacters+got.CreatedCopyrights)

	got = importTags(paw, booru.Tag{Name: "creator:patreon:4534453", Category: "artist"})
	require.Equal(t, []int{artist}, got.ArtistIDs, "a connected creator account joins the Artist")
	require.Equal(t, before, countArtists())

	// Names connected on another site, or not connected at all, keep the
	// existing name-based behaviour.
	got = importTags(dan,
		booru.Tag{Name: "aki99_(artist)", Category: "artist"},
		booru.Tag{Name: "miku_(vocaloid)", Category: "character"})
	require.NotEqual(t, []int{artist}, got.ArtistIDs)
	require.Equal(t, 1, got.CreatedArtists)
	require.Equal(t, 1, got.CreatedCharacters)

	// The profile lists those records as split: every connected name is
	// compared with each record's full name, with any disambiguation.
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		for _, split := range []struct {
			kind         string
			entity, want int
		}{{"artist", artist, got.ArtistIDs[0]}, {"character", character, got.CharacterIDs[0]}} {
			rows, err := rs.repository.ProviderIdentity.SplitRecords(ctx, split.kind, split.entity)
			require.NoError(t, err)
			ids := []int{}
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			require.Equal(t, []int{split.want}, ids, split.kind)
		}
		return nil
	}))
}

// Merging Characters keeps every source name connected, so later downloads of
// either spelling still join the remaining Character.
func TestCharacterMergeKeepsConnectedSourceNames(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	a := identityTestObserve(t, rs, dan, "character", "hatsune_miku")
	b := identityTestObserve(t, rs, gel, "character", "miku_(vocaloid)")
	keep := identityTestDecide(t, rs, "create", "character", []models.IdentityDecision{{ID: a.ID, Version: a.Version}}, "Hatsune Miku", 200)[0]
	merged := identityTestDecide(t, rs, "create", "character", []models.IdentityDecision{{ID: b.ID, Version: b.Version}}, "Miku (Vocaloid)", 200)[0]
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		return rs.repository.Performer.Merge(ctx, []int{merged.EntityID}, keep.EntityID)
	}))
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		current, err := rs.repository.ProviderIdentity.Find(ctx, b.ID)
		require.NoError(t, err)
		require.Equal(t, keep.EntityID, current.EntityID)
		return nil
	}))
}
