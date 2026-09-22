package autotag

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestSpecificPerformerAutoTagIdentityRule(t *testing.T) {
	t.Parallel()

	performer := &models.Performer{
		ID:             10,
		Name:           "Echidna",
		Disambiguation: "Re:Zero",
		Aliases:        models.NewRelatedStrings([]string{"Knuckles the Echidna"}),
	}

	taggers := getPerformerTaggers(performer, nil)
	if !assert.Len(t, taggers, 2) {
		return
	}

	canonical := taggers[0]
	assert.Equal(t, "Echidna", canonical.Name)
	assert.True(t, canonical.matchesPath("Echidna - ReZero"))
	assert.False(t, canonical.matchesPath("Knuckles the Echidna"))

	alias := taggers[1]
	assert.Equal(t, "Knuckles the Echidna", alias.Name)
	assert.True(t, alias.matchesPath("Knuckles the Echidna"))
}
