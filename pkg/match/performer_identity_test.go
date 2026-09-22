package match

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type performerIdentityReaderStub struct {
	canonical []*models.Performer
	aliases   []*models.Performer
	aliasMap  map[int][]string
}

func (s performerIdentityReaderStub) Query(context.Context, *models.PerformerFilterType, *models.FindFilterType) ([]*models.Performer, int, error) {
	return nil, 0, nil
}

func (s performerIdentityReaderStub) QueryCount(context.Context, *models.PerformerFilterType, *models.FindFilterType) (int, error) {
	return 0, nil
}

func (s performerIdentityReaderStub) QueryForAutoTag(context.Context, []string) ([]*models.Performer, error) {
	return s.canonical, nil
}

func (s performerIdentityReaderStub) QueryAliasesForAutoTag(context.Context, []string) ([]*models.Performer, error) {
	return s.aliases, nil
}

func (s performerIdentityReaderStub) GetAliases(_ context.Context, id int) ([]string, error) {
	return s.aliasMap[id], nil
}

func TestPathMatchesIdentityPart(t *testing.T) {
	t.Parallel()

	assert.True(t, PathMatchesIdentityPart("/anime/Echidna - Re Zero/image.jpg", "Re:Zero"))
	assert.True(t, PathMatchesIdentityPart("/anime/Echidna_ReZero/image.jpg", "Re:Zero"))
	assert.False(t, PathMatchesIdentityPart("/sonic/Knuckles the Echidna/image.jpg", "Re:Zero"))
}

func TestPerformerCanonicalMatchesPath(t *testing.T) {
	t.Parallel()

	echidna := &models.Performer{ID: 1, Name: "Echidna", Disambiguation: "Re:Zero"}
	bare := &models.Performer{ID: 2, Name: "Lana"}

	assert.True(t, PerformerCanonicalMatchesPath(echidna, "/anime/Echidna - ReZero/image.jpg", false))
	assert.False(t, PerformerCanonicalMatchesPath(echidna, "/sonic/Knuckles the Echidna/image.jpg", false))
	assert.True(t, PerformerCanonicalMatchesPath(bare, "/media/Lana/image.jpg", false))
	assert.False(t, PerformerCanonicalMatchesPath(bare, "/media/Lana/image.jpg", true))
}

func TestPathToPerformersIdentityAware(t *testing.T) {
	t.Parallel()

	echidnaReZero := &models.Performer{ID: 1, Name: "Echidna", Disambiguation: "Re:Zero"}
	echidnaSonic := &models.Performer{ID: 2, Name: "Echidna", Disambiguation: "Sonic"}
	knuckles := &models.Performer{ID: 3, Name: "Knuckles the Echidna"}

	reader := performerIdentityReaderStub{
		canonical: []*models.Performer{echidnaReZero, echidnaSonic, knuckles},
		aliasMap:  map[int][]string{},
	}

	matches, err := PathToPerformersIdentityAware(context.Background(), "/anime/Echidna - ReZero/image.jpg", reader, &Cache{}, true)
	require.NoError(t, err)
	assert.Equal(t, []*models.Performer{echidnaReZero}, matches)

	matches, err = PathToPerformersIdentityAware(context.Background(), "/sonic/Knuckles the Echidna/image.jpg", reader, &Cache{}, true)
	require.NoError(t, err)
	assert.Equal(t, []*models.Performer{knuckles}, matches)

	reader.aliases = []*models.Performer{echidnaReZero}
	reader.aliasMap[echidnaReZero.ID] = []string{"Knuckles the Echidna"}
	matches, err = PathToPerformersIdentityAware(context.Background(), "/sonic/Knuckles the Echidna/image.jpg", reader, &Cache{}, true)
	require.NoError(t, err)
	assert.Equal(t, []*models.Performer{echidnaReZero, knuckles}, matches)
}
