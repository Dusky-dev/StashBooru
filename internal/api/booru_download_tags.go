package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

// Keep downloaded post tags bounded because short-lived media keys can be
// issued for thousands of results on large archive searches.
const booruMaxDownloadTags = 512

func boundedBooruDownloadTags(tags []booru.Tag) []booru.Tag {
	var result []booru.Tag
	seen := make(map[string]int)
	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" || len(name) > 200 || strings.ContainsAny(name, "\x00\r\n") {
			continue
		}
		key := strings.ToLower(name)
		if index, ok := seen[key]; ok {
			// Flat post APIs can list an unknown/general tag before the same
			// name appears in a typed provider field. Keep the best category.
			if booruDownloadCategoryPriority(tag.Category) > booruDownloadCategoryPriority(result[index].Category) {
				result[index].Category = tag.Category
				if tag.Label != "" {
					result[index].Label = tag.Label
				}
			}
			continue
		}
		if len(result) >= booruMaxDownloadTags {
			// Still inspect later duplicates for stronger category metadata.
			continue
		}
		seen[key] = len(result)
		tag.Name = name
		result = append(result, tag)
	}
	return result
}

// Explicit provider types must win over flat/unknown tag lists.
func booruDownloadCategoryPriority(category string) int {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "artist", "character", "copyright":
		return 2
	case "general", "meta", "species", "lore", "contributor", "circle":
		return 1
	default:
		return 0
	}
}

// Stash keeps Character (Performer), Artist (Studio), and Copyright as
// first-class entities, not as subcategories of ordinary Tags.
func splitBooruImportTags(sourceTags []booru.Tag) ([]booru.Tag, []camietagger.Tag) {
	var ordinary []booru.Tag
	var categorized []camietagger.Tag
	for _, item := range boundedBooruDownloadTags(sourceTags) {
		category := strings.ToLower(strings.TrimSpace(item.Category))
		switch category {
		case "artist", "character", "copyright":
			categorized = append(categorized, camietagger.Tag{
				Name: item.Name, RawName: item.Name, Category: category,
				Score: 1, Source: "booru:download",
			})
		default:
			// Unknown tags are not guessed to be artists or characters.
			// Search/live categorization and exact CSV types run before the
			// server snapshot is issued; unresolved names remain Tags.
			ordinary = append(ordinary, item)
		}
	}
	return ordinary, categorized
}

// Called inside the native Stash write transaction. Reuse the same
// category-aware resolver used by Image Tagging for native entity names,
// aliases and Character/Copyright disambiguation.
func ensureBooruImportMetadata(ctx context.Context, repository models.Repository, sourceTags []booru.Tag) (taggingResolvedEntities, error) {
	ordinary, categorized := splitBooruImportTags(sourceTags)
	tagIDs, err := ensureBooruImportTagIDs(ctx, repository.Tag, ordinary)
	if err != nil {
		return taggingResolvedEntities{}, err
	}
	resolved := taggingResolvedEntities{}
	if len(categorized) > 0 {
		resolved, err = resolveTaggingEntities(ctx, repository, categorized)
		if err != nil {
			return taggingResolvedEntities{}, err
		}
	}
	resolved.TagIDs = append(resolved.TagIDs, tagIDs...)
	return resolved, nil
}

type booruImportTagStore interface {
	FindByName(context.Context, string, bool) (*models.Tag, error)
	Create(context.Context, *models.CreateTagInput) error
}

// Must be called inside the native repository write transaction. Reuse
// existing case-insensitive names and add relationships without replacing any
// tags that the image or scene already has. Retries are idempotent.
func ensureBooruImportTagIDs(ctx context.Context, tags booruImportTagStore, sourceTags []booru.Tag) ([]int, error) {
	names := boundedBooruDownloadTags(sourceTags)
	ids := make([]int, 0, len(names))
	for _, item := range names {
		found, err := tags.FindByName(ctx, item.Name, true)
		if err != nil {
			return nil, fmt.Errorf("looking up downloaded tag: %w", err)
		}
		if found == nil {
			created := models.NewTag()
			created.Name = item.Name
			if err := tags.Create(ctx, &models.CreateTagInput{Tag: &created}); err != nil {
				return nil, fmt.Errorf("creating downloaded tag: %w", err)
			}
			found = &created
		}
		ids = append(ids, found.ID)
	}
	return ids, nil
}
