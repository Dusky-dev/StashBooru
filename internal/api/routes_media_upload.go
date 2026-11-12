package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	managerConfig "github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

// One multipart request streams one file to the configured library directory.
// No fixed size cap: uploads are bounded by available server storage and any
// limits configured by the administrator's reverse proxy.

var mediaUploadExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".avif": true, ".jxl": true, ".bmp": true,
	".tif": true, ".tiff": true, ".heic": true, ".heif": true,
	".mp4": true, ".m4v": true, ".mov": true, ".mkv": true,
	".webm": true, ".avi": true, ".wmv": true, ".mpg": true,
	".mpeg": true, ".flv": true, ".ts": true,
}

func safeMediaUploadName(input string) (string, error) {
	if input == "" || len(input) > 240 || input == "." || input == ".." {
		return "", fmt.Errorf("choose a media file with a valid filename")
	}
	if filepath.Base(input) != input || strings.ContainsAny(input, "/\\:") ||
		strings.HasPrefix(input, ".") || strings.TrimSpace(input) != input {
		return "", fmt.Errorf("filename must not contain paths or hidden files")
	}
	for _, ch := range input {
		if ch < 32 || ch == 127 {
			return "", fmt.Errorf("filename contains an invalid character")
		}
	}
	if !mediaUploadExtensions[strings.ToLower(filepath.Ext(input))] {
		return "", fmt.Errorf("unsupported media extension; choose an image or video")
	}
	return input, nil
}

func mediaUploadSameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		return validBooruWriteOrigin(origin, r.Host)
	}
	return true
}

// The new endpoint follows the same same-origin rule as booru-browser settings.
// Keep it separate from GET routes so cross-origin pages cannot write media
// merely because the public read-only API allows CORS.
func validBooruWriteOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") &&
		strings.EqualFold(u.Host, host) && u.User == nil && u.Path == "" && u.RawQuery == ""
}

func parseMediaUploadIDs(input string, limit int) ([]int, error) {
	if input == "" {
		return nil, nil
	}
	parts := strings.Split(input, ",")
	if len(parts) > limit {
		return nil, fmt.Errorf("too many metadata selections")
	}
	ret := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return nil, fmt.Errorf("invalid metadata ID")
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 || seen[id] {
			return nil, fmt.Errorf("invalid or repeated metadata ID")
		}
		ret = append(ret, id)
		seen[id] = true
	}
	return ret, nil
}

// Exclusive publish prevents replacing any existing library file, including
// the original name in concurrent uploads. Staging files use a scanner-
// ignored extension; the completed contents become visible only at publish.
func publishMediaUpload(stage, dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i <= 999; i++ {
		filename := name
		if i > 0 {
			filename = fmt.Sprintf("%s_%d%s", stem, i, ext)
		}
		target := filepath.Join(dir, filename)
		err := os.Link(stage, target)
		if err == nil {
			return target, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		// Some network shares do not allow hard links. Reserve the destination
		// exclusively and copy the fully staged data instead; never overwrite.
		src, openErr := os.Open(stage)
		if openErr != nil {
			return "", fmt.Errorf("cannot reopen staged upload: %w", openErr)
		}
		dest, createErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(createErr, os.ErrExist) {
			_ = src.Close()
			continue
		}
		if createErr != nil {
			_ = src.Close()
			return "", fmt.Errorf("cannot create uploaded file in this folder: %w", createErr)
		}
		_, copyErr := io.Copy(dest, src)
		if copyErr == nil {
			copyErr = dest.Sync()
		}
		closeErr := dest.Close()
		_ = src.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(target)
			return "", fmt.Errorf("cannot complete upload in the chosen folder")
		}
		return target, nil
	}
	return "", fmt.Errorf("too many files with the same filename")
}

func validateMediaUploadMetadata(ctx context.Context, repo models.Repository, artistIDs, galleryIDs, tagIDs []int) error {
	return txn.WithReadTxn(ctx, repo.TxnManager, func(ctx context.Context) error {
		for _, id := range artistIDs {
			studio, err := repo.Studio.Find(ctx, id)
			if err != nil {
				return err
			}
			if studio == nil {
				return fmt.Errorf("selected artist no longer exists")
			}
		}
		for _, id := range galleryIDs {
			gallery, err := repo.Gallery.Find(ctx, id)
			if err != nil {
				return err
			}
			if gallery == nil || !gallery.IsUserCreated() {
				return fmt.Errorf("select an existing user-created gallery")
			}
		}
		for _, id := range tagIDs {
			tag, err := repo.Tag.Find(ctx, id)
			if err != nil {
				return err
			}
			if tag == nil {
				return fmt.Errorf("selected tag no longer exists")
			}
		}
		return nil
	})
}

// importUploadedMedia invokes Stash's native scanner, then attaches only
// explicitly selected metadata. If metadata fails, the saved media is retained
// and the error explains that the user should retry metadata association.
func importUploadedMedia(ctx context.Context, progress *job.Progress, path string, artistIDs, galleryIDs, tagIDs []int) (booruLibraryResult, error) {
	var result booruLibraryResult
	mgr := manager.GetInstance()
	if err := mgr.ScanNow(ctx, manager.ScanMetadataInput{
		Paths: []string{path},
		ScanMetadataOptions: managerConfig.ScanMetadataOptions{
			ScanGenerateThumbnails: true, ScanGenerateCovers: true,
		},
	}, progress); err != nil {
		return result, fmt.Errorf("file saved at %s, but library scan failed: %w", path, err)
	}
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		file, err := mgr.Repository.File.FindByPath(ctx, path, true)
		if err != nil {
			return err
		}
		if file == nil {
			return fmt.Errorf("the scanner did not register the file; check scan exclusions")
		}
		images, err := mgr.Repository.Image.FindByFileID(ctx, file.Base().ID)
		if err != nil {
			return err
		}
		if len(images) > 0 {
			result.Kind, result.ID = "image", strconv.Itoa(images[0].ID)
			return nil
		}
		scenes, err := mgr.Repository.Scene.FindByFileID(ctx, file.Base().ID)
		if err != nil {
			return err
		}
		if len(scenes) > 0 {
			result.Kind, result.ID = "scene", strconv.Itoa(scenes[0].ID)
			return nil
		}
		return fmt.Errorf("no Image or Video was registered by Stash")
	})
	if err != nil {
		return result, fmt.Errorf("file saved at %s, but import requires attention: %w", path, err)
	}
	result.Path = path
	if len(artistIDs) == 0 && len(galleryIDs) == 0 && len(tagIDs) == 0 {
		return result, nil
	}
	asAdd := func(ids []int) *models.UpdateIDs {
		if len(ids) == 0 {
			return nil
		}
		return &models.UpdateIDs{IDs: ids, Mode: models.RelationshipUpdateModeAdd}
	}
	id, _ := strconv.Atoi(result.ID)
	err = mgr.Repository.WithTxn(ctx, func(ctx context.Context) error {
		if err := validateUploadMetadataInsideTxn(ctx, mgr.Repository, artistIDs, galleryIDs, tagIDs); err != nil {
			return err
		}
		if result.Kind == "image" {
			if len(galleryIDs) > 0 || len(tagIDs) > 0 {
				if _, err := mgr.Repository.Image.UpdatePartial(ctx, id, models.ImagePartial{
					GalleryIDs: asAdd(galleryIDs), TagIDs: asAdd(tagIDs),
				}); err != nil {
					return err
				}
			}
			if len(artistIDs) > 0 {
				return mgr.Repository.ImageArtist.AddImageArtists(ctx, id, artistIDs)
			}
			return nil
		}
		if len(galleryIDs) > 0 || len(tagIDs) > 0 {
			if _, err := mgr.Repository.Scene.UpdatePartial(ctx, id, models.ScenePartial{
				GalleryIDs: asAdd(galleryIDs), TagIDs: asAdd(tagIDs),
			}); err != nil {
				return err
			}
		}
		if len(artistIDs) > 0 {
			return mgr.Repository.SceneArtist.AddSceneArtists(ctx, id, artistIDs)
		}
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("file imported as %s #%s, but metadata could not be applied: %w", result.Kind, result.ID, err)
	}
	return result, nil
}

func validateUploadMetadataInsideTxn(ctx context.Context, repo models.Repository, artistIDs, galleryIDs, tagIDs []int) error {
	for _, id := range artistIDs {
		artist, err := repo.Studio.Find(ctx, id)
		if err != nil {
			return err
		}
		if artist == nil {
			return fmt.Errorf("selected artist no longer exists")
		}
	}
	for _, id := range galleryIDs {
		gallery, err := repo.Gallery.Find(ctx, id)
		if err != nil {
			return err
		}
		if gallery == nil || !gallery.IsUserCreated() {
			return fmt.Errorf("select an existing user-created gallery")
		}
	}
	for _, id := range tagIDs {
		tag, err := repo.Tag.Find(ctx, id)
		if err != nil {
			return err
		}
		if tag == nil {
			return fmt.Errorf("selected tag no longer exists")
		}
	}
	return nil
}

func (rs booruBrowserRoutes) UploadMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !mediaUploadSameOrigin(r) {
		writeBooruError(w, http.StatusForbidden, "origin", "Uploads must come from this StashBooru interface.")
		return
	}
	if !rs.requireMediaUpload(w, "file") {
		return
	}
	parts, err := r.MultipartReader()
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "upload", "Expected a multipart file upload.")
		return
	}
	var destination, uploadedName, stage string
	var artistIDs, galleryIDs, tagIDs []int
	var taggingDefaults, conversionDefaults bool
	defer func() {
		if stage != "" {
			_ = os.Remove(stage)
		}
	}()
	seen := map[string]bool{}
	for {
		part, nextErr := parts.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeBooruError(w, http.StatusBadRequest, "upload", "Malformed or interrupted upload.")
			return
		}
		key := part.FormName()
		if seen[key] || key == "" {
			_ = part.Close()
			writeBooruError(w, http.StatusBadRequest, "upload", "Duplicate or unnamed upload field.")
			return
		}
		seen[key] = true
		if key == "file" {
			var nameErr error
			uploadedName, nameErr = safeMediaUploadName(part.FileName())
			if nameErr != nil {
				_ = part.Close()
				writeBooruError(w, http.StatusBadRequest, "file", nameErr.Error())
				return
			}
			if destination == "" {
				_ = part.Close()
				writeBooruError(w, http.StatusBadRequest, "directory", "Choose a Stash library folder before uploading.")
				return
			}
			staged, createErr := os.CreateTemp(destination, ".stash-upload-*.part")
			if createErr != nil {
				_ = part.Close()
				writeBooruError(w, http.StatusBadRequest, "directory", "Cannot write to this library folder.")
				return
			}
			stage = staged.Name()
			n, copyErr := io.Copy(staged, part)
			if copyErr == nil {
				copyErr = staged.Sync()
			}
			closeErr := staged.Close()
			_ = part.Close()
			if n == 0 || copyErr != nil || closeErr != nil {
				writeBooruError(w, http.StatusBadRequest, "file", "Upload was empty, interrupted, or could not be saved.")
				return
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(part, 4097))
		_ = part.Close()
		if readErr != nil || len(data) > 4096 {
			writeBooruError(w, http.StatusBadRequest, "upload", "Invalid upload field.")
			return
		}
		switch key {
		case "destination":
			destination, err = rs.destination(string(data))
			if err != nil {
				writeBooruError(w, http.StatusBadRequest, "directory", err.Error())
				return
			}
		case "artistIDs":
			artistIDs, err = parseMediaUploadIDs(string(data), 32)
		case "galleryIDs":
			galleryIDs, err = parseMediaUploadIDs(string(data), 32)
		case "tagIDs":
			tagIDs, err = parseMediaUploadIDs(string(data), 64)
		case "taggingDefaults":
			taggingDefaults, err = parseMediaUploadBoolean(string(data))
		case "conversionDefaults":
			conversionDefaults, err = parseMediaUploadBoolean(string(data))
		default:
			writeBooruError(w, http.StatusBadRequest, "upload", "Unexpected upload field.")
			return
		}
		if err != nil {
			writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
			return
		}
	}
	if stage == "" || destination == "" || uploadedName == "" {
		writeBooruError(w, http.StatusBadRequest, "upload", "Select a file and library folder.")
		return
	}
	// Revalidate the configured root after receiving bytes. A path submitted
	// by the browser is never trusted as an arbitrary filesystem destination.
	if checked, err := rs.destination(destination); err != nil || checked != destination {
		writeBooruError(w, http.StatusBadRequest, "directory", "Library folder changed during upload. Retry.")
		return
	}
	if err := validateMediaUploadMetadata(r.Context(), rs.repository, artistIDs, galleryIDs, tagIDs); err != nil {
		writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
		return
	}
	path, err := publishMediaUpload(stage, destination, uploadedName)
	if err != nil {
		writeBooruError(w, http.StatusConflict, "file", err.Error())
		return
	}
	if !fsutil.IsPathInDir(destination, path) {
		_ = os.Remove(path)
		writeBooruError(w, http.StatusBadRequest, "file", "Invalid library target.")
		return
	}
	if rs.jobs == nil {
		writeBooruError(w, http.StatusServiceUnavailable, "import", "Library import service is unavailable; the saved file remains in the selected folder.")
		return
	}
	type importOutcome struct {
		result booruLibraryResult
		err    error
	}
	completed := make(chan importOutcome, 1)
	rs.jobs.Add(r.Context(), "Import uploaded media: "+uploadedName, job.MakeJobExec(
		func(ctx context.Context, progress *job.Progress) error {
			result, importErr := importUploadedMedia(ctx, progress, path, artistIDs, galleryIDs, tagIDs)
			if importErr == nil {
				applyMediaUploadDefaults(ctx, &result, taggingDefaults, conversionDefaults)
			}
			completed <- importOutcome{result: result, err: importErr}
			return importErr
		},
	))
	select {
	case outcome := <-completed:
		if outcome.err != nil {
			// The fully written original remains for a later scan/repair.
			writeBooruError(w, http.StatusInternalServerError, "import", outcome.err.Error())
			return
		}
		writeVisualSimilarityJSON(w, outcome.result)
	case <-r.Context().Done():
		// Browser closed the upload; a fully saved original remains recoverable.
		writeBooruError(w, http.StatusRequestTimeout, "import", "Import cancelled; the saved file is still in the library folder.")
	}
}
