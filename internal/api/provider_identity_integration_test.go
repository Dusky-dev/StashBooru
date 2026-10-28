//go:build integration

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/stashapp/stash/pkg/job"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	require.Equal(t, "Waligner", posts[0].Tags[0].CanonicalName)
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
		require.Empty(t, posts[0].Tags[0].CanonicalName)
		require.Empty(t, posts[0].Tags[0].Label)
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
	_, err = conn.Exec("DROP TABLE provider_identity_rejections; DROP TABLE provider_identities; UPDATE schema_migrations SET version=99,dirty=0; ALTER TABLE tags DROP COLUMN hide_thumbnail; DROP TRIGGER IF EXISTS image_visual_embedding_cleanup")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	db = sqlite.NewDatabase()
	require.Error(t, db.Open(path))
	migrator, err := sqlite.NewMigrator(db)
	require.NoError(t, err)
	require.Equal(t, uint(102), migrator.RequiredSchemaVersion())
	require.NoError(t, migrator.RunMigration(context.Background(), 100))
	require.NoError(t, migrator.RunMigration(context.Background(), 101))
	require.NoError(t, migrator.RunMigration(context.Background(), 102))
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

func TestProviderIdentityCharacterCopyrightAndAtomicTargetFailure(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	first := identityTestObserve(t, rs, source, "character", "saber_(fate)")
	second := identityTestObserve(t, rs, source, "character", "artoria_pendragon")
	body := feedRequest(t, rs.IdentityMutation, identityMutation{Action: "create", Kind: "character", Name: "Saber", Disambiguation: "Fate", Changes: []models.IdentityDecision{{ID: first.ID, Version: 1}}}, 200)
	var linked []models.ProviderIdentity
	require.NoError(t, json.Unmarshal(body, &linked))
	first = linked[0]
	// A failure after the first write rolls back the whole batch.
	identityTestDecide(t, rs, "link", "character", []models.IdentityDecision{{ID: second.ID, Version: 1, EntityID: first.EntityID}, {ID: first.ID, Version: first.Version, EntityID: 999999}}, "", 409)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		row, err := rs.repository.ProviderIdentity.Find(ctx, second.ID)
		require.NoError(t, err)
		require.Zero(t, row.EntityID)
		require.Equal(t, 1, row.Version)
		matches, err := rs.repository.ProviderIdentity.Candidates(ctx, "character", "Saber", second.ID, false)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		require.Equal(t, "Fate", matches[0].Disambiguation)
		qualified, err := rs.repository.ProviderIdentity.Candidates(ctx, "character", "saber (fate)", second.ID, false)
		require.NoError(t, err)
		require.Len(t, qualified, 1)
		require.Equal(t, first.EntityID, qualified[0].ID)
		q := first.Target
		rows, count, err := rs.repository.Performer.Query(ctx, nil, &models.FindFilterType{Q: &q})
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.Equal(t, first.EntityID, rows[0].ID)
		return nil
	}))
	copyright := identityTestObserve(t, rs, source, "copyright", "fate_(series)")
	copyright = identityTestDecide(t, rs, "create", "copyright", []models.IdentityDecision{{ID: copyright.ID, Version: 1}}, "Fate", 200)[0]
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		q := copyright.Target
		rows, count, err := rs.repository.Copyright.Query(ctx, &models.FindFilterType{Q: &q})
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.Equal(t, copyright.EntityID, rows[0].ID)
		parents, err := rs.repository.Copyright.FindParents(ctx, copyright.EntityID)
		require.NoError(t, err)
		require.Empty(t, parents)
		return nil
	}))
}

func TestProviderIdentityScanReceiptsFeedBoundsDecisionsAndCancellation(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	a := identityTestObserve(t, rs, source, "artist", "kept_artist")
	a = identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: a.ID, Version: 1}}, "Curated name", 200)[0]
	post := booru.Post{ID: "42", Tags: []booru.Tag{{Name: a.Target, Category: "artist", Label: "Remote name"}, {Name: "receipt_character", Category: "character"}, {Name: "lowres", Category: "general"}}}
	require.NoError(t, rs.saveDownloadReceipt(booruDownloadReceipt{Source: booru.PublicSource{ID: source.ID, Name: source.Name, Engine: source.Engine, URL: source.URL, Instance: source.Instance()}, Post: post}))
	directory := filepath.Join(rs.configPath, "booru-download-history")
	receipt, ok := readIdentityReceipt(filepath.Join(directory, receiptName(source.Instance(), post.ID)))
	require.True(t, ok)
	require.Equal(t, "42", receipt.Post.ID)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "broken.json"), []byte("{"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "large.json"), make([]byte, 64<<10+1), 0600))
	require.NoError(t, os.Symlink(filepath.Join(directory, receiptName(source.Instance(), post.ID)), filepath.Join(directory, "linked.json")))
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		follow, err := rs.repository.BooruFollow.Create(ctx, models.BooruReviewProvider{Instance: source.Instance(), SourceID: source.ID, Engine: source.Engine, URL: source.URL}, models.BooruFollowDefinition{Name: "Saved", Kind: "artist", Target: a.Target})
		if err != nil {
			return err
		}
		payload, err := json.Marshal(followingStoredPost{Post: booru.Post{Tags: []booru.Tag{{Name: "feed_copyright", Category: "copyright"}}}})
		if err != nil {
			return err
		}
		_, err = rs.repository.Following.ApplyPage(ctx, *follow, 0, "opaque", false, []models.FollowingSnapshot{{PostID: "9", Payload: payload}})
		return err
	}))
	// Invalid current settings must not hide saved source provenance.
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte("{"), 0600))
	var scanned, observations, skipped int
	progress := func(s, o, k int) { scanned, observations, skipped = s, o, k }
	require.NoError(t, rs.scanIdentityMetadata(context.Background(), progress))
	require.Equal(t, 5, scanned)
	require.Equal(t, 3, observations)
	require.Equal(t, 3, skipped)
	require.NoError(t, rs.scanIdentityMetadata(context.Background(), progress))
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Limit: 51})
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for _, row := range rows {
			require.Equal(t, source.Name, row.SourceName, "follow labels must not replace provider names")
		}
		row, err := rs.repository.ProviderIdentity.Find(ctx, a.ID)
		require.NoError(t, err)
		require.Equal(t, a.EntityID, row.EntityID)
		require.Equal(t, a.Version, row.Version)
		require.Equal(t, "Curated name", row.EntityName)
		require.Equal(t, "Remote name", row.Label)
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, rs.scanIdentityMetadata(ctx, progress), context.Canceled)
	// Recovery never removes bindings or explicit unlink/reject decisions.
	require.NoError(t, os.Remove(filepath.Join(rs.configPath, "booru-browser.json")))
	untouched := identityTestObserve(t, rs, source, "artist", "unreviewed")
	unlinked := identityTestObserve(t, rs, source, "artist", "explicit_unlink")
	identityTestDecide(t, rs, "unlink", "artist", []models.IdentityDecision{{ID: unlinked.ID, Version: 1}}, "", 200)
	feedRequest(t, rs.IdentityMutation, identityMutation{Action: "discard", Kind: "artist"}, 200)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		gone, err := rs.repository.ProviderIdentity.Find(ctx, untouched.ID)
		require.NoError(t, err)
		require.Nil(t, gone)
		kept, err := rs.repository.ProviderIdentity.Find(ctx, unlinked.ID)
		require.NoError(t, err)
		require.NotNil(t, kept)
		return nil
	}))
}

func TestProviderIdentityCapacityKeepsExistingConnectionsVisible(t *testing.T) {
	rs, db, path := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	a := identityTestObserve(t, rs, source, "artist", "kept")
	a = identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: a.ID, Version: 1}}, "Kept canonical", 200)[0]
	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = conn.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<99999)
 INSERT INTO provider_identities(instance,source_id,source_name,engine,source_url,kind,form,target,label,updated_at)
 SELECT p.instance,p.source_id,p.source_name,p.engine,p.source_url,'artist','tag','capacity_'||n.x,'capacity','2026-10-20' FROM n CROSS JOIN provider_identities p WHERE p.id=?`, a.ID)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	feedRequest(t, rs.IdentityMutation, identityMutation{Action: "observe", Kind: "artist", Source: source.ID, Instance: source.Instance(), Target: "new_at_capacity"}, 409)
	// A mixed fresh/known page must still project existing bindings.
	posts := []booru.Post{{Tags: []booru.Tag{{Name: "new_at_capacity", Category: "artist"}, {Name: a.Target, Category: "artist", Label: "Original provider label"}}}}
	rs.captureIdentityPosts(context.Background(), source, posts)
	require.Equal(t, "Kept canonical", posts[0].Tags[1].CanonicalName)
	require.Equal(t, "Original provider label", posts[0].Tags[1].Label)
	require.NotEmpty(t, posts[0].Tags[1].IdentityID)
	feedRequest(t, rs.IdentityMutation, identityMutation{Action: "discard", Kind: "artist"}, 200)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Limit: 51})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, a.ID, rows[0].ID)
		return nil
	}))
}

func TestProviderIdentityScanQueuedCancelAndDuplicateStart(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	jobs := job.NewManager()
	defer jobs.Stop()
	rs.jobs = jobs
	entered := make(chan struct{})
	release := make(chan struct{})
	jobs.Add(context.Background(), "hold", job.MakeJobExec(func(context.Context, *job.Progress) error { close(entered); <-release; return nil }))
	<-entered
	defer close(release)
	first := feedRequest(t, rs.IdentityScan, map[string]any{"action": "start"}, 200)
	var state identityScanState
	require.NoError(t, json.Unmarshal(first, &state))
	require.Equal(t, "queued", state.Status)
	second := feedRequest(t, rs.IdentityScan, map[string]any{"action": "start"}, 200)
	var duplicate identityScanState
	require.NoError(t, json.Unmarshal(second, &duplicate))
	require.Equal(t, state.JobID, duplicate.JobID)
	jobs.CancelJob(state.JobID)
	w := httptest.NewRecorder()
	rs.IdentityScan(w, httptest.NewRequest("GET", "/identities/scan", nil))
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &duplicate))
	require.Equal(t, "cancelled", duplicate.Status)
	restarted := feedRequest(t, rs.IdentityScan, map[string]any{"action": "start"}, 200)
	require.NoError(t, json.Unmarshal(restarted, &state))
	require.Equal(t, "queued", state.Status)
	feedRequest(t, rs.IdentityScan, map[string]any{"action": "cancel", "jobID": state.JobID + 1}, 409)
	feedRequest(t, rs.IdentityScan, map[string]any{"action": "cancel", "jobID": state.JobID}, 200)
	require.Eventually(t, func() bool {
		w := httptest.NewRecorder()
		rs.IdentityScan(w, httptest.NewRequest("GET", "/identities/scan", nil))
		require.Equal(t, 200, w.Code)
		var result identityScanState
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		return result.Status == "cancelled"
	}, time.Second, 10*time.Millisecond)
}

func TestProviderIdentityAccountServiceScopeAndDirectoryLabelPriority(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Source{ID: "paw", Name: "Pawchive", Engine: "pawchive", URL: "https://pawchive.pw"}
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.Observe(ctx, []models.IdentityObservation{
			identityObservation(source, "artist", "creator:patreon:49481727", "Waligner", "", 2),
			identityObservation(source, "artist", "creator:fanbox:49481727", "Different account", "", 2),
		})
		require.NoError(t, err)
		require.NotEqual(t, rows[0].ID, rows[1].ID)
		return nil
	}))
	posts := []booru.Post{{Tags: []booru.Tag{{Name: "creator:patreon:49481727", Category: "artist", Label: "patreon/49481727"}}}}
	rs.captureIdentityPosts(context.Background(), source, posts)
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Kind: "artist", Limit: 51})
		require.NoError(t, err)
		require.Len(t, rows, 2)
		require.Equal(t, "Waligner", rows[0].Label)
		require.Zero(t, rows[0].EntityID, "matching names are never automatic proof")
		return nil
	}))
	require.Equal(t, "patreon/49481727", posts[0].Tags[0].Label)
}

func TestProviderIdentitySearchTermsSpanAllConnections(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	for _, kind := range []string{"artist", "character"} {
		a := identityTestObserve(t, rs, source, kind, "foo")
		b := identityTestObserve(t, rs, source, kind, "bar")
		other := identityTestObserve(t, rs, source, kind, "unrelated")
		a = identityTestDecide(t, rs, "create", kind, []models.IdentityDecision{{ID: a.ID, Version: 1}}, "Canonical", 200)[0]
		identityTestDecide(t, rs, "link", kind, []models.IdentityDecision{{ID: b.ID, Version: 1, EntityID: a.EntityID}}, "", 200)
		other = identityTestDecide(t, rs, "create", kind, []models.IdentityDecision{{ID: other.ID, Version: 1}}, "Other", 200)[0]
		for _, test := range []struct {
			q  string
			id int
		}{{"-foo", other.EntityID}, {"foo bar", a.EntityID}, {"foo OR missing", a.EntityID}, {"bar -foo", 0}} {
			require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
				var ids []int
				var count int
				if kind == "artist" {
					rows, n, err := rs.repository.Studio.Query(ctx, nil, &models.FindFilterType{Q: &test.q})
					if err != nil {
						return err
					}
					count = n
					for _, row := range rows {
						ids = append(ids, row.ID)
					}
				} else {
					rows, n, err := rs.repository.Performer.Query(ctx, nil, &models.FindFilterType{Q: &test.q})
					if err != nil {
						return err
					}
					count = n
					for _, row := range rows {
						ids = append(ids, row.ID)
					}
				}
				if test.id == 0 {
					require.Zero(t, count)
					require.Empty(t, ids)
				} else {
					require.Equal(t, 1, count)
					require.Equal(t, []int{test.id}, ids)
				}
				return nil
			}), kind+" "+test.q)
		}
	}
}

type identityExpiredWriteManager struct {
	models.TxnManager
	recovered bool
}

func (m *identityExpiredWriteManager) Begin(ctx context.Context, writable bool) (context.Context, error) {
	if writable {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m.recovered = ctx.Err() == nil
	return m.TxnManager.Begin(ctx, false)
}
func TestProviderIdentityRecoveryReadOutlivesWriteTimeout(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	row := identityTestObserve(t, rs, source, "artist", "slow_refresh")
	identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: row.ID, Version: 1}}, "Still connected", 200)
	manager := &identityExpiredWriteManager{TxnManager: rs.repository.TxnManager}
	rs.repository.TxnManager = manager
	posts := []booru.Post{{Tags: []booru.Tag{{Name: row.Target, Category: "artist"}}}}
	rs.captureIdentityPosts(context.Background(), source, posts)
	require.True(t, manager.recovered)
	require.Equal(t, "Still connected", posts[0].Tags[0].CanonicalName)
	require.NotEmpty(t, posts[0].Tags[0].IdentityID)
}

type identityBlockingStore struct {
	models.ProviderIdentityReaderWriter
	entered chan struct{}
}

func (s identityBlockingStore) Observe(ctx context.Context, _ []models.IdentityObservation) ([]models.ProviderIdentity, error) {
	close(s.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestProviderIdentityRunningCancelMatchesNativeJobStatus(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Defaults()[0]
	require.NoError(t, rs.saveDownloadReceipt(booruDownloadReceipt{Source: booru.PublicSource{ID: source.ID, Name: source.Name, Engine: source.Engine, URL: source.URL, Instance: source.Instance()}, Post: booru.Post{ID: "1", Tags: []booru.Tag{{Name: "artist", Category: "artist"}}}}))
	entered := make(chan struct{})
	rs.repository.ProviderIdentity = identityBlockingStore{rs.repository.ProviderIdentity, entered}
	jobs := job.NewManager()
	defer jobs.Stop()
	rs.jobs = jobs
	body := feedRequest(t, rs.IdentityScan, map[string]any{"action": "start"}, 200)
	var state identityScanState
	require.NoError(t, json.Unmarshal(body, &state))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("scan never entered observation")
	}
	feedRequest(t, rs.IdentityScan, map[string]any{"action": "cancel", "jobID": state.JobID}, 200)
	require.Eventually(t, func() bool {
		current := jobs.GetJob(state.JobID)
		return current != nil && current.Status == job.StatusCancelled && current.Error == nil
	}, 5*time.Second, 10*time.Millisecond)
	w := httptest.NewRecorder()
	rs.IdentityScan(w, httptest.NewRequest("GET", "/identities/scan", nil))
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &state))
	require.Equal(t, "cancelled", state.Status)
}

func TestProviderIdentityScanKeepsUnavailableCustomSourceName(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	source := booru.Source{ID: "custom-source", Name: "Custom provider", Engine: "gelbooru", URL: "https://example.com"}
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := rs.repository.ProviderIdentity.Observe(ctx, []models.IdentityObservation{identityObservation(source, "artist", "known_artist", "Artist", "", 1)})
		if err != nil {
			return err
		}
		follow, err := rs.repository.BooruFollow.Create(ctx, models.BooruReviewProvider{Instance: source.Instance(), SourceID: source.ID, Engine: source.Engine, URL: source.URL}, models.BooruFollowDefinition{Name: "My favorites", Kind: "artist", Target: "known_artist"})
		if err != nil {
			return err
		}
		data, err := json.Marshal(followingStoredPost{Post: booru.Post{Tags: []booru.Tag{{Name: "known_artist", Category: "artist"}, {Name: "new_character", Category: "character"}}}})
		if err != nil {
			return err
		}
		_, err = rs.repository.Following.ApplyPage(ctx, *follow, 0, "done", false, []models.FollowingSnapshot{{PostID: "1", Payload: data}})
		return err
	}))
	require.NoError(t, os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte("{"), 0600))
	require.NoError(t, rs.scanIdentityMetadata(context.Background(), func(int, int, int) {}))
	require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.List(ctx, models.IdentityFilter{Limit: 51})
		require.NoError(t, err)
		require.Len(t, rows, 2)
		require.Equal(t, "Custom provider", rows[0].SourceName)
		require.Equal(t, source.ID, rows[1].SourceName, "unknown provider name falls back to the immutable source ID, not the follow label")
		return nil
	}))
}
