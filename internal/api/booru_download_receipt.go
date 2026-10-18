package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

type booruDownloadReceipt struct {
	AssetHash   string              `json:"assetHash,omitempty"`
	Template    string              `json:"template,omitempty"`
	Source      booru.PublicSource  `json:"source"`
	Post        booru.Post          `json:"post"`
	Directory   string              `json:"directory"`
	SavedPath   string              `json:"savedPath"`
	OriginalMD5 string              `json:"originalMD5"`
	OutputMD5   string              `json:"outputMD5"`
	Convert     bool                `json:"convert"`
	Library     *booruLibraryResult `json:"library,omitempty"`
}

func receiptName(instance, id string) string { return instance + "-" + id + ".json" }
func (rs booruBrowserRoutes) saveDownloadReceipt(value booruDownloadReceipt) error {
	dir := filepath.Join(rs.configPath, "booru-download-history")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if value.Post.OriginalURL != "" {
		value.AssetHash = fmt.Sprintf("%x", sha256.Sum256([]byte(value.Post.OriginalURL)))
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeBooruAtomic(dir, receiptName(value.Source.Instance, value.Post.ID), data)
}
func (rs booruBrowserRoutes) reusableDownloadReceipt(destination string, source booru.Source, post booru.Post, convert bool, templates ...string) *booruDownloadReceipt {
	data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-download-history", receiptName(source.Instance(), post.ID)))
	if err != nil || len(data) > 64<<10 {
		return nil
	}
	var receipt booruDownloadReceipt
	if json.Unmarshal(data, &receipt) != nil || receipt.Directory != destination || receipt.Source.Instance != source.Instance() || receipt.Post.ID != post.ID || receipt.Library == nil || receipt.Library.Path == "" || receipt.OutputMD5 == "" || (post.MD5 != "" && post.MD5 != receipt.OriginalMD5) {
		return nil
	}
	template := ""
	if len(templates) > 0 {
		template = templates[0]
	}
	if receipt.Template != template || (receipt.AssetHash != "" && receipt.AssetHash != fmt.Sprintf("%x", sha256.Sum256([]byte(post.OriginalURL)))) || (receipt.AssetHash == "" && post.MD5 == "") {
		return nil
	}
	if dir, err := rs.destination(filepath.Dir(receipt.Library.Path)); err != nil || (dir != destination && !strings.HasPrefix(dir, destination+string(filepath.Separator))) {
		return nil
	}
	stat, err := os.Lstat(receipt.Library.Path)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 256<<20 {
		return nil
	}
	actual, err := mediaconvert.MD5(receipt.Library.Path)
	if err != nil || actual != receipt.OutputMD5 {
		return nil
	}
	return &receipt
}
func publishBooruFile(ctx context.Context, source, directory, instance, id, hash, extension string) (string, error) {
	return publishNamedBooruFile(ctx, source, directory, "booru-"+instance[:12]+"-"+id+"-"+hash+"."+extension, hash)
}
func publishNamedBooruFile(ctx context.Context, source, directory, filename, hash string) (string, error) {
	destination := filepath.Join(directory, filename)
	if stat, err := os.Lstat(destination); err == nil {
		if stat.Mode().IsRegular() {
			if actual, err := mediaconvert.MD5(destination); err == nil && actual == hash {
				return destination, nil
			}
		}
		return "", fmt.Errorf("destination already exists with different bytes; it was not overwritten: %w", os.ErrExist)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	stage, err := os.CreateTemp(directory, ".stash-booru-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	if _, err = io.Copy(stage, in); err != nil {
		return "", err
	}
	if err = stage.Sync(); err != nil {
		return "", err
	}
	if err = stage.Close(); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	// Hard linking publishes atomically without overwriting a raced destination.
	if err = os.Link(stage.Name(), destination); err != nil {
		return "", fmt.Errorf("cannot publish without replacing an existing file: %w", err)
	}
	return destination, nil
}

func (rs booruBrowserRoutes) publishTemplatedBooruFile(ctx context.Context, staged, root string, source booru.Source, post booru.Post, hash, extension, format string) (string, error) {
	if format == "" {
		return publishBooruFile(ctx, staged, root, source.Instance(), post.ID, hash, extension)
	}
	for n := 0; n < 1000; n++ {
		relative, err := booru.RenderFilename(format, source, post, hash, extension, n)
		if err != nil {
			return "", err
		}
		parts := strings.Split(relative, "/")
		directory := root
		for _, part := range parts[:len(parts)-1] {
			directory = filepath.Join(directory, part)
			if err = os.Mkdir(directory, 0755); err != nil && !os.IsExist(err) {
				return "", err
			}
			stat, e := os.Lstat(directory)
			if e != nil || stat.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("template folder must not be a symlink")
			}
			valid, e := rs.destination(directory)
			if e != nil || valid != directory {
				return "", fmt.Errorf("template folder is outside the selected library")
			}
		}
		if valid, e := rs.destination(directory); e != nil || valid != directory {
			return "", fmt.Errorf("template destination changed")
		}
		name := parts[len(parts)-1]
		if n > 0 && !strings.Contains(format, "%num%") {
			name = strings.TrimSuffix(name, "."+extension) + "_" + fmt.Sprint(n) + "." + extension
		}
		saved, err := publishNamedBooruFile(ctx, staged, directory, name, hash)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return saved, err
	}
	return "", fmt.Errorf("too many existing files for this filename template")
}
