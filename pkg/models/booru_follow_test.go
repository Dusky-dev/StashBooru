package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBooruFollowDefinitionValidation(t *testing.T) {
	for _, kind := range []string{"artist", "character", "copyright", "query"} {
		require.NoError(t, (BooruFollowDefinition{Name: "A / Series", Kind: kind, Target: "(artist_a OR artist_b) AND series_s -lowres"}).Validate())
	}
	for _, definition := range []BooruFollowDefinition{
		{Name: "", Kind: "query", Target: "a"},
		{Name: " \t", Kind: "query", Target: "a"},
		{Name: strings.Repeat("a", 121), Kind: "query", Target: "a"},
		{Name: "Name", Kind: "series", Target: "a"},
		{Name: "Name", Kind: "query", Target: "  "},
		{Name: "Name", Kind: "query", Target: strings.Repeat("a", 1025)},
		{Name: "Name", Kind: "query", Target: "a\nb"},
		{Name: "Name", Kind: "query", Target: "a\x00b"},
		{Name: "Name", Kind: "query", Target: "a\u0085b"},
		{Name: "Name", Kind: "query", Target: string([]byte{0xff})},
	} {
		require.ErrorIs(t, definition.Validate(), ErrBooruFollowInvalid)
	}
}
