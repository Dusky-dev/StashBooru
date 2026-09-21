package manager

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/stashapp/stash/pkg/mediaconvert"
)

func (s *Manager) MediaConversionStore() mediaconvert.Store {
	return mediaconvert.Store{Root: filepath.Join(s.Config.GetConfigPath(), "media-conversions"), Repository: mediaconvert.ModelRepository{Repository: s.Repository}}
}

// MediaUpscalingStore deliberately uses a different root from media conversions.
// Destructive "replace current image" upscales therefore have an independent
// restore limit, manifest history and original-file cache.
func (s *Manager) MediaUpscalingStore() mediaconvert.Store {
	return mediaconvert.Store{Root: filepath.Join(s.Config.GetConfigPath(), "media-upscaling-restores"), Repository: mediaconvert.ModelRepository{Repository: s.Repository}}
}

// Recover before scanning/cleaning so interrupted activations cannot be indexed
// as duplicate entries or mistaken for missing primary files.
func (s *Manager) RecoverMediaConversions(ctx context.Context) error {
	stores := []struct {
		name  string
		store mediaconvert.Store
	}{
		{name: "media conversions", store: s.MediaConversionStore()},
		{name: "image upscaling", store: s.MediaUpscalingStore()},
	}
	for _, entry := range stores {
		if err := entry.store.Recover(ctx); err != nil {
			return fmt.Errorf("recover %s before scanning/cleaning: %w", entry.name, err)
		}
		if err := entry.store.Trim(); err != nil {
			return fmt.Errorf("trim %s restore cache before scanning/cleaning: %w", entry.name, err)
		}
	}
	return nil
}
