package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type performerNameFinderStub struct {
	matches []*models.Performer
	err     error
}

func (s performerNameFinderStub) FindByNames(context.Context, []string, bool) ([]*models.Performer, error) {
	return s.matches, s.err
}

func TestPerformerNameIsAmbiguous(t *testing.T) {
	t.Parallel()

	lana := &models.Performer{ID: 1, Name: "Lana"}

	t.Run("unique name", func(t *testing.T) {
		ambiguous, err := performerNameIsAmbiguous(context.Background(), performerNameFinderStub{
			matches: []*models.Performer{lana},
		}, lana)
		require.NoError(t, err)
		assert.False(t, ambiguous)
	})

	t.Run("duplicate name", func(t *testing.T) {
		ambiguous, err := performerNameIsAmbiguous(context.Background(), performerNameFinderStub{
			matches: []*models.Performer{lana, {ID: 2, Name: "Lana"}},
		}, lana)
		require.NoError(t, err)
		assert.True(t, ambiguous)
	})

	t.Run("case-only duplicate", func(t *testing.T) {
		ambiguous, err := performerNameIsAmbiguous(context.Background(), performerNameFinderStub{
			matches: []*models.Performer{lana, {ID: 2, Name: "lana"}},
		}, lana)
		require.NoError(t, err)
		assert.True(t, ambiguous)
	})

	t.Run("duplicate query rows for same performer are safe", func(t *testing.T) {
		ambiguous, err := performerNameIsAmbiguous(context.Background(), performerNameFinderStub{
			matches: []*models.Performer{lana, lana},
		}, lana)
		require.NoError(t, err)
		assert.False(t, ambiguous)
	})

	t.Run("finder error", func(t *testing.T) {
		expected := errors.New("lookup failed")
		ambiguous, err := performerNameIsAmbiguous(context.Background(), performerNameFinderStub{err: expected}, lana)
		assert.False(t, ambiguous)
		assert.ErrorIs(t, err, expected)
	})
}
