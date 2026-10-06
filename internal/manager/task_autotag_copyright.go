package manager

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/stashapp/stash/internal/autotag"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

// Copyrights are native Image/Video associations. Galleries have no Copyright
// relationship, so this pass deliberately does not synthesize ordinary Tags.
func (j *autoTagJob) findAutoTagCopyrights(ctx context.Context) ([]*models.Copyright, error) {
	if len(j.input.Copyrights) == 0 {
		return nil, nil
	}
	r := j.repository
	var copyrights []*models.Copyright
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		if len(j.input.Copyrights) == 1 && j.input.Copyrights[0] == "*" {
			all := -1
			copyrights, _, err = r.Copyright.Query(ctx, &models.FindFilterType{PerPage: &all})
		} else {
			ids := []int{}
			for _, value := range j.input.Copyrights {
				id, e := strconv.Atoi(value)
				if e != nil || id <= 0 {
					return fmt.Errorf("invalid Copyright ID %q", value)
				}
				ids = append(ids, id)
			}
			slices.Sort(ids)
			ids = slices.Compact(ids)
			copyrights, err = r.Copyright.FindMany(ctx, ids)
			if err == nil && len(copyrights) != len(ids) {
				return fmt.Errorf("one or more selected Copyrights no longer exist")
			}
		}
		return err
	}); err != nil {
		return nil, err
	}
	return copyrights, nil
}

func (j *autoTagJob) autoTagCopyrights(ctx context.Context, progress *job.Progress, copyrights []*models.Copyright) error {
	r := j.repository
	progress.AddTotal(len(copyrights))
	tagger := autotag.Tagger{TxnManager: r.TxnManager, Cache: &j.cache}
	for _, copyright := range copyrights {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.WithDB(ctx, func(ctx context.Context) error {
			if err := tagger.CopyrightScenes(ctx, copyright, j.input.Paths, r.Scene, r.Copyright); err != nil {
				return err
			}
			return tagger.CopyrightImages(ctx, copyright, j.input.Paths, r.Image, r.Copyright)
		}); err != nil {
			return fmt.Errorf("auto-tagging Copyright %q: %w", copyright.Name, err)
		}
		progress.Increment()
	}
	return nil
}
