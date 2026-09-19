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

// Recover before scanning/cleaning so interrupted activations cannot be indexed
// as duplicate entries or mistaken for missing primary files.
func (s *Manager) RecoverMediaConversions(ctx context.Context) error {
	store := s.MediaConversionStore()
	if err := store.Recover(ctx); err != nil {
		return fmt.Errorf("recover media conversions before scanning/cleaning: %w", err)
	}
	return store.Trim()
}
