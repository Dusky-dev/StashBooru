package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/imagerestore"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

var restorationMutex sync.Mutex
var restorationServerSession = mediaconvert.NewID()

func restorationStore() imagerestore.Store {
	return imagerestore.Store{Root: filepath.Join(manager.GetInstance().Config.GetConfigPath(), "image-restoration")}
}
func restorationClient(ctx context.Context, backend string) (imagerestore.Client, imagerestore.Capabilities, string, error) {
	var remote imagerestore.Client
	if backend != "local" {
		config, err := loadVisualSimilarityRemoteConfig()
		if err != nil {
			return remote, imagerestore.Capabilities{}, "", err
		}
		remote.URL, remote.Token = config.URL, config.Token
	}
	return imagerestore.SelectWorker(ctx, backend, imagerestore.Client{}, remote)
}

type restorationRequest struct {
	Action    string               `json:"action"`
	SessionID string               `json:"sessionID"`
	ImageID   int                  `json:"imageID"`
	Backend   string               `json:"backend"`
	Mask      string               `json:"mask"`
	Reference string               `json:"reference"`
	Feather   int                  `json:"feather"`
	Options   imagerestore.Options `json:"options"`
	Stack     bool                 `json:"stack"`
}

func restoreSource(ctx context.Context, record *imagerestore.Record) (models.File, error) {
	return validateRestorationSource(ctx, manager.GetInstance().Repository, record)
}

func validateRestorationSource(ctx context.Context, repo models.Repository, record *imagerestore.Record) (models.File, error) {
	source, err := imageDerivativeSource(ctx, repo, record.SourceImageID)
	if err != nil {
		return nil, err
	}
	base := source.Base()
	if int(base.ID) != record.SourceFileID || base.Path != record.SourcePath {
		return nil, fmt.Errorf("source primary file changed; prepare a new session")
	}
	stat, err := os.Lstat(base.Path)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() != record.SourceSize || base.Size != stat.Size() {
		return nil, fmt.Errorf("source changed; rescan and prepare a new session")
	}
	digest, err := imagerestore.SHA256(base.Path)
	if err != nil {
		return nil, err
	}
	if digest != record.SourceSHA256 {
		return nil, fmt.Errorf("source bytes changed; prepare a new session")
	}
	return source, nil
}

// Recovery identifies catalogue commits by the journal's unique destination.
// Interrupted previews are disposable; saved masks/provenance are permanent.
func recoverRestoration(ctx context.Context, record *imagerestore.Record) error {
	s := restorationStore()
	if record.Status == "saving" {
		var images []*models.Image
		err := txn.WithReadTxn(ctx, manager.GetInstance().Repository.TxnManager, func(ctx context.Context) error {
			repo := manager.GetInstance().Repository
			file, err := repo.File.FindByPath(ctx, record.Destination, true)
			if err != nil {
				return err
			}
			if file != nil {
				images, err = repo.Image.FindByFileID(ctx, file.Base().ID)
			}
			return err
		})
		if err != nil {
			return err
		}
		if len(images) == 1 {
			record.Status = "saved"
			record.DerivedImageID = images[0].ID
		} else {
			if err := os.Remove(record.Destination); err != nil && !os.IsNotExist(err) {
				return err
			}
			record.Status = "failed"
			record.Error = "save interrupted before catalogue commit; prepare a new session"
		}
		if record.Status == "saved" {
			_ = cleanSavedRestoration(s, record.ID)
		} else {
			_ = s.CleanTemporary(record.ID)
		}
		return s.Save(record)
	}
	if record.Status == "queued" || record.Status == "running" {
		native := manager.GetInstance().JobManager.GetJob(record.JobID)
		if record.ServerSession != restorationServerSession || native == nil || native.Status == job.StatusCancelled || native.Status == job.StatusFailed {
			record.Status = "cancelled"
			record.Error = "processing interrupted or cancelled; prepare a new session"
			if err := s.CleanTemporary(record.ID); err != nil {
				return err
			}
			return s.Save(record)
		}
	}
	return nil
}

func handleImageRestorationGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Query().Get("capabilities") == "1" {
		client, caps, notice, err := restorationClient(r.Context(), r.URL.Query().Get("backend"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		backend := "local"
		if client.URL != "" {
			backend = "remote"
		}
		writeVisualSimilarityJSON(w, map[string]interface{}{"capabilities": caps, "backend": backend, "notice": notice})
		return
	}
	restorationMutex.Lock()
	defer restorationMutex.Unlock()
	s := restorationStore()
	record, err := s.Load(r.URL.Query().Get("sessionID"))
	if err == nil {
		err = recoverRestoration(r.Context(), record)
	}
	if err == nil && record.Status != "saved" && !time.Now().Before(record.ExpiresAt) {
		err = fmt.Errorf("preview expired; prepare a new session")
		_ = s.CleanTemporary(record.ID)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if role := r.URL.Query().Get("file"); role != "" {
		if role != "source" && role != "output" && role != "effective-mask" && role != "mask" {
			http.Error(w, "invalid preview role", http.StatusBadRequest)
			return
		}
		if record.Status == "running" || record.Status == "queued" || record.Status == "saving" || record.Status == "cancelled" || record.Status == "failed" {
			http.Error(w, "preview unavailable", http.StatusConflict)
			return
		}
		dir, _ := s.Directory(record.ID)
		http.ServeFile(w, r, filepath.Join(dir, role+".png"))
		return
	}
	writeVisualSimilarityJSON(w, record)
}

func handleImageRestorationPost(w http.ResponseWriter, r *http.Request) {
	var request restorationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 48*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "send one JSON request", http.StatusBadRequest)
		return
	}
	if request.Action == "prepare" && (request.ImageID <= 0 || (request.Backend != "auto" && request.Backend != "local" && request.Backend != "remote")) {
		http.Error(w, "choose an image and auto/local/remote backend", http.StatusBadRequest)
		return
	}
	if request.Action == "generate" {
		if err := request.Options.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if request.Feather < 0 || request.Feather > 16 {
			http.Error(w, "feather radius must be 0–16 source pixels", http.StatusBadRequest)
			return
		}
	}
	restorationMutex.Lock()
	defer restorationMutex.Unlock()
	s := restorationStore()
	var record *imagerestore.Record
	var err error
	if request.Action == "prepare" {
		var usage int64
		var active int
		entries, readErr := os.ReadDir(s.Root)
		if readErr != nil && !os.IsNotExist(readErr) {
			err = readErr
		}
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				previous, loadErr := s.Load(entry.Name())
				if loadErr != nil {
					err = loadErr
					break
				}
				if recoveryErr := recoverRestoration(r.Context(), previous); recoveryErr != nil {
					err = recoveryErr
					break
				}
			}
		}
		if err == nil {
			usage, active, err = s.Prune(time.Now())
		}
		if err == nil && (usage > 768*1024*1024 || active >= 16) {
			err = fmt.Errorf("restoration preview cache full (1 GiB / 16 sessions); discard old sessions or wait 24 hours")
		}
		now := time.Now()
		record = &imagerestore.Record{ID: mediaconvert.NewID(), CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour), Status: "queued", SourceImageID: request.ImageID, Backend: request.Backend, ServerSession: restorationServerSession}
	} else {
		record, err = s.Load(request.SessionID)
		if err == nil {
			err = recoverRestoration(r.Context(), record)
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	switch request.Action {
	case "cancel":
		if record.Status == "saved" || record.Status == "saving" {
			http.Error(w, "saved provenance cannot be cancelled", http.StatusConflict)
			return
		}
		if record.ServerSession == restorationServerSession && (record.Status == "queued" || record.Status == "running") {
			manager.GetInstance().JobManager.CancelJob(record.JobID)
		}
		record.Status = "cancelled"
		record.Error = "cancelled; prepare a new session"
		// Running workers own cleanup after they acknowledge cancellation.
		native := manager.GetInstance().JobManager.GetJob(record.JobID)
		if native == nil || native.Status == job.StatusCancelled {
			_ = s.CleanTemporary(record.ID)
		}
		err = s.Save(record)
	case "discard":
		if record.Status == "saved" || record.Status == "saving" || record.Status == "queued" || record.Status == "running" {
			err = fmt.Errorf("cancel processing first; saved provenance cannot be discarded")
		} else {
			dir, _ := s.Directory(record.ID)
			err = os.RemoveAll(dir)
		}
	case "save":
		if record.Status != "preview" {
			err = fmt.Errorf("generate and review a preview before saving")
		} else {
			err = saveRestoration(r.Context(), record, request.Stack)
		}
	case "prepare", "generate":
		if request.Action == "generate" {
			switch {
			case !record.Capabilities.Available:
				err = fmt.Errorf("%s", record.Capabilities.Notice)
			case request.Options.Hardware == "cpu" && !record.Capabilities.CPU:
				err = fmt.Errorf("selected worker does not support CPU restoration")
			case request.Options.Hardware == "gpu" && !record.Capabilities.GPU:
				err = fmt.Errorf("selected worker has no CUDA GPU; GPU-only execution is unavailable")
			}
		}
		if request.Action == "generate" && record.Status != "ready" && record.Status != "preview" {
			err = fmt.Errorf("wait for source preparation before generating")
		}
		if err == nil {
			err = s.Save(record)
		}
		if err == nil && request.Action == "generate" {
			usage, _, usageErr := s.Prune(time.Now())
			switch {
			case usageErr != nil:
				err = usageErr
			case usage > 1024*1024*1024-288*1024*1024:
				err = fmt.Errorf("preview cache lacks headroom for restoration; discard older sessions")
			default:
				err = prepareRestoreMask(s, record, request)
			}
		}
		if err == nil {
			record.Status = "queued"
			record.Error = ""
			record.ServerSession = restorationServerSession
			record.JobID = manager.GetInstance().JobManager.Add(r.Context(), "Image restoration: "+request.Action, job.MakeJobExec(func(ctx context.Context, _ *job.Progress) error {
				return runRestoration(ctx, record.ID, request.Action)
			}))
			err = s.Save(record)
		}
	default:
		http.Error(w, "choose prepare, generate, save, cancel or discard", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, record)
}

func prepareRestoreMask(s imagerestore.Store, record *imagerestore.Record, request restorationRequest) error {
	if !time.Now().Before(record.ExpiresAt) {
		return fmt.Errorf("preview expired; prepare a new session")
	}
	dir, _ := s.Directory(record.ID)
	data, err := base64.StdEncoding.DecodeString(request.Mask)
	if err != nil || len(data) == 0 || len(data) > 24*1024*1024 {
		return fmt.Errorf("send a PNG mask of at most 24 MiB")
	}
	for _, name := range []string{"output.png", "mask.png", "effective-mask.png", "reference"} {
		_ = os.Remove(filepath.Join(dir, name))
	}
	// Any mask/options edit invalidates the old review immediately, even if validation fails.
	record.Status = "ready"
	record.Receipt = imagerestore.Receipt{}
	if err := s.Save(record); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "mask.png"), data, 0600); err != nil {
		return err
	}
	mask, err := imagerestore.ReadPNG(filepath.Join(dir, "mask.png"))
	if err != nil {
		return err
	}
	source, err := imagerestore.ReadPNG(filepath.Join(dir, "source.png"))
	if err != nil {
		return err
	}
	if source.Bounds() != mask.Bounds() {
		return fmt.Errorf("mask dimensions must match the prepared source")
	}
	effective, err := imagerestore.Feather(mask, request.Feather)
	if err != nil {
		return err
	}
	painted := false
	for _, pixel := range effective.Pix {
		painted = painted || pixel > 0
	}
	if !painted {
		return fmt.Errorf("paint a non-empty mask")
	}
	if err := imagerestore.WritePNG(filepath.Join(dir, "effective-mask.png"), effective); err != nil {
		return err
	}
	record.MaskSHA256, err = imagerestore.SHA256(filepath.Join(dir, "mask.png"))
	if err != nil {
		return err
	}
	record.EffectiveMaskSHA256, err = imagerestore.SHA256(filepath.Join(dir, "effective-mask.png"))
	if err != nil {
		return err
	}
	record.ReferenceSHA256 = ""
	if request.Reference != "" {
		if request.Options.Strength >= 1 {
			return fmt.Errorf("starting-pixel reference needs strength below 1")
		}
		ref, err := base64.StdEncoding.DecodeString(request.Reference)
		if err != nil || len(ref) > 8*1024*1024 {
			return fmt.Errorf("reference must be at most 8 MiB")
		}
		if err := os.WriteFile(filepath.Join(dir, "reference"), ref, 0600); err != nil {
			return err
		}
		record.ReferenceSHA256, err = imagerestore.SHA256(filepath.Join(dir, "reference"))
		if err != nil {
			return err
		}
	}
	record.Options = request.Options
	record.Options.Operation = "generate"
	record.Options.Signature = record.Capabilities.Signature
	record.Feather = request.Feather
	return s.Save(record)
}

func runRestoration(ctx context.Context, id, action string) (retErr error) {
	s := restorationStore()
	restorationMutex.Lock()
	record, err := s.Load(id)
	if err == nil && record.Status == "cancelled" {
		err = context.Canceled
	}
	if err == nil {
		record.Status = "running"
		err = s.Save(record)
	}
	restorationMutex.Unlock()
	if err != nil {
		return err
	}
	dir, _ := s.Directory(id)
	defer func() {
		restorationMutex.Lock()
		defer restorationMutex.Unlock()
		_ = os.Remove(filepath.Join(dir, "input.zip"))
		_ = os.Remove(filepath.Join(dir, "result.zip"))
		_ = os.Remove(filepath.Join(dir, "input-source"))
		if retErr != nil || ctx.Err() != nil {
			record.Status = "failed"
			record.Error = fmt.Sprint(retErr)
			if ctx.Err() != nil {
				record.Status = "cancelled"
				record.Error = "cancelled; prepare a new session"
			}
			_ = s.CleanTemporary(id)
		}
		retErr = joinRestorationError(retErr, s.Save(record))
	}()
	conversionMutations.Lock()
	defer conversionMutations.Unlock()
	client, caps, notice, err := restorationClient(ctx, record.Backend)
	if err != nil {
		return err
	}
	if client.URL != "" {
		record.Backend = "remote"
	} else {
		record.Backend = "local"
	}
	if action == "prepare" {
		record.Capabilities = caps
		record.Notice = notice
		source, err := imageUpscaleSource(ctx, record.SourceImageID)
		if err != nil {
			return err
		}
		base := source.Base()
		stat, err := os.Lstat(base.Path)
		if err != nil {
			return err
		}
		if !stat.Mode().IsRegular() || stat.Size() > 32*1024*1024 || stat.Size() != base.Size {
			return fmt.Errorf("prepare a regular still-image file of at most 32 MiB; rescan changed files")
		}
		record.SourcePath = base.Path
		record.SourceFileID = int(base.ID)
		record.SourceSize = stat.Size()
		record.SourceSHA256, err = imagerestore.SHA256(base.Path)
		if err != nil {
			return err
		}
		record.SourceMD5, err = mediaconvert.MD5(base.Path)
		if err != nil {
			return err
		}
		if stored := base.Fingerprints.GetString(models.FingerprintTypeMD5); stored != "" && stored != record.SourceMD5 {
			return fmt.Errorf("source content changed; rescan first")
		}
		if err := imagerestore.Bundle(filepath.Join(dir, "input.zip"), map[string]string{"source": base.Path}, imagerestore.Options{Operation: "prepare", Hardware: "auto", Steps: 20, Seed: 42, Guidance: 7.5, Strength: 1}); err != nil {
			return err
		}
	} else {
		if !caps.Available {
			return fmt.Errorf("%s", caps.Notice)
		}
		if caps.Signature != record.Capabilities.Signature {
			return fmt.Errorf("model/revision changed; prepare a new session")
		}
		if record.Options.Hardware == "cpu" && !caps.CPU {
			return fmt.Errorf("this worker does not support CPU restoration; enable STASH_RESTORATION_ALLOW_CPU=1")
		}
		if record.Options.Hardware == "gpu" && !caps.GPU {
			return fmt.Errorf("GPU-only restoration requires a CUDA GPU")
		}
		if _, err := restoreSource(ctx, record); err != nil {
			return err
		}
		entries := map[string]string{"source": filepath.Join(dir, "source.png"), "mask.png": filepath.Join(dir, "effective-mask.png")}
		if record.ReferenceSHA256 != "" {
			entries["reference"] = filepath.Join(dir, "reference")
		}
		if err := imagerestore.Bundle(filepath.Join(dir, "input.zip"), entries, record.Options); err != nil {
			return err
		}
	}
	if err := client.Process(ctx, filepath.Join(dir, "input.zip"), filepath.Join(dir, "result.zip")); err != nil {
		return err
	}
	if _, err := restoreSource(ctx, record); err != nil {
		return err
	}
	role := "output"
	if action == "prepare" {
		role = "source"
	}
	receipt, err := imagerestore.Unbundle(filepath.Join(dir, "result.zip"), dir, role)
	if err != nil {
		return err
	}
	decoded, err := imagerestore.ReadPNG(filepath.Join(dir, role+".png"))
	if err != nil {
		return err
	}
	if decoded.Bounds().Dx() != receipt.Width || decoded.Bounds().Dy() != receipt.Height {
		return fmt.Errorf("worker canvas dimensions disagree with PNG")
	}
	if action == "prepare" {
		record.CanonicalSHA256 = receipt.OutputSHA256
		record.Status = "ready"
	} else {
		if (receipt.Hardware != "cuda" && receipt.Hardware != "cpu") || (record.Options.Hardware == "gpu" && receipt.Hardware != "cuda") || (record.Options.Hardware == "cpu" && receipt.Hardware != "cpu") {
			return fmt.Errorf("worker did not honor restoration hardware selection")
		}
		if receipt.Signature != caps.Signature || receipt.Model != caps.Model || receipt.Revision != caps.Revision {
			return fmt.Errorf("worker returned a different model/revision")
		}
		if err := imagerestore.VerifyPixels(filepath.Join(dir, "source.png"), filepath.Join(dir, "effective-mask.png"), filepath.Join(dir, "output.png")); err != nil {
			return err
		}
		record.Status = "preview"
	}
	record.Receipt = receipt
	return ctx.Err()
}

func joinRestorationError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

func cleanSavedRestoration(s imagerestore.Store, id string) error {
	dir, err := s.Directory(id)
	if err != nil {
		return err
	}
	for _, name := range []string{"source.png", "output.png", "input.zip", "result.zip", "input-source"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func saveRestoration(ctx context.Context, record *imagerestore.Record, stack bool) error {
	conversionMutations.Lock()
	defer conversionMutations.Unlock()
	repo := manager.GetInstance().Repository
	s := restorationStore()
	if err := publishRestoration(ctx, repo, s, record, stack); err != nil {
		return err
	}
	// Thumbnail failure does not invalidate an already committed derivative.
	var fileID models.FileID
	err := txn.WithReadTxn(ctx, repo.TxnManager, func(ctx context.Context) error {
		im, err := repo.Image.Find(ctx, record.DerivedImageID)
		if err == nil && im != nil && im.PrimaryFileID != nil {
			fileID = *im.PrimaryFileID
		}
		return err
	})
	if err == nil && fileID != 0 {
		err = generateConvertedMedia(context.WithoutCancel(ctx), []models.FileID{fileID})
	}
	if err != nil {
		record.Notice = "Saved; thumbnail generation needs a retry: " + err.Error()
		_ = s.Save(record)
	}
	return nil
}

// Called with conversion serialization; the file and catalogue commit are journaled.
func publishRestoration(ctx context.Context, repo models.Repository, s imagerestore.Store, record *imagerestore.Record, stack bool) error {
	if !time.Now().Before(record.ExpiresAt) {
		return fmt.Errorf("preview expired; generate a new preview")
	}
	dir, _ := s.Directory(record.ID)
	source, err := validateRestorationSource(ctx, repo, record)
	if err != nil {
		return err
	}
	for name, expected := range map[string]string{"source.png": record.CanonicalSHA256, "mask.png": record.MaskSHA256, "effective-mask.png": record.EffectiveMaskSHA256, "output.png": record.Receipt.OutputSHA256} {
		actual, err := imagerestore.SHA256(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("preview bytes changed; generate a new preview")
		}
	}
	if record.ReferenceSHA256 != "" {
		hash, err := imagerestore.SHA256(filepath.Join(dir, "reference"))
		if err != nil {
			return err
		}
		if hash != record.ReferenceSHA256 {
			return fmt.Errorf("reference changed")
		}
	}
	if err := imagerestore.VerifyPixels(filepath.Join(dir, "source.png"), filepath.Join(dir, "effective-mask.png"), filepath.Join(dir, "output.png")); err != nil {
		return err
	}
	base := source.Base()
	record.Destination = base.Path[:len(base.Path)-len(filepath.Ext(base.Path))] + ".restored-" + record.ID + ".png"
	record.Status = "saving"
	if _, err := os.Lstat(record.Destination); err == nil {
		record.Status = "preview"
		return fmt.Errorf("derivative destination already exists")
	} else if !os.IsNotExist(err) {
		record.Status = "preview"
		return err
	}
	if err := s.Save(record); err != nil {
		record.Status = "preview"
		return err
	}
	input, err := os.Open(filepath.Join(dir, "output.png"))
	if err != nil {
		record.Status = "preview"
		_ = s.Save(record)
		return err
	}
	defer input.Close()
	out, err := os.OpenFile(record.Destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		record.Status = "preview"
		_ = s.Save(record)
		return err
	}
	keep := false
	defer func() {
		out.Close()
		if !keep {
			_ = os.Remove(record.Destination)
			record.Status = "preview"
			_ = s.Save(record)
		}
	}()
	if _, err := io.Copy(out, input); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if _, err := validateRestorationSource(ctx, repo, record); err != nil {
		return err
	}
	stat, err := os.Stat(record.Destination)
	if err != nil {
		return err
	}
	outputMD5, err := mediaconvert.MD5(record.Destination)
	if err != nil {
		return err
	}
	now := time.Now()
	derived := &models.ImageFile{BaseFile: &models.BaseFile{DirEntry: models.DirEntry{ModTime: stat.ModTime()}, Path: record.Destination, Basename: filepath.Base(record.Destination), ParentFolderID: base.ParentFolderID, Fingerprints: upscaledImageFingerprints(base.Fingerprints, record.SourceMD5, outputMD5), Size: stat.Size(), FrameCount: 1, CreatedAt: now, UpdatedAt: now}, Format: "png", Width: record.Receipt.Width, Height: record.Receipt.Height}
	err = txn.WithTxn(ctx, repo.TxnManager, func(ctx context.Context) error {
		amend := func(input *models.CreateImageInput) {
			input.Title += " [Generated restoration]"
			if input.CustomFields == nil {
				input.CustomFields = map[string]interface{}{}
			}
			provenance, _ := json.Marshal(map[string]interface{}{"version": 1, "generated": true, "sourceImageID": record.SourceImageID, "sourceSHA256": record.SourceSHA256, "maskSHA256": record.MaskSHA256, "effectiveMaskSHA256": record.EffectiveMaskSHA256, "referenceSHA256": record.ReferenceSHA256, "model": record.Receipt.Model, "revision": record.Receipt.Revision, "options": record.Options, "journalID": record.ID})
			input.CustomFields["StashBooru restoration"] = string(provenance)
		}
		if err := registerImageDerivative(ctx, repo, record.SourceImageID, models.FileID(record.SourceFileID), derived, amend, &record.DerivedImageID); err != nil {
			return err
		}
		if stack {
			return stackRestoration(ctx, repo, record.SourceImageID, record.DerivedImageID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	keep = true
	record.Status = "saved"
	if err := s.Save(record); err != nil {
		return err
	} // A saving journal recovers the committed native ID.
	if err := cleanSavedRestoration(s, record.ID); err != nil {
		return err
	}
	return nil
}

func stackRestoration(ctx context.Context, repo models.Repository, sourceID, derivedID int) error {
	source := models.MediaReference{Kind: models.MediaKindImage, ID: sourceID}
	derived := &models.VisualStackMemberInput{Media: models.MediaReference{Kind: models.MediaKindImage, ID: derivedID}, Label: "Generated restoration"}
	stack, err := repo.VisualStack.FindByMedia(ctx, source)
	if err != nil {
		return err
	}
	if stack == nil {
		_, err = repo.VisualStack.Create(ctx, models.VisualStackCreateInput{Members: []*models.VisualStackMemberInput{{Media: source, Label: "Source"}, derived}, Representative: source})
		return err
	}
	stack, err = repo.VisualStack.Find(ctx, stack.ID)
	if err != nil {
		return err
	}
	if stack == nil {
		return fmt.Errorf("source stack changed before saving")
	}
	var members []*models.VisualStackMemberInput
	representative := source
	for _, member := range stack.Members {
		members = append(members, &models.VisualStackMemberInput{Media: member.Media, Label: member.Label})
		if member.Representative {
			representative = member.Media
		}
	}
	members = append(members, derived)
	_, err = repo.VisualStack.Update(ctx, models.VisualStackUpdateInput{ID: stack.ID, Version: stack.Version, Title: stack.Title, Members: members, Representative: representative})
	return err
}
