package autotag

import (
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

// filterAmbiguousPerformerMatches removes performer matches whose canonical
// name belongs to more than one performer. Native auto-tag matches performer
// names case-insensitively, so same-name performers cannot be assigned safely
// from a bare path match.
func filterAmbiguousPerformerMatches(performers []*models.Performer) []*models.Performer {
	owners := make(map[string]map[int]struct{})
	for _, performer := range performers {
		if performer == nil {
			continue
		}

		name := strings.ToLower(performer.Name)
		if owners[name] == nil {
			owners[name] = make(map[int]struct{})
		}
		owners[name][performer.ID] = struct{}{}
	}

	ret := make([]*models.Performer, 0, len(performers))
	for _, performer := range performers {
		if performer == nil {
			continue
		}

		if len(owners[strings.ToLower(performer.Name)]) == 1 {
			ret = append(ret, performer)
		}
	}

	return ret
}
