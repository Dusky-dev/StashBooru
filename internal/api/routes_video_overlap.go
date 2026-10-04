package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stashapp/stash/pkg/videooverlap"
)

var videoOverlapSession = videooverlap.NewID()
var videoOverlapWork sync.Mutex
var videoOverlapRequests sync.Mutex

func videoOverlapStore() videooverlap.Store {
	return videooverlap.Store{Root: filepath.Join(manager.GetInstance().Config.GetConfigPath(), "video-overlap")}
}

// Every read is anchored in the native primary file, never a worker path or
// source_md5 lineage value. Workers cannot select files or mutate the catalogue.
func videoOverlapSource(ctx context.Context, repository models.Repository, id int) (videooverlap.Source, error) {
	var ret videooverlap.Source
	err := txn.WithReadTxn(ctx, repository.TxnManager, func(ctx context.Context) error {
		scene, err := repository.Scene.Find(ctx, id)
		if err != nil {
			return err
		}
		if scene == nil {
			return fmt.Errorf("video %d no longer exists", id)
		}
		if err = scene.LoadPrimaryFile(ctx, repository.File); err != nil {
			return err
		}
		f := scene.Files.Primary()
		if f == nil || f.Base().ZipFileID != nil {
			return fmt.Errorf("video %d needs a regular primary file", id)
		}
		base := f.Base()
		ret = videooverlap.Source{SceneID: id, FileID: int64(base.ID), Path: base.Path, Fingerprint: base.Fingerprints.GetString("md5") + ":" + base.Fingerprints.GetString("oshash")}
		ret, err = videooverlap.Measure(ret)
		if err != nil {
			return err
		}
		if ret.Size != base.Size {
			return fmt.Errorf("video %d size differs from the catalogue; rescan it before indexing", id)
		}
		return nil
	})
	return ret, err
}

type videoOverlapInfo struct {
	ID             int                `json:"id"`
	Title          string             `json:"title"`
	Basename       string             `json:"basename"`
	Media          videooverlap.Media `json:"media"`
	MetadataFields int                `json:"metadataFields"`
}

func videoOverlapDetails(ctx context.Context, repository models.Repository, id int, media videooverlap.Media) (videoOverlapInfo, error) {
	ret := videoOverlapInfo{ID: id, Media: media}
	err := txn.WithReadTxn(ctx, repository.TxnManager, func(ctx context.Context) error {
		scene, err := repository.Scene.Find(ctx, id)
		if err != nil {
			return err
		}
		if scene == nil {
			return fmt.Errorf("video %d no longer exists", id)
		}
		if err = scene.LoadPrimaryFile(ctx, repository.File); err != nil {
			return err
		}
		ret.Title = scene.GetTitle()
		if f := scene.Files.Primary(); f != nil {
			ret.Basename = f.Base().Basename
		}
		if err = scene.LoadURLs(ctx, repository.Scene); err != nil {
			return err
		}
		if err = scene.LoadTagIDs(ctx, repository.Scene); err != nil {
			return err
		}
		if err = scene.LoadPerformerIDs(ctx, repository.Scene); err != nil {
			return err
		}
		for _, present := range []bool{scene.Title != "", scene.Details != "", scene.Date != nil, scene.Rating != nil, scene.StudioID != nil, len(scene.URLs.List()) > 0, len(scene.TagIDs.List()) > 0, len(scene.PerformerIDs.List()) > 0} {
			if present {
				ret.MetadataFields++
			}
		}
		return nil
	})
	return ret, err
}

func videoOverlapClient(ctx context.Context, c videooverlap.Config) (videooverlap.Client, videooverlap.Capabilities, string, error) {
	mgr := manager.GetInstance()
	local, remote := videooverlap.Client{}, videooverlap.Client{}
	if mgr.FFMpeg != nil {
		local.FFMpeg = mgr.FFMpeg.Path()
	}
	if mgr.FFProbe != nil {
		local.FFProbe = mgr.FFProbe.Path()
	}
	if c.Backend != "local" {
		config, err := loadVisualSimilarityRemoteConfig()
		if err != nil {
			return local, videooverlap.Capabilities{}, "", err
		}
		remote.URL, remote.Token = config.URL, config.Token
	}
	return videooverlap.SelectClient(ctx, c.Backend, local, remote)
}

type videoOverlapRequest struct {
	Action    string                     `json:"action"`
	Config    *videooverlap.Config       `json:"config,omitempty"`
	IDs       []int                      `json:"ids,omitempty"`
	All       bool                       `json:"all"`
	Force     bool                       `json:"force"`
	Reference int                        `json:"reference"`
	Options   *videooverlap.MatchOptions `json:"options,omitempty"`
	JobID     string                     `json:"id,omitempty"`
}

func videoOverlapBusy(j *videooverlap.Job) bool {
	if j.Session != videoOverlapSession || j.NativeID == 0 {
		return false
	}
	native := manager.GetInstance().JobManager.GetJob(j.NativeID)
	return native != nil && (native.Status == job.StatusReady || native.Status == job.StatusRunning || native.Status == job.StatusStopping)
}

func videoOverlapObservedStatus(j videooverlap.Job, native *job.Job) string {
	if j.Status == "queued" || j.Status == "running" {
		if j.Session != videoOverlapSession || native == nil {
			return "interrupted"
		}
		if native.Status == job.StatusCancelled {
			return "cancelled"
		}
		if native.Status == job.StatusFinished || native.Status == job.StatusFailed {
			return "interrupted"
		}
	}
	return j.Status
}

func videoOverlapPublicJob(j videooverlap.Job) videooverlap.Job {
	var native *job.Job
	if (j.Status == "queued" || j.Status == "running") && j.Session == videoOverlapSession {
		native = manager.GetInstance().JobManager.GetJob(j.NativeID)
	}
	j.Status = videoOverlapObservedStatus(j, native)
	j.Session = ""
	j.Targets = nil
	j.FailedIDs = nil
	j.Versions = nil
	j.Review = nil
	return j
}

func sceneIDPage(ctx context.Context, r models.Repository, cursor, upper, size int, descending bool) ([]int, int, error) {
	sortBy := "id"
	direction := models.SortDirectionEnumAsc
	if descending {
		direction = models.SortDirectionEnumDesc
	}
	var ids []int
	var total int
	err := txn.WithReadTxn(ctx, r.TxnManager, func(ctx context.Context) error {
		options := models.SceneQueryOptions{QueryOptions: models.QueryOptions{FindFilter: &models.FindFilterType{Sort: &sortBy, Direction: &direction, PerPage: &size}, Count: true}}
		if upper > 0 {
			options.SceneFilter = &models.SceneFilterType{ID: &models.IntCriterionInput{Value: cursor + 1, Value2: &upper, Modifier: models.CriterionModifierBetween}}
		}
		result, err := r.Scene.Query(ctx, options)
		if err != nil {
			return err
		}
		ids, total = result.IDs, result.Count
		return nil
	})
	return ids, total, err
}

func (rs sceneRoutes) VideoOverlap(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		videoOverlapGet(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	var request videoOverlapRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid video review request: "+err.Error(), http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "expected one video review request", http.StatusBadRequest)
		return
	}
	// Serialize checkpoint admission with native-ID assignment. Concurrent resume
	// requests must observe the first request's queued job before adding another.
	videoOverlapRequests.Lock()
	defer videoOverlapRequests.Unlock()
	ctx := r.Context()
	s := videoOverlapStore()
	config, err := s.Config(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if request.Action == "configure" {
		if request.Config == nil {
			http.Error(w, "missing video index configuration", 400)
			return
		}
		if err = s.Configure(ctx, *request.Config); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeVisualSimilarityJSON(w, request.Config)
		return
	}
	if request.Config != nil {
		if err = request.Config.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		config = *request.Config
	}
	if request.Action == "cancel" {
		j, e := s.LoadJob(ctx, request.JobID)
		if e != nil || j == nil {
			http.Error(w, "video job not found", 404)
			return
		}
		if videoOverlapBusy(j) {
			manager.GetInstance().JobManager.CancelJob(j.NativeID)
		}
		writeVisualSimilarityJSON(w, map[string]bool{"cancelled": true})
		return
	}
	j := &videooverlap.Job{ID: videooverlap.NewID(), Action: request.Action, Config: config, Status: "queued", Session: videoOverlapSession, Targets: request.IDs, All: request.All, Force: request.Force, Items: []videooverlap.JobItem{}, FailedIDs: []int{}}
	switch request.Action {
	case "resume":
		j, err = s.LoadJob(ctx, request.JobID)
		if err != nil || j == nil || j.Action != "index" {
			http.Error(w, "index checkpoint not found", 404)
			return
		}
		if videoOverlapBusy(j) {
			http.Error(w, "this video job is already queued or running", http.StatusConflict)
			return
		}
		j.Status, j.Session, j.Error = "queued", videoOverlapSession, ""
	case "index":
		if j.All == (len(j.Targets) > 0) || len(j.Targets) > 10000 {
			http.Error(w, "select 1–10000 Videos or all Videos", 400)
			return
		}
		seen := map[int]bool{}
		ids := []int{}
		for _, id := range j.Targets {
			if id <= 0 {
				http.Error(w, "invalid Video ID", 400)
				return
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		j.Targets = ids
		j.Total = len(ids)
		if j.All {
			var last []int
			last, j.Total, err = sceneIDPage(ctx, manager.GetInstance().Repository, 0, 0, 1, true)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if len(last) > 0 {
				j.UpperID = last[0]
			}
		}
	case "search":
		if request.Reference <= 0 {
			http.Error(w, "select a reference Video", 400)
			return
		}
		o := videooverlap.DefaultMatchOptions()
		if request.Options != nil {
			o = *request.Options
		}
		if err = o.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		j.Review = &videooverlap.Review{Reference: request.Reference, Options: o, Matches: []videooverlap.Match{}, Errors: []string{}}
		j.Versions = map[int]string{}
	default:
		http.Error(w, "unknown video review action", 400)
		return
	}
	ready := make(chan struct{})
	defer close(ready)
	if err = s.SaveJob(ctx, j); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	j.NativeID = manager.GetInstance().JobManager.Add(ctx, "Video interval review: "+j.Action, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) (retErr error) {
		<-ready
		videoOverlapWork.Lock()
		defer videoOverlapWork.Unlock()
		j.Status = "running"
		defer func() {
			switch {
			case ctx.Err() != nil:
				j.Status = "cancelled"
			case retErr != nil:
				j.Status = "failed"
				j.Error = retErr.Error()
			case j.Failed > 0:
				j.Status = "complete_with_errors"
			default:
				j.Status = "complete"
			}
			final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if e := s.SaveJob(final, j); retErr == nil && e != nil {
				retErr = e
			}
		}()
		if err := ctx.Err(); err != nil {
			return err
		}
		client, caps, backend, e := videoOverlapClient(ctx, j.Config)
		if e != nil {
			return e
		}
		j.Backend = backend
		if e = s.SaveJob(ctx, j); e != nil {
			return e
		}
		if j.Action == "index" {
			return runVideoOverlapIndex(ctx, manager.GetInstance().Repository, s, j, client, caps, progress)
		}
		return runVideoOverlapSearch(ctx, manager.GetInstance().Repository, s, j, caps, progress)
	}))
	if err = s.SaveJob(ctx, j); err != nil {
		manager.GetInstance().JobManager.CancelJob(j.NativeID)
		http.Error(w, err.Error(), 500)
		return
	}
	writeVisualSimilarityJSON(w, map[string]any{"id": j.ID, "jobID": j.NativeID})
}

type videoOverlapProgress interface {
	SetTotal(int)
	SetProcessed(int)
	Increment()
}

func runVideoOverlapIndex(ctx context.Context, r models.Repository, s videooverlap.Store, j *videooverlap.Job, client videooverlap.Client, caps videooverlap.Capabilities, progress videoOverlapProgress) error {
	progress.SetTotal(j.Total)
	progress.SetProcessed(j.Processed)
	process := func(id int, advance bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		item := videooverlap.JobItem{ID: id, Status: "indexed"}
		source, err := videoOverlapSource(ctx, r, id)
		if err == nil {
			var existing *videooverlap.Signature
			existing, err = s.Get(ctx, id)
			if err == nil && existing != nil && !j.Force && existing.Current(source, j.Config, caps.Signature) {
				item.Status = "current"
			} else if err == nil {
				var signature videooverlap.Signature
				signature, err = client.Index(ctx, source, j.Config, caps)
				if err == nil {
					var current videooverlap.Source
					current, err = videoOverlapSource(ctx, r, id)
					if err == nil && current != source {
						err = fmt.Errorf("native primary file changed during indexing")
					}
					if err == nil {
						err = s.Put(ctx, signature)
					}
				}
			}
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if err != nil {
			item.Status, item.Error = "failed", err.Error()
		}
		items := j.Items[:0]
		for _, previous := range j.Items {
			if previous.ID != id {
				items = append(items, previous)
			}
		}
		j.Items = items
		j.AddItem(item)
		if advance {
			if j.All {
				j.Cursor = id
			} else {
				j.Cursor++
			}
		}
		progress.SetProcessed(j.Processed)
		return s.SaveJob(ctx, j)
	}
	// Finished entries are atomic checkpoints. Retry failed entries, then continue
	// at the stored native ID/list cursor; completed media are not reprocessed.
	retry := append([]int(nil), j.FailedIDs...)
	j.FailedIDs = []int{}
	j.Processed -= len(retry)
	j.Failed -= len(retry)
	for i, id := range retry {
		before := j.Processed
		if err := process(id, false); err != nil {
			first := i
			if j.Processed > before {
				first++
			}
			j.FailedIDs = append(j.FailedIDs, retry[first:]...)
			j.Processed += len(retry[first:])
			j.Failed += len(retry[first:])
			return err
		}
	}
	if j.All {
		for j.UpperID > 0 && j.Cursor < j.UpperID {
			ids, _, err := sceneIDPage(ctx, r, j.Cursor, j.UpperID, 500, false)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				break
			}
			for _, id := range ids {
				if err = process(id, true); err != nil {
					return err
				}
			}
		}
	} else {
		for j.Cursor < len(j.Targets) {
			if err := process(j.Targets[j.Cursor], true); err != nil {
				return err
			}
		}
	}
	return nil
}

func runVideoOverlapSearch(ctx context.Context, r models.Repository, s videooverlap.Store, j *videooverlap.Job, caps videooverlap.Capabilities, progress videoOverlapProgress) error {
	review := j.Review
	source, err := videoOverlapSource(ctx, r, review.Reference)
	if err != nil {
		return err
	}
	reference, err := s.Get(ctx, review.Reference)
	if err != nil {
		return err
	}
	if reference == nil || !reference.Current(source, j.Config, caps.Signature) {
		return fmt.Errorf("reference Video is missing or stale in this decoder/settings index; index it first")
	}
	j.Versions[review.Reference] = reference.Key()
	review.Candidates, err = s.Candidates(ctx, *reference, review.Options.MaxCandidates)
	if err != nil {
		return err
	}
	j.Total = len(review.Candidates.IDs)
	progress.SetTotal(j.Total)
	exactReferenceChecked := false
	for _, id := range review.Candidates.IDs {
		if err = ctx.Err(); err != nil {
			return err
		}
		candidate, e := s.Get(ctx, id)
		var current videooverlap.Source
		if e == nil {
			current, e = videoOverlapSource(ctx, r, id)
		}
		if e == nil && (candidate == nil || !candidate.Current(current, j.Config, caps.Signature)) {
			e = fmt.Errorf("video %d index is stale; index it again", id)
		}
		if e == nil && reference.SHA256 == candidate.SHA256 {
			if !exactReferenceChecked {
				var digest string
				digest, e = videooverlap.DigestFile(ctx, reference.Source.Path)
				if e == nil && digest != reference.SHA256 {
					return fmt.Errorf("reference contents changed; force reindex after scanning")
				}
				exactReferenceChecked = e == nil
			}
			if e == nil {
				var digest string
				digest, e = videooverlap.DigestFile(ctx, candidate.Source.Path)
				if e == nil && digest != candidate.SHA256 {
					e = fmt.Errorf("video %d contents changed; force reindex it", id)
				}
			}
		}
		if e == nil {
			var match videooverlap.Match
			match, e = videooverlap.Compare(ctx, *reference, *candidate, review.Options)
			if match.Limited {
				review.AlignmentLimited++
			}
			if e == nil && match.Class != "" {
				review.Matches = append(review.Matches, match)
				j.Versions[id] = candidate.Key()
			}
		}
		if e != nil {
			review.Skipped++
			if len(review.Errors) < 50 {
				review.Errors = append(review.Errors, fmt.Sprintf("Video %d: %s", id, e))
			}
		}
		j.Processed++
		progress.Increment()
		if j.Processed%10 == 0 {
			if err = s.SaveJob(ctx, j); err != nil {
				return err
			}
		}
	}
	videooverlap.SortMatches(review.Matches)
	return nil
}

type videoOverlapRow struct {
	Match videooverlap.Match `json:"match"`
	A     videoOverlapInfo   `json:"a"`
	B     videoOverlapInfo   `json:"b"`
	Stale string             `json:"stale,omitempty"`
}

func videoOverlapGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := videoOverlapStore()
	repository := manager.GetInstance().Repository
	config, err := s.Config(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	count, err := s.Count(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var totalVideos int
	err = txn.WithReadTxn(ctx, repository.TxnManager, func(ctx context.Context) error { var e error; totalVideos, e = repository.Scene.Count(ctx); return e })
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	reference, _ := strconv.Atoi(r.URL.Query().Get("reference"))
	jobs, err := s.LatestJobs(ctx, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	response := map[string]any{"config": config, "indexed": count, "totalVideos": totalVideos, "algorithm": videooverlap.Algorithm, "jobs": []videooverlap.Job{}}
	public := make([]videooverlap.Job, 0, len(jobs))
	for _, j := range jobs {
		public = append(public, videoOverlapPublicJob(j))
	}
	response["jobs"] = public
	if id := r.URL.Query().Get("id"); id != "" {
		j, e := s.LoadJob(ctx, id)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		if j != nil {
			response["job"] = videoOverlapPublicJob(*j)
		}
	}
	var chosen *videooverlap.Job
	if id := r.URL.Query().Get("reviewID"); id != "" {
		chosen, err = s.LoadJob(ctx, id)
	} else if reference > 0 {
		var reviews []videooverlap.Job
		reviews, err = s.LatestJobs(ctx, reference)
		if len(reviews) > 0 {
			chosen = &reviews[0]
		}
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if chosen != nil && chosen.Review != nil {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			http.Error(w, "invalid review page", 400)
			return
		}
		review := chosen.Review
		start := min((page-1)*10, len(review.Matches))
		end := min(start+10, len(review.Matches))
		rows := []videoOverlapRow{}
		for _, match := range review.Matches[start:end] {
			row := videoOverlapRow{Match: match, A: videoOverlapInfo{ID: match.A}, B: videoOverlapInfo{ID: match.B}}
			for _, id := range []int{match.A, match.B} {
				signature, e := s.Get(ctx, id)
				if e != nil || signature == nil {
					row.Stale = "Video signature is no longer available; run another index/search."
					continue
				}
				current, e := videoOverlapSource(ctx, repository, id)
				if e != nil || !signature.Current(current, chosen.Config, signature.Decoder) || signature.Key() != chosen.Versions[id] {
					row.Stale = "Primary file or index changed since this review; index/search again."
				}
				info, e := videoOverlapDetails(ctx, repository, id, signature.Media)
				if e != nil {
					row.Stale = e.Error()
				}
				if id == match.A {
					row.A = info
				} else {
					row.B = info
				}
			}
			rows = append(rows, row)
		}
		response["review"] = map[string]any{"id": chosen.ID, "reference": review.Reference, "options": review.Options, "candidates": review.Candidates, "total": len(review.Matches), "rows": rows, "skipped": review.Skipped, "errors": review.Errors, "alignmentLimited": review.AlignmentLimited, "page": page, "status": videoOverlapPublicJob(*chosen).Status}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeVisualSimilarityJSON(w, response)
}
