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

var thumbnailCacheName = regexp.MustCompile(`^[a-fA-F0-9]{32}_[0-9]+(?:_q[0-9]+)?\.(?:jpg|webp|jxl|webm|mp4)$`)
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
		return extension == ".jpg" || extension == ".webp" || extension == ".jxl"
	case "clip":
		return extension == ".webm" || extension == ".mp4"
	default:
		return false
	}
}

// removeThumbnailCategory removes only recognized native generated cache files
// in the selected category. WalkDir does not follow symlink directories.
func removeThumbnailCategory(ctx context.Context, root, category string) (int, error) {
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
		if !entry.Type().IsRegular() || !thumbnailCacheMatches(entry.Name(), category) {
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

// Run in the Job Manager: cleanup and generation are consecutive stages of
// one queued job, not a scan and not separate, racy button presses.
func (m *Manager) RebuildExperimentalThumbnailCache(ctx context.Context, progress *job.Progress, category string) error {
	progress.SetTotal(1)
	progress.ExecuteTask("Removing selected generated thumbnail cache", func() {
		logger.Infof("Experimental %s thumbnail cache rebuild requested", category)
	})
	removed, err := removeThumbnailCategory(ctx, m.Paths.Generated.Thumbnails, category)
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
		repository:       m.Repository,
		input:            input,
		ThumbnailCategory: category,
	}).Execute(ctx, progress)
}
