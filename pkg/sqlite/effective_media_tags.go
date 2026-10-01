package sqlite

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

// effectiveMediaTagMatches starts with the requested Tags, then finds the
// native media links that imply them. Only metadata ancestry is expanded;
// this does not calculate or materialize every media/Tag pair in the library.
// UNION keeps diamonds, shared profiles and legacy/multi-Artist links unique
// and terminates traversal even if an imported hierarchy contains a cycle.
func effectiveMediaTagMatches(primaryTable, valuesClause string, settings config.AssociationInheritanceSettings) string {
	mediaFK := imageIDColumn
	performerLinks := performersImagesTable
	if primaryTable == sceneTable {
		mediaFK = sceneIDColumn
		performerLinks = performersScenesTable
	}

	tagParents := ""
	if settings.Tags {
		tagParents = `UNION
		SELECT v.root_id, r.child_id FROM tag_values v
		JOIN tags_relations r ON r.parent_id = v.id`
	}
	characterParents := ""
	if settings.Characters {
		characterParents = `UNION
		SELECT v.root_id, p.id FROM character_values v
		JOIN performers p ON p.parent_performer_id = v.id`
	}
	artistParents := ""
	if settings.Artists {
		artistParents = `UNION
		SELECT v.root_id, s.id FROM artist_values v
		JOIN studios s ON s.parent_id = v.id`
	}
	copyrightParents := ""
	if settings.Copyrights {
		copyrightParents = `UNION
		SELECT v.root_id, r.child_id FROM copyright_values v
		JOIN copyright_relations r ON r.parent_id = v.id`
	}

	return fmt.Sprintf(`WITH RECURSIVE
		tag_values(root_id, id) AS (
			SELECT column1, column2 FROM (%s)
			%s
		),
		character_values(root_id, id) AS (
			SELECT v.root_id, p.performer_id FROM tag_values v
			JOIN performers_tags p ON p.tag_id = v.id
			%s
		),
		artist_values(root_id, id) AS (
			SELECT v.root_id, s.studio_id FROM tag_values v
			JOIN studios_tags s ON s.tag_id = v.id
			%s
		),
		copyright_values(root_id, id) AS (
			SELECT v.root_id, c.copyright_id FROM tag_values v
			JOIN copyrights_tags c ON c.tag_id = v.id
			%s
		)
		SELECT j.%s AS media_id, v.root_id FROM %s_tags j
		JOIN tag_values v ON v.id = j.tag_id
		UNION
		SELECT j.%s, v.root_id FROM %s j
		JOIN character_values v ON v.id = j.performer_id
		UNION
		SELECT j.id, v.root_id FROM %s j
		JOIN artist_values v ON v.id = j.studio_id
		UNION
		SELECT j.%s, v.root_id FROM %s_artists j
		JOIN artist_values v ON v.id = j.studio_id
		UNION
		SELECT j.%s, v.root_id FROM %s_copyrights j
		JOIN copyright_values v ON v.id = j.copyright_id`,
		valuesClause, tagParents, characterParents, artistParents, copyrightParents,
		mediaFK, primaryTable, mediaFK, performerLinks, primaryTable,
		mediaFK, primaryTable, mediaFK, primaryTable,
	)
}

// Hierarchical inclusion/exclusion searches use effective memberships, just
// like the detail view and Tag media counts. Exact-set and null criteria retain
// their native direct-link semantics for editing/maintenance compatibility.
func effectiveMediaTagsCriterionHandler(primaryTable string, tags *models.HierarchicalMultiCriterionInput, direct criterionHandlerFunc) criterionHandlerFunc {
	return func(ctx context.Context, f *filterBuilder) {
		if tags == nil {
			return
		}
		switch tags.Modifier {
		case models.CriterionModifierIncludes, models.CriterionModifierIncludesAll, models.CriterionModifierExcludes:
			// Use the shared inheritance settings, including direct profile Tags
			// when that profile's ancestor switch is disabled.
		default:
			direct(ctx, f)
			return
		}

		criterion := tags.CombineExcludes()
		settings := config.GetInstance().GetAssociationInheritanceSettings()
		match := func(values []string, exclude bool) {
			if len(values) == 0 {
				return
			}
			valuesClause, err := getHierarchicalValues(ctx, values, tagTable, "tags_relations", "", "", criterion.Depth)
			if err != nil {
				f.setError(err)
				return
			}
			matches := effectiveMediaTagMatches(primaryTable, valuesClause, settings)
			query := "SELECT media_id FROM (" + matches + ")"
			if !exclude && criterion.Modifier == models.CriterionModifierIncludesAll {
				query += fmt.Sprintf(" GROUP BY media_id HAVING COUNT(DISTINCT root_id) = %d", len(values))
			}
			in := "IN"
			if exclude {
				in = "NOT IN"
			}
			f.addWhere(fmt.Sprintf("%s.id %s (%s)", primaryTable, in, query))
		}
		match(criterion.Value, false)
		match(criterion.Excludes, true)
	}
}
