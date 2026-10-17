package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
)

type booruDownloadSettings struct {
	JPEGToJXL bool `json:"jpegToJXL"`
}

var booruDownloadSettingsMu sync.Mutex

func (rs booruBrowserRoutes) downloadSettings() (booruDownloadSettings, error) {
	var value booruDownloadSettings
	data, err := os.ReadFile(filepath.Join(rs.configPath, "booru-downloads.json"))
	if errors.Is(err, os.ErrNotExist) {
		return value, nil
	}
	if err != nil || len(data) > 1024 {
		return value, fmt.Errorf("cannot read download settings")
	}
	err = json.Unmarshal(data, &value)
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
	booruDownloadSettingsMu.Lock()
	defer booruDownloadSettingsMu.Unlock()
	data, _ := json.Marshal(value)
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
	Key     string `json:"key"`
	Convert bool   `json:"convert"`
	Cancel  int    `json:"cancel,omitempty"`
}
type booruDownloadState struct {
	JobID       int    `json:"jobID"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	URL         string `json:"url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	OriginalMD5 string `json:"originalMD5,omitempty"`
	OutputMD5   string `json:"outputMD5,omitempty"`
	Converted   bool   `json:"converted"`
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
	if state == nil {
		writeBooruError(w, 404, "download", "Download expired or the server restarted. Start it again.")
		return
	}
	writeVisualSimilarityJSON(w, state)
}
func booruDownloadConverter(ctx context.Context) (mediaconvert.Client, mediaconvert.Capabilities, string, error) {
	return conversionClient(ctx, "")
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
		if state == nil {
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
	state := &booruDownloadState{Status: "queued"}
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
		info, err := rs.client.FetchMedia(ctx, source, booru.MediaRequest{URL: post.OriginalURL, Limit: 128 << 20, ExpectedBytes: post.Bytes, ExpectedMD5: post.MD5}, f)
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
		if input.Convert && info.ContentType == "image/jpeg" {
			if rs.converter == nil {
				return fmt.Errorf("converter is unavailable")
			}
			set("converting")
			client, caps, _, clientErr := rs.converter(ctx)
			if clientErr != nil {
				return fmt.Errorf("converter unavailable; check Media converter settings")
			}
			if _, err = conversionOutputFormat(caps, "jxl", "image"); err != nil {
				return fmt.Errorf("JPEG XL unavailable on the configured converter")
			}
			output := filepath.Join(dir, "converted.jxl")
			// JPEG reconstruction at quality 100 avoids a second lossy encode. The
			// original remains staged until conversion and transfer registration finish.
			result, convertErr := client.Convert(ctx, path, output, mediaconvert.Options{Format: "jxl", Hardware: "cpu", Quality: 100, Distance: 0, Effort: 7, AllowLarger: true})
			if convertErr != nil {
				return fmt.Errorf("JPEG XL conversion failed; no file was published")
			}
			stat, statErr := os.Stat(output)
			if statErr != nil || stat.Size() <= 0 || stat.Size() > 256<<20 || result.Size != stat.Size() || result.Frames != 1 || result.Width <= 0 || result.Height <= 0 {
				return fmt.Errorf("converter returned invalid JPEG XL output")
			}
			header := make([]byte, 12)
			encoded, openErr := os.Open(output)
			if openErr != nil {
				return fmt.Errorf("cannot verify converted output")
			}
			n, readErr := io.ReadFull(encoded, header)
			encoded.Close()
			if (readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF) || !((n >= 2 && header[0] == 0xff && header[1] == 0x0a) || (n == 12 && bytes.Equal(header, []byte{0, 0, 0, 12, 'J', 'X', 'L', ' ', 13, 10, 135, 10}))) {
				return fmt.Errorf("converter output is not JPEG XL")
			}
			if outputMD5, err = mediaconvert.MD5(output); err != nil {
				return fmt.Errorf("cannot verify converted output")
			}
			path, info.Extension, info.ContentType, converted = output, "jxl", "image/jxl", true
		}
		if err = ctx.Err(); err != nil {
			return err
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
	writeVisualSimilarityJSON(w, map[string]int{"jobID": id})
}
