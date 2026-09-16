package manager

import (
	"context"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

type performerNameFinder interface {
	FindByNames(ctx context.Context, names []string, nocase bool) ([]*models.Performer, error)
}

// performerNameIsAmbiguous reports whether a performer's canonical name is
// owned by more than one performer. Native performer auto-tag matches names
// case-insensitively, so a bare same-name match cannot identify either record
// safely.
func performerNameIsAmbiguous(ctx context.Context, finder performerNameFinder, performer *models.Performer) (bool, error) {
	matches, err := finder.FindByNames(ctx, []string{performer.Name}, true)
	if err != nil {
		return false, err
	}

	owners := make(map[int]struct{})
	for _, match := range matches {
		if match == nil || !strings.EqualFold(match.Name, performer.Name) {
			continue
		}

		owners[match.ID] = struct{}{}
		if len(owners) > 1 {
			return true, nil
		}
	}

	return false, nil
}
