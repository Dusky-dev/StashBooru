package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stashapp/stash/pkg/mediaconvert"
)

func (s *Manager) MediaConversionStore() mediaconvert.Store {
	return mediaconvert.Store{Root: filepath.Join(s.Config.GetConfigPath(), "media-conversions"), Repository: mediaconvert.ModelRepository{Repository: s.Repository}}
}

func (s *Manager) MediaUpscalingStore() mediaconvert.Store {
	return mediaconvert.Store{Root: filepath.Join(s.Config.GetConfigPath(), "media-upscaling"), Repository: mediaconvert.ModelRepository{Repository: s.Repository}}
}

// migrateLegacyUpscalingRecords moves destructive upscales created before the
// stores were split out of the converter journal. The journal is moved last so
// an interrupted migration can be retried without losing the cached original.
func migrateLegacyUpscalingRecords(conversion, upscaling mediaconvert.Store) error {
	records, err := conversion.History()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(upscaling.Root, 0700); err != nil {
		return err
	}
	for _, record := range records {
		if record.Options.Upscaler == "" {
			continue
		}

		journalSource := filepath.Join(conversion.Root, record.ID+".json")
		journalDestination := filepath.Join(upscaling.Root, record.ID+".json")
		if _, err := os.Lstat(journalDestination); err == nil {
			return fmt.Errorf("upscaling journal %s already exists in both restore stores", record.ID)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		backupSource := filepath.Join(conversion.Root, record.ID+".original")
		backupDestination := filepath.Join(upscaling.Root, record.ID+".original")
		movedBackup := false
		if record.Cached {
			if _, err := os.Lstat(backupSource); err != nil {
				return fmt.Errorf("legacy upscaling restore %s is marked cached but its original is unavailable: %w", record.ID, err)
			}
			if _, err := os.Lstat(backupDestination); err == nil {
				return fmt.Errorf("upscaling restore %s already exists in both restore stores", record.ID)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.Rename(backupSource, backupDestination); err != nil {
				return fmt.Errorf("moving legacy upscaling restore %s: %w", record.ID, err)
			}
			movedBackup = true
		}

		if err := os.Rename(journalSource, journalDestination); err != nil {
			if movedBackup {
				_ = os.Rename(backupDestination, backupSource)
			}
			return fmt.Errorf("moving legacy upscaling journal %s: %w", record.ID, err)
		}
	}
	return nil
}

// Recover before scanning/cleaning so interrupted activations cannot be indexed
// as duplicate entries or mistaken for missing primary files.
func (s *Manager) RecoverMediaConversions(ctx context.Context) error {
	conversion := s.MediaConversionStore()
	if err := conversion.Recover(ctx); err != nil {
		return fmt.Errorf("recover media conversions before scanning/cleaning: %w", err)
	}

	upscaling := s.MediaUpscalingStore()
	if err := migrateLegacyUpscalingRecords(conversion, upscaling); err != nil {
		return fmt.Errorf("separate legacy upscaling restore records: %w", err)
	}
	if err := upscaling.Recover(ctx); err != nil {
		return fmt.Errorf("recover image upscaling before scanning/cleaning: %w", err)
	}
	if err := conversion.Trim(); err != nil {
		return fmt.Errorf("trim media conversion restore cache: %w", err)
	}
	if err := upscaling.Trim(); err != nil {
		return fmt.Errorf("trim image upscaling restore cache: %w", err)
	}
	return nil
}
