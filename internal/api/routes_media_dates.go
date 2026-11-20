package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

// Tasks → Fill missing dates: give Images and Videos without a Date one
// from the photo's EXIF capture date, then from the site's post date in
// saved Booru download records. Started by the owner; can be cancelled.

type mediaDateRun struct {
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Checked    int    `json:"checked"`
	FromFile   int    `json:"fromFile"`
	FromPosted int    `json:"fromPosted"`
	Failed     int    `json:"failed"`
	Cancelled  bool   `json:"cancelled,omitempty"`
	Note       string `json:"note,omitempty"`
}

var mediaDates = struct {
	sync.Mutex
	running bool
	jobID   int
	last    *mediaDateRun
}{}

func (rs booruBrowserRoutes) MediaDates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		var input struct {
			Action string `json:"action"`
		}
		if !readBooruReviewJSON(w, r, &input) {
			return
		}
		if input.Action != "run" || rs.jobs == nil {
			writeBooruError(w, 400, "media-dates", "Unknown action.")
			return
		}
		mediaDates.Lock()
		if !mediaDates.running {
			mediaDates.running = true
			run := &mediaDateRun{StartedAt: time.Now().UTC().Format(time.RFC3339)}
			mediaDates.last = run
			mediaDates.jobID = rs.jobs.Add(context.Background(), "Fill missing Image and Video dates", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error {
				err := rs.fillMediaDates(ctx, p, run)
				mediaDates.Lock()
				defer mediaDates.Unlock()
				mediaDates.running = false
				run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
				if ctx.Err() != nil {
					run.Cancelled = true
					return nil
				}
				if err != nil {
					run.Note = err.Error()
				}
				return err
			}))
		}
		mediaDates.Unlock()
	}
	mediaDates.Lock()
	defer mediaDates.Unlock()
	response := map[string]any{"running": mediaDates.running}
	if mediaDates.last != nil {
		copied := *mediaDates.last
		response["lastRun"] = copied
	}
	writeVisualSimilarityJSON(w, response)
}

func (rs booruBrowserRoutes) fillMediaDates(ctx context.Context, p *job.Progress, run *mediaDateRun) error {
	if p != nil {
		p.Indefinite()
	}
	count := func(from string, err error) {
		mediaDates.Lock()
		defer mediaDates.Unlock()
		run.Checked++
		switch {
		case err != nil:
			run.Failed++
		case from == "file":
			run.FromFile++
		case from == "posted":
			run.FromPosted++
		}
	}
	// 1. Images: EXIF capture date.
	last := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var ids []int
		err := rs.repository.WithReadTxn(ctx, func(ctx context.Context) error {
			perPage, sort, direction := 200, "id", models.SortDirectionEnumAsc
			result, err := rs.repository.Image.Query(ctx, models.ImageQueryOptions{
				QueryOptions: models.QueryOptions{FindFilter: &models.FindFilterType{PerPage: &perPage, Sort: &sort, Direction: &direction}},
				ImageFilter: &models.ImageFilterType{
					Date: &models.DateCriterionInput{Modifier: models.CriterionModifierIsNull},
					ID:   &models.IntCriterionInput{Value: last, Modifier: models.CriterionModifierGreaterThan},
				},
			})
			if result != nil {
				ids = result.IDs
			}
			return err
		})
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			last = max(last, id)
			var from string
			err := rs.repository.WithTxn(ctx, func(ctx context.Context) error {
				item, err := rs.repository.Image.Find(ctx, id)
				if err != nil || item == nil {
					return err
				}
				if err := item.LoadPrimaryFile(ctx, rs.repository.File); err != nil || item.Files.Primary() == nil {
					return err
				}
				file := item.Files.Primary().Base()
				if file.ZipFileID != nil {
					return nil // inside an archive: no direct file to read
				}
				from, err = fillMissingMediaDate(ctx, rs.repository, "image", id, file.Path, "")
				return err
			})
			count(from, err)
		}
	}
	// 2. Images and Videos from Booru downloads: the site's post date.
	return rs.eachDownloadReceipt(ctx, func(receipt booruDownloadReceipt) {
		library := receipt.Library
		if library == nil || (library.Kind != "image" && library.Kind != "scene") || receipt.Post.CreatedAt == "" {
			return
		}
		id, err := strconv.Atoi(library.ID)
		if err != nil {
			return
		}
		var from string
		err = rs.repository.WithTxn(ctx, func(ctx context.Context) error {
			// The record names the item by ID; only use it while the item
			// still carries the post's URL (an ID can be reused).
			urls, err := rs.mediaURLs(ctx, library.Kind, id)
			if err != nil || !slices.Contains(urls, receipt.Post.PostURL) {
				return err
			}
			from, err = fillMissingMediaDate(ctx, rs.repository, library.Kind, id, "", receipt.Post.CreatedAt)
			return err
		})
		count(from, err)
	})
}

func (rs booruBrowserRoutes) mediaURLs(ctx context.Context, kind string, id int) ([]string, error) {
	if kind == "image" {
		item, err := rs.repository.Image.Find(ctx, id)
		if err != nil || item == nil {
			return nil, err
		}
		if err := item.LoadURLs(ctx, rs.repository.Image); err != nil {
			return nil, err
		}
		return item.URLs.List(), nil
	}
	item, err := rs.repository.Scene.Find(ctx, id)
	if err != nil || item == nil {
		return nil, err
	}
	if err := item.LoadURLs(ctx, rs.repository.Scene); err != nil {
		return nil, err
	}
	return item.URLs.List(), nil
}

// eachDownloadReceipt visits saved download records (the same records the
// Identities scan reads), stopping when ctx is cancelled.
func (rs booruBrowserRoutes) eachDownloadReceipt(ctx context.Context, visit func(booruDownloadReceipt)) error {
	directory := filepath.Join(rs.configPath, "booru-download-history")
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("saved download records are unavailable")
	}
	dir, err := os.Open(directory)
	if err != nil {
		return errors.New("saved download records are unavailable")
	}
	defer dir.Close()
	for entries := 0; ; {
		if err := ctx.Err(); err != nil {
			return err
		}
		files, err := dir.ReadDir(50)
		if err != nil && !errors.Is(err, io.EOF) {
			return errors.New("cannot list saved download records")
		}
		for _, file := range files {
			if entries++; entries > 100000 {
				return errors.New("more than 100,000 download records; the rest were not read")
			}
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			if receipt, ok := readIdentityReceipt(filepath.Join(directory, file.Name())); ok {
				visit(receipt)
			}
		}
		if errors.Is(err, io.EOF) || len(files) == 0 {
			return nil
		}
	}
}
