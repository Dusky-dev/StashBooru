package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

// Keep downloaded post tags bounded because short-lived media keys can be
// issued for thousands of results on large archive searches.
const booruMaxDownloadTags = 512

func boundedBooruDownloadTags(tags []booru.Tag) []booru.Tag {
	var result []booru.Tag
	seen := make(map[string]bool)
	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" || len(name) > 200 || strings.ContainsAny(name, "\x00\r\n") {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		tag.Name = name
		result = append(result, tag)
		if len(result) >= booruMaxDownloadTags {
			break
		}
	}
	return result
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
