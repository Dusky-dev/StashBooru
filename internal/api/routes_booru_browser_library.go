package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	managerConfig "github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

type booruLibraryResult struct {
	Kind      string `json:"kind,omitempty"`
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	Notice    string `json:"notice,omitempty"`
	Converted bool   `json:"converted"`
}

func (rs booruBrowserRoutes) destination(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || len(value) > 4096 || rs.libraryRoots == nil {
		return "", fmt.Errorf("choose an existing folder inside a configured Stash library")
	}
	path, err := filepath.EvalSymlinks(filepath.Clean(value))
	if err != nil {
		return "", fmt.Errorf("destination folder does not exist or cannot be accessed")
	}
	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return "", fmt.Errorf("destination must be a directory")
	}
	for _, root := range rs.libraryRoots() {
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err == nil && fsutil.IsPathInDir(resolvedRoot, path) {
			// Keep the configured library namespace for native scan filters, even
			// when the library root is a symlink to a different filesystem path.
			relative, relErr := filepath.Rel(resolvedRoot, path)
			if relErr == nil {
				return filepath.Join(filepath.Clean(root), relative), nil
			}
		}
	}
	return "", fmt.Errorf("destination is outside the configured Stash libraries")
}
func (rs booruBrowserRoutes) Directories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	roots := []string{}
	if rs.libraryRoots != nil {
		roots = rs.libraryRoots()
	}
	if r.URL.Query().Get("path") == "" {
		writeVisualSimilarityJSON(w, map[string]any{"directories": roots, "parent": ""})
		return
	}
	path, err := rs.destination(r.URL.Query().Get("path"))
	if err != nil {
		writeBooruError(w, 400, "directory", err.Error())
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeBooruError(w, 400, "directory", "Cannot list destination folder.")
		return
	}
	dirs := []string{}
	for _, entry := range entries {
		if len(dirs) >= 1000 {
			break
		}
		if entry.IsDir() {
			candidate := filepath.Join(path, entry.Name())
			if valid, err := rs.destination(candidate); err == nil {
				dirs = append(dirs, valid)
			}
		}
	}
	parent := ""
	if p, err := rs.destination(filepath.Dir(path)); err == nil && p != path {
		parent = p
	}
	writeVisualSimilarityJSON(w, map[string]any{"directories": dirs, "parent": parent})
}

func booruImportFile(ctx context.Context, path string, source booru.Source, post booru.Post, convert bool, progress *job.Progress) (result booruLibraryResult, err error) {
	mgr := manager.GetInstance()
	if err = mgr.ScanNow(ctx, manager.ScanMetadataInput{Paths: []string{path}, ScanMetadataOptions: managerConfig.ScanMetadataOptions{ScanGenerateThumbnails: true, ScanGenerateCovers: true}}, progress); err != nil {
		return result, err
	}
	var fileID models.FileID
	var target conversionTarget
	err = txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		f, err := mgr.Repository.File.FindByPath(ctx, path, true)
		if err != nil {
			return err
		}
		if f == nil {
			return fmt.Errorf("file was saved but the native scanner did not register it; check scan exclusions and media extensions")
		}
		fileID = f.Base().ID
		images, err := mgr.Repository.Image.FindByFileID(ctx, fileID)
		if err != nil {
			return err
		}
		if len(images) > 0 {
			target = conversionTarget{Kind: "image", ID: images[0].ID}
			return nil
		}
		scenes, err := mgr.Repository.Scene.FindByFileID(ctx, fileID)
		if err != nil {
			return err
		}
		if len(scenes) > 0 {
			target = conversionTarget{Kind: "scene", ID: scenes[0].ID}
			return nil
		}
		return fmt.Errorf("file is saved but no native Image/Video was registered; check library exclusions")
	})
	if err != nil {
		return result, err
	}
	result.Kind, result.ID, result.Path = target.Kind, strconv.Itoa(target.ID), path
	err = mgr.Repository.WithTxn(ctx, func(ctx context.Context) error {
		urls := &models.UpdateStrings{Values: []string{post.PostURL}, Mode: models.RelationshipUpdateModeAdd}
		if target.Kind == "image" {
			_, err := mgr.Repository.Image.UpdatePartial(ctx, target.ID, models.ImagePartial{URLs: urls})
			return err
		}
		_, err := mgr.Repository.Scene.UpdatePartial(ctx, target.ID, models.ScenePartial{URLs: urls})
		return err
	})
	if err != nil || !convert {
		return result, err
	}
	conversionMutations.Lock()
	defer conversionMutations.Unlock()
	store := conversionStore()
	config, err := store.Config()
	if err != nil {
		return result, err
	}
	client, caps, notice, err := conversionClient(ctx, config.Backend)
	if err != nil {
		return result, err
	}
	options, format, err := resolveConversionOptions(ctx, store, config, fileID, target, conversionRequest{Options: mediaconvert.Options{Format: "auto", Hardware: "auto"}, UseEncodingDefaults: true}, caps)
	if err != nil {
		return result, err
	}
	record, err := store.ConvertWithSavings(ctx, fileID, "booru-"+source.Instance()+"-"+post.ID, client, options, format, config.Savings)
	if err != nil {
		return result, err
	}
	result.Notice = notice
	if record != nil && record.Status == "skipped" {
		result.Notice = record.Error
		return result, nil
	}
	if record == nil || record.Status != "complete" {
		return result, fmt.Errorf("converter did not complete; original remains in Stash")
	}
	result.Converted = true
	if f := record.After.File(); f != nil {
		result.Path = f.Base().Path
	}
	if err = generateConvertedMedia(ctx, []models.FileID{fileID}); err != nil {
		result.Notice = "Converted and imported; thumbnail generation needs retry in Stash."
	}
	return result, nil
}
