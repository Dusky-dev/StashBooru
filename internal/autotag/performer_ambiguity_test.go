package autotag

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestFilterAmbiguousPerformerMatches(t *testing.T) {
	t.Parallel()

	lanaA := &models.Performer{ID: 1, Name: "Lana"}
	lanaB := &models.Performer{ID: 2, Name: "Lana"}
	lanaLower := &models.Performer{ID: 3, Name: "lana"}
	other := &models.Performer{ID: 4, Name: "Other Character"}

	t.Run("keeps unique canonical name", func(t *testing.T) {
		assert.Equal(t, []*models.Performer{lanaA}, filterAmbiguousPerformerMatches([]*models.Performer{lanaA}))
	})

	t.Run("drops duplicate canonical name", func(t *testing.T) {
		assert.Empty(t, filterAmbiguousPerformerMatches([]*models.Performer{lanaA, lanaB}))
	})

	t.Run("treats case-only duplicates as ambiguous", func(t *testing.T) {
		assert.Empty(t, filterAmbiguousPerformerMatches([]*models.Performer{lanaA, lanaLower}))
	})

	t.Run("keeps other distinct matches", func(t *testing.T) {
		assert.Equal(t, []*models.Performer{other}, filterAmbiguousPerformerMatches([]*models.Performer{lanaA, lanaB, other}))
	})

	t.Run("same performer repeated is not ambiguous", func(t *testing.T) {
		assert.Equal(t, []*models.Performer{lanaA, lanaA}, filterAmbiguousPerformerMatches([]*models.Performer{lanaA, lanaA}))
	})
}
