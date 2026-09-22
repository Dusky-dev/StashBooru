package manager

import (
	"context"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

type performerNameFinder interface {
	FindByNames(ctx context.Context, names []string, nocase bool) ([]*models.Performer, error)
}

// performerNameIsAmbiguous reports whether a performer's canonical auto-tag
// identity is owned by more than one performer. A performer with a
// disambiguation is identified by name + disambiguation. A performer without
// one still has an ambiguous bare name if any other same-name performer exists.
func performerNameIsAmbiguous(ctx context.Context, finder performerNameFinder, performer *models.Performer) (bool, error) {
	matches, err := finder.FindByNames(ctx, []string{performer.Name}, true)
	if err != nil {
		return false, err
	}

	targetDisambiguation := strings.TrimSpace(performer.Disambiguation)
	owners := make(map[int]struct{})
	for _, match := range matches {
		if match == nil || !strings.EqualFold(match.Name, performer.Name) {
			continue
		}

		if targetDisambiguation != "" && !strings.EqualFold(strings.TrimSpace(match.Disambiguation), targetDisambiguation) {
			continue
		}

		owners[match.ID] = struct{}{}
		if len(owners) > 1 {
			return true, nil
		}
	}

	return false, nil
}
