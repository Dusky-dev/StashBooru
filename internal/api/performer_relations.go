package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func parsePerformerRelationID(value *string, label string) (*int, error) {
	if value == nil {
		return nil, nil
	}

	id, err := strconv.Atoi(*value)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid %s ID %q", label, *value)
	}
	return &id, nil
}

func parsePerformerContextInput(input *models.PerformerDisambiguationContextInput) (*int, *int, error) {
	if input == nil {
		return nil, nil, nil
	}
	if input.CopyrightID != nil && input.ArtistID != nil {
		return nil, nil, fmt.Errorf("a Character disambiguation context can target either a Copyright or an Artist, not both")
	}

	copyrightID, err := parsePerformerRelationID(input.CopyrightID, "Copyright")
	if err != nil {
		return nil, nil, err
	}
	artistID, err := parsePerformerRelationID(input.ArtistID, "Artist")
	if err != nil {
		return nil, nil, err
	}

	return copyrightID, artistID, nil
}

func validatePerformerContextTargets(ctx context.Context, repository models.Repository, copyrightID, artistID *int) error {
	if copyrightID != nil && artistID != nil {
		return fmt.Errorf("a Character disambiguation context can target either a Copyright or an Artist, not both")
	}

	if copyrightID != nil {
		copyright, err := repository.Copyright.Find(ctx, *copyrightID)
		if err != nil {
			return fmt.Errorf("checking Copyright context %d: %w", *copyrightID, err)
		}
		if copyright == nil {
			return fmt.Errorf("Copyright context %d not found", *copyrightID)
		}
	}

	if artistID != nil {
		artist, err := repository.Studio.Find(ctx, *artistID)
		if err != nil {
			return fmt.Errorf("checking Artist context %d: %w", *artistID, err)
		}
		if artist == nil {
			return fmt.Errorf("Artist context %d not found", *artistID)
		}
	}

	return nil
}
