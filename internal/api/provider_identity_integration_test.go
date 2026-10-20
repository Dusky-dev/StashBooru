//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

func identityTestRoutes(t *testing.T) (booruBrowserRoutes, *sqlite.Database, string) {
	t.Helper()
	config.InitializeEmpty()
	path := filepath.Join(t.TempDir(), "identities.sqlite")
	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	return booruBrowserRoutes{repository: db.Repository(), configPath: t.TempDir()}, db, path
}

func identityTestObserve(t *testing.T, rs booruBrowserRoutes, source booru.Source, kind, target string) models.ProviderIdentity {
	t.Helper()
	body := feedRequest(t, rs.IdentityMutation, identityMutation{Action: "observe", Kind: kind, Source: source.ID, Instance: source.Instance(), Target: target}, 200)
	var row models.ProviderIdentity
	require.NoError(t, json.Unmarshal(body, &row))
	return row
}

func identityTestDecide(t *testing.T, rs booruBrowserRoutes, action, kind string, changes []models.IdentityDecision, name string, status int) []models.ProviderIdentity {
	t.Helper()
	body := feedRequest(t, rs.IdentityMutation, identityMutation{Action: action, Kind: kind, Changes: changes, Name: name}, status)
	rows := []models.ProviderIdentity{}
	if status == 200 {
		require.NoError(t, json.Unmarshal(body, &rows))
	}
	return rows
}

func TestProviderIdentityCrossSiteLinkRenameSearchUnlinkAndRestart(t *testing.T) {
	rs, db, path := identityTestRoutes(t)
	dan, gel := booru.Defaults()[0], booru.Defaults()[2]
	a := identityTestObserve(t, rs, dan, "artist", "waligner")
	b := identityTestObserve(t, rs, gel, "artist", "waligner_(artist)")
	created := identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: a.ID, Version: a.Version}}, "Waligner", 200)[0]
	b = identityTestDecide(t, rs, "link", "artist", []models.IdentityDecision{{ID: b.ID, Version: b.Version, EntityID: created.EntityID}}, "", 200)[0]
	require.Equal(t, created.EntityID, b.EntityID)
	paw := booru.Source{ID: "paw-test", Name: "Pawchive", Engine: "pawchive", URL: "https://pawchive.pw"}
	data, err := json.Marshal([]booru.Source{paw})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600))
	account := identityTestObserve(t, rs, paw, "artist", "creator:patreon:49481727")
	require.Equal(t, "account", account.Form)
	account = identityTestDecide(t, rs, "link", "artist", []models.IdentityDecision{{ID: account.ID, Version: account.Version, EntityID: created.EntityID}}, "", 200)[0]
	posts := []booru.Post{{Tags: []booru.Tag{{Name: b.Target, Category: "artist"}, {Name: "lowres", Category: "general"}}}}
	rs.captureIdentityPosts(context.Background(), gel, posts)
	require.Equal(t, "Waligner", posts[0].Tags[0].Label)
	require.Equal(t, "waligner_(artist)", posts[0].Tags[0].Name)
	require.NotEmpty(t, posts[0].Tags[0].IdentityID)
	require.Empty(t, posts[0].Tags[1].IdentityID)
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := rs.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: created.EntityID, Name: models.NewOptionalString("New display name")})
		return err
	}))
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		current, err := rs.repository.ProviderIdentity.Find(ctx, account.ID)
		require.NoError(t, err)
		require.Equal(t, "New display name", current.EntityName)
		query := account.Target
		studios, count, err := rs.repository.Studio.Query(ctx, nil, &models.FindFilterType{Q: &query})
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.Equal(t, created.EntityID, studios[0].ID)
		return nil
	}))
	require.NoError(t, db.Close())
	db = sqlite.NewDatabase()
	require.NoError(t, db.Open(path))
	defer db.Close()
	rs.repository = db.Repository()
	b = identityTestDecide(t, rs, "unlink", "artist", []models.IdentityDecision{{ID: b.ID, Version: b.Version}}, "", 200)[0]
	require.Zero(t, b.EntityID)
	require.Equal(t, "waligner_(artist)", b.Target)
	rs.captureIdentityPosts(context.Background(), gel, posts)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		current, err := rs.repository.ProviderIdentity.Find(ctx, b.ID)
		require.NoError(t, err)
		require.Zero(t, current.EntityID, "observing again must not silently re-link")
		return nil
	}))
	feedRequest(t, rs.IdentityMutation, identityMutation{Action: "observe", Kind: "general", Source: dan.ID, Instance: dan.Instance(), Target: "lowres"}, 400)
}

func TestProviderIdentityVersionsAtomicCreateRejectionsAndNativeDeletion(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	a := identityTestObserve(t, rs, source, "artist", "waligner")
	b := identityTestObserve(t, rs, source, "artist", "waligner_(artist)")
	identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: a.ID, Version: 1}, {ID: b.ID, Version: 99}}, "Must not exist", 409)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		count, err := rs.repository.Studio.Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)
		return nil
	}))
	a = identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: a.ID, Version: 1}}, "Waligner", 200)[0]
	b = identityTestDecide(t, rs, "reject", "artist", []models.IdentityDecision{{ID: b.ID, Version: 1, EntityID: a.EntityID}}, "", 200)[0]
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.Candidates(ctx, "artist", "Waligner", b.ID, false)
		require.NoError(t, err)
		require.Empty(t, rows)
		rows, err = rs.repository.ProviderIdentity.Candidates(ctx, "artist", "Waligner", b.ID, true)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.True(t, rows[0].Rejected)
		return nil
	}))
	b = identityTestDecide(t, rs, "restore", "artist", []models.IdentityDecision{{ID: b.ID, Version: b.Version, EntityID: a.EntityID}}, "", 200)[0]
	identityTestDecide(t, rs, "link", "artist", []models.IdentityDecision{{ID: b.ID, Version: 1, EntityID: a.EntityID}}, "", 409)
	identityTestDecide(t, rs, "link", "copyright", []models.IdentityDecision{{ID: b.ID, Version: b.Version, EntityID: a.EntityID}}, "", 409)
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		return rs.repository.Studio.Destroy(ctx, a.EntityID)
	}))
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		row, err := rs.repository.ProviderIdentity.Find(ctx, a.ID)
		require.NoError(t, err)
		require.Zero(t, row.EntityID)
		require.Greater(t, row.Version, a.Version)
		return nil
	}))
	identityTestDecide(t, rs, "unlink", "artist", []models.IdentityDecision{{ID: a.ID, Version: a.Version}}, "", 409)
}

func TestProviderIdentityScopeAliasesCategoriesAndPagination(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	original := identityTestObserve(t, rs, source, "artist", "same_name")
	alias := source
	alias.ID, alias.Name = "alias-dan", "An alias"
	data, err := json.Marshal([]booru.Source{alias})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), data, 0600))
	duplicate := identityTestObserve(t, rs, alias, "artist", "same_name")
	require.Equal(t, original.ID, duplicate.ID)
	character := identityTestObserve(t, rs, source, "character", "same_name")
	copyright := identityTestObserve(t, rs, source, "copyright", "same_name")
	other := identityTestObserve(t, rs, booru.Defaults()[2], "artist", "same_name")
	require.NotEqual(t, original.ID, character.ID)
	require.NotEqual(t, character.ID, copyright.ID)
	require.NotEqual(t, original.ID, other.ID)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		page, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Limit: 2})
		require.NoError(t, err)
		require.Len(t, page, 2)
		next, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Limit: 2, After: page[1].ID})
		require.NoError(t, err)
		require.Len(t, next, 2)
		require.Greater(t, next[0].ID, page[1].ID)
		return nil
	}))
	for _, query := range []string{"?after=bad", "?identity=-1", "?kind=general", "?entity=2", "?state=anything"} {
		w := httptest.NewRecorder()
		rs.IdentityList(w, httptest.NewRequest("GET", "/identities/list"+query, nil))
		require.Equal(t, 400, w.Code)
	}
}

func TestProviderIdentityMigration99PreservesCatalogue(t *testing.T) {
	rs, db, path := identityTestRoutes(t)
	image := models.NewImage()
	image.Title = "Keep me"
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		return rs.repository.Image.Create(ctx, &models.CreateImageInput{Image: &image})
	}))
	require.NoError(t, db.Close())
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec("DROP TABLE provider_identity_rejections; DROP TABLE provider_identities; UPDATE schema_migrations SET version=99,dirty=0")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(100), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 100))
	migrator.Close()
	require.NoError(t, db.Open(path))
	defer db.Close()
	repo := db.Repository()
	require.NoError(t, repo.WithReadTxn(context.Background(), func(ctx context.Context) error {
		found, err := repo.Image.Find(ctx, image.ID)
		require.NoError(t, err)
		require.Equal(t, "Keep me", found.Title)
		return nil
	}))
}
