//go:build integration

package api

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

// creatorNameClient answers archive creator names; browsing itself is unused.
type creatorNameClient struct {
	mu      sync.Mutex
	cached  map[string]string
	remote  map[string]string
	lookups []string
}

func (c *creatorNameClient) Search(context.Context, booru.Source, string, int, int) (booru.Result, error) {
	return booru.Result{}, nil
}
func (c *creatorNameClient) FetchMedia(context.Context, booru.Source, booru.MediaRequest, io.Writer) (booru.MediaInfo, error) {
	return booru.MediaInfo{}, nil
}
func (c *creatorNameClient) CachedCreatorName(_ booru.Source, service, id string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name, ok := c.cached[service+":"+id]
	return name, ok
}
func (c *creatorNameClient) LookupCreatorName(_ context.Context, _ booru.Source, service, id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lookups = append(c.lookups, service+":"+id)
	name, ok := c.remote[service+":"+id]
	if !ok {
		return "", errors.New("not found")
	}
	return name, nil
}

func TestArchiveCreatorNamesLabelIdentitiesWithoutChangingConnections(t *testing.T) {
	rs, db, _ := identityTestRoutes(t)
	defer db.Close()
	client := &creatorNameClient{
		cached: map[string]string{"patreon:1": "Known Name"},
		remote: map[string]string{"patreon:2": "Fetched Name"},
	}
	rs.client = client
	source := booru.Source{ID: "paw", Name: "Pawchive", Engine: "pawchive", URL: "https://pawchive.pw"}

	// A connected creator seen before its name was known.
	var connected models.ProviderIdentity
	require.NoError(t, rs.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		rows, err := rs.repository.ProviderIdentity.Observe(ctx, []models.IdentityObservation{
			identityObservation(source, "artist", "creator:patreon:2", "", "", 0),
		})
		if err == nil {
			connected = rows[0]
		}
		return err
	}))
	require.Equal(t, "creator:patreon:2", connected.Label)
	connected = identityTestDecide(t, rs, "create", "artist", []models.IdentityDecision{{ID: connected.ID, Version: connected.Version}}, "Local Artist", 200)[0]

	posts := []booru.Post{
		{Tags: []booru.Tag{{Name: "creator:patreon:1", Category: "artist", Label: "patreon/1"}}},
		{Tags: []booru.Tag{{Name: "creator:patreon:2", Category: "artist", Label: "patreon/2"}}},
		{Tags: []booru.Tag{{Name: "creator:patreon:3", Category: "artist", Label: "patreon/3"}}},
	}
	rs.captureIdentityPosts(context.Background(), source, posts)
	require.Equal(t, "patreon/1", posts[0].Tags[0].Label, "post tags keep their provider label")

	label := func(target string) models.ProviderIdentity {
		var row *models.ProviderIdentity
		require.NoError(t, rs.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
			var err error
			row, err = rs.repository.ProviderIdentity.Lookup(ctx, source.Instance(), "artist", "account", target)
			return err
		}))
		require.NotNil(t, row)
		return *row
	}
	require.Equal(t, "Known Name", label("creator:patreon:1").Label, "a cached name is used at once")
	require.Eventually(t, func() bool {
		return label("creator:patreon:2").Label == "Fetched Name"
	}, 5*time.Second, 20*time.Millisecond, "an unknown name is fetched after the response")
	upgraded := label("creator:patreon:2")
	require.Equal(t, connected.EntityID, upgraded.EntityID, "a name never changes the connection")
	require.Equal(t, connected.Version, upgraded.Version, "a label upgrade is not a decision")
	require.Equal(t, "creator:patreon:3", label("creator:patreon:3").Label, "a failed lookup keeps the key")

	// A later key-only observation does not downgrade the fetched name.
	client.mu.Lock()
	client.remote = map[string]string{}
	client.mu.Unlock()
	rs.captureIdentityPosts(context.Background(), source, posts[1:2])
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, "Fetched Name", label("creator:patreon:2").Label)
}
