package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

const maxTaggingSourceURLs = 4

// validateTaggingSourceURLs accepts the few http(s) post links that a tagging
// review adds to the media, such as the Booru post its MD5 matched.
func validateTaggingSourceURLs(values []string) ([]string, error) {
	if len(values) > maxTaggingSourceURLs {
		return nil, fmt.Errorf("cannot add more than %d post links at once", maxTaggingSourceURLs)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		parsed, err := url.Parse(value)
		if err != nil || len(value) > 2048 || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("invalid post link %q", value)
		}
		result = append(result, value)
	}
	return result, nil
}

// missingTaggingURLs returns the links not already on the media, in order.
func missingTaggingURLs(existing, add []string) []string {
	have := make(map[string]bool, len(existing)+len(add))
	for _, value := range existing {
		have[strings.TrimRight(strings.TrimSpace(value), "/")] = true
	}
	var missing []string
	for _, value := range add {
		key := strings.TrimRight(value, "/")
		if have[key] {
			continue
		}
		have[key] = true
		missing = append(missing, value)
	}
	return missing
}

// addImageTaggingURLs adds post links to an Image, add-only; it returns the
// links that were added.
func addImageTaggingURLs(ctx context.Context, imageID int, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	repository := manager.GetInstance().Repository
	var added []string
	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		image, err := repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("image %d not found", imageID)
		}
		if err := image.LoadURLs(ctx, repository.Image); err != nil {
			return err
		}
		added = missingTaggingURLs(image.URLs.List(), urls)
		if len(added) == 0 {
			return nil
		}
		partial := models.NewImagePartial()
		partial.URLs = &models.UpdateStrings{Values: added, Mode: models.RelationshipUpdateModeAdd}
		_, err = repository.Image.UpdatePartial(ctx, imageID, partial)
		return err
	})
	return added, err
}

// addSceneTaggingURLs adds post links to a Video, add-only; it returns the
// links that were added.
func addSceneTaggingURLs(ctx context.Context, sceneID int, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	repository := manager.GetInstance().Repository
	var added []string
	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		scene, err := repository.Scene.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if scene == nil {
			return fmt.Errorf("video %d not found", sceneID)
		}
		if err := scene.LoadURLs(ctx, repository.Scene); err != nil {
			return err
		}
		added = missingTaggingURLs(scene.URLs.List(), urls)
		if len(added) == 0 {
			return nil
		}
		partial := models.NewScenePartial()
		partial.URLs = &models.UpdateStrings{Values: added, Mode: models.RelationshipUpdateModeAdd}
		_, err = repository.Scene.UpdatePartial(ctx, sceneID, partial)
		return err
	})
	return added, err
}
