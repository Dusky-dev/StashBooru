package autotag

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestPerformerTaggersUseDisambiguationAndExplicitAliases(t *testing.T) {
	t.Parallel()

	performer := &models.Performer{
		ID:             1,
		Name:           "Echidna",
		Disambiguation: "Re:Zero",
		Aliases:        models.NewRelatedStrings([]string{"Knuckles the Echidna"}),
	}

	taggers := getPerformerTaggers(performer, nil)
	if assert.Len(t, taggers, 2) {
		canonical := taggers[0]
		alias := taggers[1]

		assert.Equal(t, "Echidna", canonical.Name)
		assert.True(t, canonical.matchesPath("/anime/Echidna - ReZero/image.jpg"))
		assert.False(t, canonical.matchesPath("/sonic/Knuckles the Echidna/image.jpg"))
		assert.False(t, canonical.matchesPath("/anime/ReZero/Knuckles the Echidna/image.jpg"))

		assert.Equal(t, "Knuckles the Echidna", alias.Name)
		assert.True(t, alias.matchesPath("/sonic/Knuckles the Echidna/image.jpg"))
	}
}

func TestPerformerTaggersTolerateUnloadedAliases(t *testing.T) {
	t.Parallel()

	performer := &models.Performer{
		ID:             1,
		Name:           "Echidna",
		Disambiguation: "Re:Zero",
	}

	assert.NotPanics(t, func() {
		taggers := getPerformerTaggers(performer, nil)
		if assert.Len(t, taggers, 1) {
			assert.Equal(t, "Echidna", taggers[0].Name)
			assert.True(t, taggers[0].matchesPath("/anime/Echidna - ReZero/image.jpg"))
		}
	})
}
