package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Media identity, not security.
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

type booruDownloadSettings struct {
	JPEGToJXL        bool     `json:"jpegToJXL,omitempty"` // Legacy preference, migrated to converter defaults.
	Convert          bool     `json:"convert"`
	DefaultDirectory string   `json:"defaultDirectory"`
	FilenameTemplate string   `json:"filenameTemplate,omitempty"`
	Roots            []string `json:"roots,omitempty"`
}

var booruDownloadSettingsMu sync.Mutex

func (rs booruBrowserRoutes) downloadSettings() (booruDownloadSettings, error) {
	var value booruDownloadSettings
	data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-downloads.json"))
	if errors.Is(err, os.ErrNotExist) {
		if rs.libraryRoots != nil {
			value.Roots = rs.libraryRoots()
		}
		return value, nil
	}
	if err != nil || len(data) > 8192 {
		return value, fmt.Errorf("cannot read download settings")
	}
	err = json.Unmarshal(data, &value)
	if err == nil {
		err = booru.ValidateFilenameTemplate(value.FilenameTemplate)
	}
	value.Convert = value.Convert || value.JPEGToJXL
	value.JPEGToJXL = false
	if rs.libraryRoots != nil {
		value.Roots = rs.libraryRoots()
	}
	return value, err
}
func (rs booruBrowserRoutes) DownloadSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	value, err := rs.downloadSettings()
	if err != nil {
		writeBooruError(w, 500, "settings", "Cannot load download settings.")
		return
	}
	writeVisualSimilarityJSON(w, value)
}
func (rs booruBrowserRoutes) UpdateDownloadSettings(w http.ResponseWriter, r *http.Request) {
	var value booruDownloadSettings
	if !readBooruReviewJSON(w, r, &value) {
		return
	}
	if err := booru.ValidateFilenameTemplate(value.FilenameTemplate); err != nil {
		writeBooruError(w, 400, "filename", err.Error())
		return
	}
	if value.DefaultDirectory != "" {
		dir, err := rs.destination(value.DefaultDirectory)
		if err != nil {
			writeBooruError(w, 400, "directory", err.Error())
			return
		}
		value.DefaultDirectory = dir
	}
	value.Convert = value.Convert || value.JPEGToJXL
	value.JPEGToJXL = false
	value.Roots = nil
	booruDownloadSettingsMu.Lock()
	defer booruDownloadSettingsMu.Unlock()
	data, err := json.Marshal(value)
	if err != nil {
		writeBooruError(w, 500, "settings", "Cannot encode download settings.")
		return
	}
	if err := writeBooruAtomic(rs.configPath, "booru-downloads.json", data); err != nil {
		writeBooruError(w, 500, "settings", "Cannot save download settings.")
		return
	}
	writeVisualSimilarityJSON(w, value)
}
func writeBooruAtomic(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".booru-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

type booruDownloadRequest struct {
	Key         string `json:"key"`
	Convert     bool   `json:"convert"`
	Cancel      int    `json:"cancel,omitempty"`
	Token       string `json:"token,omitempty"`
	Destination string `json:"destination,omitempty"`
	Computer    bool   `json:"computer,omitempty"`
	ArchiveFile *int   `json:"archiveFile,omitempty"`
}
type booruDownloadState struct {
	JobID       int                 `json:"jobID"`
	Token       string              `json:"token"`
	Status      string              `json:"status"`
	Error       string              `json:"error,omitempty"`
	URL         string              `json:"url,omitempty"`
	Filename    string              `json:"filename,omitempty"`
	OriginalMD5 string              `json:"originalMD5,omitempty"`
	OutputMD5   string              `json:"outputMD5,omitempty"`
	Converted   bool                `json:"converted"`
	Library     *booruLibraryResult `json:"library,omitempty"`
	SavedPath   string              `json:"savedPath,omitempty"`
}

var booruDownloads = struct {
	sync.Mutex
	jobs map[int]*booruDownloadState
}{jobs: map[int]*booruDownloadState{}}

func (rs booruBrowserRoutes) DownloadStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, _ := strconv.Atoi(r.URL.Query().Get("jobID"))
	var native *job.Job
	if rs.jobs != nil {
		native = rs.jobs.GetJob(id)
	}
	booruDownloads.Lock()
	defer booruDownloads.Unlock()
	state := booruDownloads.jobs[id]
	if state != nil && native != nil && native.Status == job.StatusCancelled {
		state.Status, state.Error = "cancelled", "Download cancelled."
	}
	if state == nil || state.Token != r.URL.Query().Get("token") {
		writeBooruError(w, 404, "download", "Download expired or the server restarted. Start it again.")
		return
	}
	writeVisualSimilarityJSON(w, state)
}

func (rs booruBrowserRoutes) StartDownload(w http.ResponseWriter, r *http.Request) {
	var input booruDownloadRequest
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if rs.jobs == nil || rs.downloads == nil {
		writeBooruError(w, 503, "download", "Download service is unavailable.")
		return
	}
	if input.Cancel > 0 {
		booruDownloads.Lock()
		state := booruDownloads.jobs[input.Cancel]
		booruDownloads.Unlock()
		if state == nil || state.Token != input.Token {
			writeBooruError(w, 404, "download", "Download not found.")
			return
		}
		rs.jobs.CancelJob(input.Cancel)
		booruDownloads.Lock()
		if state.Status == "queued" {
			state.Status, state.Error = "cancelled", "Download cancelled."
		}
		booruDownloads.Unlock()
		writeVisualSimilarityJSON(w, map[string]bool{"cancelled": true})
		return
	}
	source, post, ok := rs.mediaSnapshot(input.Key)
	if !ok {
		writeBooruError(w, 410, "expired", "Post expired or its source changed. Refresh the search.")
		return
	}
	if post.OriginalURL == "" || post.Bytes > 128<<20 {
		writeBooruError(w, 400, "download", "Original unavailable or larger than the 128 MiB download limit.")
		return
	}
	if input.ArchiveFile != nil {
		if *input.ArchiveFile < 0 || *input.ArchiveFile >= booruArchiveMaxFiles ||
			(strings.ToLower(post.Format) != "zip" && strings.ToLower(post.Format) != "cbz") {
			writeBooruError(w, 400, "archive", "Invalid archive file selection.")
			return
		}
		if source.ApprovedMediaURL(post.OriginalURL) != post.OriginalURL || post.Bytes > booruArchiveMaxBytes {
			writeBooruError(w, 400, "archive", "Archive source is no longer allowed or exceeds the preview limit.")
			return
		}
	}
	preferences, err := rs.downloadSettings()
	if err != nil {
		writeBooruError(w, 500, "settings", "Cannot read download settings.")
		return
	}
	destination := ""
	if !input.Computer {
		if input.Destination == "" {
			input.Destination = preferences.DefaultDirectory
		}
		var err error
		destination, err = rs.destination(input.Destination)
		if err != nil {
			writeBooruError(w, 400, "directory", err.Error())
			return
		}
		if rs.importFile == nil {
			writeBooruError(w, 503, "import", "Native import is unavailable.")
			return
		}
	} else if input.Convert {
		writeBooruError(w, 400, "convert", "Conversion uses Stash's converter defaults after importing. Choose Download to Stash.")
		return
	}
	state := &booruDownloadState{Status: "queued", Token: mediaconvert.NewID()}
	booruDownloads.Lock()
	if len(booruDownloads.jobs) >= 100 {
		booruDownloads.Unlock()
		writeBooruError(w, 429, "download", "Download queue is full. Completed entries expire after one hour.")
		return
	}
	// Hold the state lock through registration so the dispatcher cannot race JobID.
	id := rs.jobs.Add(r.Context(), "Booru download: "+source.ID+" #"+post.ID, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) (retErr error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		set := func(status string) { booruDownloads.Lock(); state.Status = status; booruDownloads.Unlock() }
		set("downloading")
		defer func() {
			booruDownloads.Lock()
			defer booruDownloads.Unlock()
			switch {
			case ctx.Err() != nil:
				state.Status, state.Error = "cancelled", "Download cancelled or timed out."
			case retErr != nil:
				state.Status, state.Error = "failed", retErr.Error()
			default:
				state.Status = "complete"
			}
		}()
		var archiveBytes []byte
		var selected booruArchiveFile
		if input.ArchiveFile != nil {
			data, err := rs.loadBooruArchive(r.WithContext(ctx), input.Key)
			if err != nil {
				return fmt.Errorf("archive could not be fetched: %w", err)
			}
			listing, _, err := indexBooruArchive(data)
			if err != nil {
				return err
			}
			found := false
			for _, item := range listing.Files {
				if item.Index == *input.ArchiveFile {
					selected, found = item, true
					break
				}
			}
			if !found {
				return fmt.Errorf("selected archive entry is not supported")
			}
			archiveBytes = data
			// A distinct stable receipt and output name for each archive member,
			// rather than overwriting or deduplicating against the parent archive.
			identity := sha256.Sum256([]byte(input.Key + ":" + strconv.Itoa(selected.Index)))
			post.ID = "a" + hex.EncodeToString(identity[:16])
			post.Filename = filepath.Base(selected.Name)
			post.Bytes = int64(selected.Bytes)
			post.MD5 = ""
			post.Format = selected.Format
		}
		if !input.Computer {
			receipt := rs.reusableDownloadReceipt(destination, source, post, input.Convert, preferences.FilenameTemplate)
			if receipt != nil && receipt.Library != nil {
				imported, err := rs.importFile(ctx, receipt.Library.Path, source, post, input.Convert && !receipt.Library.Converted, progress)
				if err != nil {
					return fmt.Errorf("existing download needs native scan retry: %w", err)
				}
				imported.Converted = imported.Converted || receipt.Library.Converted
				receipt.Library = &imported
				receipt.Convert = input.Convert
				if imported.Path != "" {
					receipt.OutputMD5, _ = mediaconvert.MD5(imported.Path)
				}
				if err := rs.saveDownloadReceipt(*receipt); err != nil {
					return fmt.Errorf("import recovered but its receipt could not be updated")
				}
				booruDownloads.Lock()
				state.Library = &imported
				state.Converted = imported.Converted
				state.SavedPath = imported.Path
				state.OriginalMD5, state.OutputMD5 = receipt.OriginalMD5, receipt.OutputMD5
				booruDownloads.Unlock()
				return nil
			}
		}
		dir, err := os.MkdirTemp("", "stash-booru-download-*")
		if err != nil {
			return fmt.Errorf("cannot stage download")
		}
		defer os.RemoveAll(dir)
		original := filepath.Join(dir, "original")
		f, err := os.OpenFile(original, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("cannot stage original")
		}
		var info booru.MediaInfo
		if input.ArchiveFile != nil {
			_, reader, archiveErr := indexBooruArchive(archiveBytes)
			if archiveErr != nil {
				_ = f.Close()
				return archiveErr
			}
			var decoded []byte
			decoded, info.ContentType, err = readBooruArchiveEntry(ctx, reader.File[selected.Index], selected)
			if err == nil {
				_, err = f.Write(decoded)
				if err == nil {
					checksum := md5.Sum(decoded) //nolint:gosec // Content fingerprint.
					info.MD5 = hex.EncodeToString(checksum[:])
					info.Extension = selected.Format
					info.Bytes = int64(len(decoded))
				}
			}
		} else {
			info, err = rs.client.FetchMedia(ctx, source, booru.MediaRequest{URL: post.OriginalURL, Limit: 128 << 20, ExpectedBytes: post.Bytes, ExpectedMD5: post.MD5}, f)
		}
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return fmt.Errorf("cannot save original")
		}
		path := original + "." + info.Extension
		if err = os.Rename(original, path); err != nil {
			return fmt.Errorf("cannot stage original")
		}
		outputMD5 := info.MD5
		converted := false

		if err = ctx.Err(); err != nil {
			return err
		}
		if !input.Computer {
			// Stage in the destination filesystem, then publish without replacing any
			// existing file. A durable receipt permits retry after scan/conversion errors.
			valid, validateErr := rs.destination(destination)
			if validateErr != nil || valid != destination {
				return fmt.Errorf("destination changed while downloading; file was not published")
			}
			saved, saveErr := rs.publishTemplatedBooruFile(ctx, path, destination, source, post, info.MD5, info.Extension, preferences.FilenameTemplate)
			if saveErr != nil {
				return saveErr
			}
			booruDownloads.Lock()
			state.SavedPath = saved
			booruDownloads.Unlock()
			receipt := booruDownloadReceipt{Source: source.Public(), Post: post, OriginalMD5: info.MD5, SavedPath: saved, Convert: input.Convert, Directory: destination, Template: preferences.FilenameTemplate}
			if err := rs.saveDownloadReceipt(receipt); err != nil {
				return fmt.Errorf("file saved, but receipt could not be written; retry native scan for %s", saved)
			}
			set("importing")
			if input.Convert {
				set("importing/converting")
			}
			imported, importErr := rs.importFile(ctx, saved, source, post, input.Convert, progress)
			booruDownloads.Lock()
			state.Library = &imported
			state.Converted = imported.Converted
			state.Filename = filepath.Base(imported.Path)
			booruDownloads.Unlock()
			receipt.Library = &imported
			if imported.Path != "" {
				receipt.OutputMD5, _ = mediaconvert.MD5(imported.Path)
			}
			if saveErr := rs.saveDownloadReceipt(receipt); saveErr != nil {
				return fmt.Errorf("file imported, but receipt update failed; original remains in Stash")
			}
			if importErr != nil {
				return fmt.Errorf("file saved in Stash; import/conversion needs retry: %w", importErr)
			}
			booruDownloads.Lock()
			state.OriginalMD5, state.OutputMD5 = info.MD5, receipt.OutputMD5
			booruDownloads.Unlock()
			return nil
		}
		// Native DownloadStore owns this completed file; partial/staged files are
		// always removed. Nothing is imported or marked in-library by an export.
		exported, err := os.CreateTemp("", "stash-booru-export-*."+info.Extension)
		if err != nil {
			return fmt.Errorf("cannot prepare download")
		}
		exportPath := exported.Name()
		if err = exported.Close(); err == nil {
			err = os.Rename(path, exportPath)
		}
		if err != nil {
			os.Remove(exportPath)
			return fmt.Errorf("cannot prepare download")
		}
		key, err := rs.downloads.RegisterFile(exportPath, info.ContentType, false)
		if err != nil {
			os.Remove(exportPath)
			return fmt.Errorf("cannot register download")
		}
		rs.downloads.ExpireFile(key, time.Hour)
		filename := source.ID + "-" + post.ID + "." + info.Extension
		booruDownloads.Lock()
		state.URL = "downloads/" + key + "/" + filename
		state.Filename, state.OriginalMD5, state.OutputMD5, state.Converted = filename, info.MD5, outputMD5, converted
		booruDownloads.Unlock()
		progress.SetTotal(1)
		progress.Increment()
		return nil
	}))
	state.JobID = id
	booruDownloads.jobs[id] = state
	booruDownloads.Unlock()
	time.AfterFunc(time.Hour, func() {
		rs.jobs.CancelJob(id)
		booruDownloads.Lock()
		defer booruDownloads.Unlock()
		delete(booruDownloads.jobs, id)
	})
	writeVisualSimilarityJSON(w, map[string]any{"jobID": id, "token": state.Token})
}
