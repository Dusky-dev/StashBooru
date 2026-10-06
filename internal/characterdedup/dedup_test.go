package characterdedup

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func candidate(id int, name string, copyrights ...int) record {
	p := models.NewPerformer()
	p.ID, p.Name = id, name
	p.Aliases = models.NewRelatedStrings([]string{})
	p.URLs = models.NewRelatedStrings([]string{})
	p.TagIDs = models.NewRelatedIDs([]int{})
	p.StashIDs = models.NewRelatedStashIDs([]models.StashID{})
	r := record{Performer: &p, Fields: map[string]interface{}{}}
	for _, id := range copyrights {
		r.Copyrights = append(r.Copyrights, Character{id, "Series"})
	}
	return r
}

func TestMatching(t *testing.T) {
	for _, tc := range []struct {
		name            string
		records         []record
		groups, sources int
	}{
		{"reversed", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "miku_hatsune", 1)}, 1, 1},
		{"normalized", []record{candidate(1, "Ｈａｔｓｕｎｅ Miku", 1), candidate(2, "Miku, Hatsune", 1)}, 1, 1},
		{"short", []record{candidate(1, "Miku", 1), candidate(2, "Hatsune Miku", 1)}, 1, 1},
		{"different series", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "Miku Hatsune", 2)}, 0, 0},
		{"no context", []record{candidate(1, "Hatsune Miku"), candidate(2, "Miku Hatsune")}, 0, 0},
		{"overlapping sets", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "Miku Hatsune", 1, 2)}, 0, 0},
		{"ambiguous short", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "Other Miku", 1), candidate(3, "Miku", 1)}, 0, 0},
		{"no bridging", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "Miku Hatsune", 1), candidate(3, "Other Miku", 1), candidate(4, "Miku", 1)}, 1, 1},
		{"unrelated short names", []record{candidate(1, "Miku", 1), candidate(2, "Miku", 1)}, 0, 0},
		{"not substring", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "Hatsu", 1)}, 0, 0},
		{"not initials", []record{candidate(1, "Hatsune Miku", 1), candidate(2, "H Miku", 1)}, 0, 0},
		{"not arbitrary reorder", []record{candidate(1, "A B C", 1), candidate(2, "C B A", 1)}, 0, 0},
		{"exact long name", []record{candidate(1, "Alice Mary Smith", 1), candidate(2, "Alice Mary Smith", 1)}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := plan(tc.records)
			require.NoError(t, err)
			require.Len(t, p.Groups, tc.groups)
			if tc.groups > 0 {
				require.Len(t, p.Groups[0].Sources, tc.sources)
				require.GreaterOrEqual(t, len(words(p.Groups[0].Destination.Name)), 2)
			}
		})
	}
}

func TestVariantsStillBlockShortNameAmbiguity(t *testing.T) {
	full, other, short := candidate(1, "Hatsune Miku", 1), candidate(2, "Other Miku", 1), candidate(3, "Miku", 1)
	parent := 4
	other.Performer.ParentID = &parent
	p, err := plan([]record{full, other, short})
	require.NoError(t, err)
	require.Empty(t, p.Groups)
	parent = full.Performer.ID
	p, err = plan([]record{full, other, candidate(4, "Miku Hatsune", 1)})
	require.NoError(t, err)
	require.Empty(t, p.Groups)
}

func TestConflictsAreNotSilentlyDiscarded(t *testing.T) {
	for _, field := range []string{"details", "disambiguation", "artist", "copyright", "portrait", "custom", "stash"} {
		t.Run(field, func(t *testing.T) {
			a, b := candidate(1, "Hatsune Miku", 1), candidate(2, "Miku Hatsune", 1)
			switch field {
			case "details":
				a.Performer.Details = "one"
				b.Performer.Details = "two"
			case "disambiguation":
				a.Performer.Disambiguation = "one"
				b.Performer.Disambiguation = "two"
			case "artist":
				x, y := 1, 2
				a.Performer.DisambiguationStudioID = &x
				b.Performer.DisambiguationStudioID = &y
			case "copyright":
				x := 2
				b.Performer.DisambiguationCopyrightID = &x
			case "portrait":
				a.Image = []byte("a")
				b.Image = []byte("b")
			case "custom":
				a.Fields["role"] = "a"
				b.Fields["role"] = "b"
			case "stash":
				a.Performer.StashIDs = models.NewRelatedStashIDs([]models.StashID{{Endpoint: "site", StashID: "a"}})
				b.Performer.StashIDs = models.NewRelatedStashIDs([]models.StashID{{Endpoint: "site", StashID: "b"}})
			}
			p, err := plan([]record{a, b})
			require.NoError(t, err)
			require.Empty(t, p.Groups)
			require.NotEmpty(t, p.Skipped)
		})
	}
}

func TestSnapshotIncludesPrivateRelationshipContainers(t *testing.T) {
	for _, field := range []string{"alias", "url", "tag", "stash", "image", "custom", "name", "copyright", "new candidate"} {
		t.Run(field, func(t *testing.T) {
			r := candidate(1, "Hatsune Miku", 1)
			p, err := plan([]record{r})
			require.NoError(t, err)
			records := []record{r}
			switch field {
			case "alias":
				r.Performer.Aliases = models.NewRelatedStrings([]string{"new"})
			case "url":
				r.Performer.URLs = models.NewRelatedStrings([]string{"https://example.test"})
			case "tag":
				r.Performer.TagIDs = models.NewRelatedIDs([]int{2})
			case "stash":
				r.Performer.StashIDs = models.NewRelatedStashIDs([]models.StashID{{Endpoint: "site", StashID: "new"}})
			case "image":
				records[0].Image = []byte("changed")
			case "custom":
				r.Fields["new"] = "value"
			case "name":
				r.Performer.Name = "Other Miku"
			case "copyright":
				records[0].Copyrights = []Character{{2, "other"}}
			case "new candidate":
				records = append(records, candidate(2, "Other Miku", 1))
			}
			updated, err := plan(records)
			require.NoError(t, err)
			require.NotEqual(t, p.Fingerprint, updated.Fingerprint)
		})
	}
}
