package manager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
)

var thumbnailCacheName = regexp.MustCompile(`^[a-fA-F0-9]{32}_[0-9]+(?:(?:_exp)?_q[0-9]+|_exp_aq[0-9]+_m[0-9]+_b[0-9]+)?\.(?:jpg|webp|jxl|avif|webm|mp4)$`)
var legacyImageCacheName = regexp.MustCompile(`^[a-fA-F0-9]{32}_[0-9]+\.jpg$`)
var legacyClipCacheName = regexp.MustCompile(`^[a-fA-F0-9]{32}_[0-9]+\.webm$`)
var animationCacheName = regexp.MustCompile(`^[a-fA-F0-9]{32}_[0-9]+_anim_q[0-9]+\.webp$`)

// Cache files are shared by hash prefix but not by media type. Never remove
// the generated directory itself: other thumbnails and originals are untouched.
func thumbnailCacheMatches(name, category string) bool {
	if category == "animated" {
		return animationCacheName.MatchString(name)
	}
	if !thumbnailCacheName.MatchString(name) {
		return false
	}
	extension := strings.ToLower(filepath.Ext(name))
	switch category {
	case "image":
		return extension == ".jpg" || extension == ".webp" || extension == ".jxl" || extension == ".avif"
	case "clip":
		return extension == ".webm" || extension == ".mp4"
	default:
		return false
	}
}

// native thumbnails predate the experiment and must be retained verbatim
// so switching the experiment off can instantly bring the old cache back.
func legacyThumbnailCacheMatches(name, category string) bool {
	switch category {
	case "image":
		return legacyImageCacheName.MatchString(name)
	case "clip":
		return legacyClipCacheName.MatchString(name)
	default:
		return false
	}
}

// WalkDir does not follow symlink directories. Only generated, checksum-keyed
// cache paths are eligible for deletion, never original media or unknown files.
func removeMatchingThumbnailCache(ctx context.Context, root, category string, matches func(name, category string) bool) (int, error) {
	if category != "image" && category != "clip" && category != "animated" {
		return 0, fmt.Errorf("unrecognized thumbnail cache category")
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("thumbnail cache root must be a real directory")
	}
	count := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !matches(entry.Name(), category) {
			return nil
		}
		// Ensure a checksum cache filename is under the expected nested hash directories.
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 3 || len(parts[0]) != 2 || len(parts[1]) != 2 {
			return nil
		}
		checksum := strings.SplitN(entry.Name(), "_", 2)[0]
		if !strings.EqualFold(parts[0]+parts[1], checksum[:4]) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("deleting generated %s cache %s: %w", category, path, err)
		}
		count++
		return nil
	})
	return count, err
}

// removeThumbnailCategory remains the broad category cleaner, including
// native and experimental variants.
func removeThumbnailCategory(ctx context.Context, root, category string) (int, error) {
	return removeMatchingThumbnailCache(ctx, root, category, thumbnailCacheMatches)
}

func removeThumbnailCategoryPreservingOld(ctx context.Context, root, category string) (int, error) {
	return removeMatchingThumbnailCache(ctx, root, category, func(name, category string) bool {
		return thumbnailCacheMatches(name, category) && !legacyThumbnailCacheMatches(name, category)
	})
}

func RemoveOldThumbnailCategory(ctx context.Context, root, category string) (int, error) {
	if category != "image" && category != "clip" {
		return 0, fmt.Errorf("only still-image and clip preview caches have native thumbnails to remove")
	}
	return removeMatchingThumbnailCache(ctx, root, category, legacyThumbnailCacheMatches)
}

// Run in the Job Manager: cleanup and generation are consecutive stages of
// one queued job, not a scan and not separate, racy button presses.
func (s *Manager) RebuildExperimentalThumbnailCache(ctx context.Context, progress *job.Progress, category string, keepOld bool) error {
	progress.SetTotal(1)
	progress.ExecuteTask("Removing selected generated thumbnail cache", func() {
		logger.Infof("Experimental %s thumbnail cache rebuild requested", category)
	})
	var removed int
	var err error
	if keepOld {
		removed, err = removeThumbnailCategoryPreservingOld(ctx, s.Paths.Generated.Thumbnails, category)
	} else {
		removed, err = removeThumbnailCategory(ctx, s.Paths.Generated.Thumbnails, category)
	}
	if err != nil {
		return err // never generate after a partial/failed cleanup
	}
	logger.Infof("Removed %d generated %s thumbnail cache files", removed, category)
	if err := ctx.Err(); err != nil {
		return err
	}
	input := GenerateMetadataInput{}
	if category == "clip" {
		input.ClipPreviews = true
	} else {
		input.ImageThumbnails = true
	}
	return (&GenerateJob{
		repository:        s.Repository,
		input:             input,
		ThumbnailCategory: category,
	}).Execute(ctx, progress)
}
