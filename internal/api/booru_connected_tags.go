package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

// Some sources (Paheal and other Shimmie sites, sources without tag types)
// send every tag uncategorised. A plain tag the user connected to a
// Character, Artist or Copyright is treated as that kind: it is listed in
// that section while browsing and joins the record on download.

// connectPlainBooruTags re-categorises connected plain tags in place, for
// every post. A target connected as more than one kind is left alone.
func connectPlainBooruTags(ctx context.Context, store models.ProviderIdentityReaderWriter, source booru.Source, tagLists ...[]booru.Tag) error {
	if store == nil {
		return nil
	}
	seen := map[string]bool{}
	var targets []string
	for _, tags := range tagLists {
		for _, tag := range tags {
			if models.IdentityKind(tag.Category) || !models.IdentityText(tag.Name, 1024) || seen[tag.Name] {
				continue
			}
			seen[tag.Name] = true
			targets = append(targets, tag.Name)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	rows, err := store.ConnectedTags(ctx, source.Instance(), targets)
	if err != nil {
		return err
	}
	connected := map[string]*models.ProviderIdentity{}
	ambiguous := map[string]bool{}
	for index := range rows {
		row := &rows[index]
		if existing := connected[row.Target]; existing != nil && existing.Kind != row.Kind {
			ambiguous[row.Target] = true
		}
		connected[row.Target] = row
	}
	for _, tags := range tagLists {
		for index := range tags {
			tag := &tags[index]
			row := connected[tag.Name]
			if row == nil || ambiguous[tag.Name] || models.IdentityKind(tag.Category) {
				continue
			}
			tag.Category = row.Kind
			tag.CanonicalName = row.EntityName
			tag.IdentityID = strconv.FormatInt(row.ID, 10)
		}
	}
	return nil
}
