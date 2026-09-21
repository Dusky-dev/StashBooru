package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/hash/oshash"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/txn"
)

type imageUpscaleRequest struct {
	Backend             string               `json:"backend"`
	Targets             []conversionTarget   `json:"targets"`
	Options             mediaconvert.Options `json:"options"`
	UseEncodingDefaults bool                 `json:"useEncodingDefaults"`
}

func handleImageUpscalePost(w http.ResponseWriter, r *http.Request) {
	var request imageUpscaleRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(request.Targets) == 0 || len(request.Targets) > 10000 {
		http.Error(w, "select between 1 and 10000 images", http.StatusBadRequest)
		return
	}
	if request.Options.Upscaler != "waifu2x" && request.Options.Upscaler != "seedvr2" {
		http.Error(w, "choose waifu2x or SeedVR2", http.StatusBadRequest)
		return
	}
	if request.Options.UpscaleScale != 2 && request.Options.UpscaleScale != 4 {
		http.Error(w, "upscale scale must be 2 or 4", http.StatusBadRequest)
		return
	}
	seen := make(map[int]bool, len(request.Targets))
	unique := make([]conversionTarget, 0, len(request.Targets))
	for _, target := range request.Targets {
		if target.Kind != "image" || target.ID <= 0 {
			http.Error(w, "image upscaling only accepts image targets", http.StatusBadRequest)
			return
		}
		if !seen[target.ID] {
			seen[target.ID] = true
			unique = append(unique, target)
		}
	}
	request.Targets = unique

	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Upscale images (create copies)", job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		// Keep derivative creation serialized with destructive conversions so the
		// source cannot be swapped while the worker is reading it.
		conversionMutations.Lock()
		defer conversionMutations.Unlock()

		s := conversionStore()
		config, err := s.Config()
		if err != nil {
			return err
		}
		backend := request.Backend
		if backend == "" {
			backend = config.Backend
		}
		client, capabilities, _, err := conversionClient(ctx, backend)
		if err != nil {
			return err
		}
		if err := validateDerivativeUpscaler(capabilities, request.Options); err != nil {
			return err
		}

		progress.SetTotal(len(request.Targets))
		generated := make([]models.FileID, 0, len(request.Targets))
		for _, target := range request.Targets {
			if err := ctx.Err(); err != nil {
				return err
			}
			source, err := imageUpscaleSource(ctx, target.ID)
			if err != nil {
				return err
			}
			options := request.Options
			var input string
			if options.Format == "" || options.Format == "auto" {
				options.Format, input, err = conversionDefaultForFile(ctx, s, config, source.Base().ID)
			} else if request.UseEncodingDefaults {
				_, input, err = conversionDefaultForFile(ctx, s, config, source.Base().ID)
			}
			if err != nil {
				return err
			}
			if request.UseEncodingDefaults {
				defaults := config.DefaultEncoding(input)
				options.Quality, options.Effort = defaults.Quality, defaults.Effort
			}
			if options.Hardware == "" {
				options.Hardware = "auto"
			}
			if options.Format == "jxl" || options.Format == "ajxl" {
				options.Distance = mediaconvert.JXLDistanceFromQuality(options.Quality)
			}
			format, err := conversionOutputFormat(capabilities, options.Format, "image")
			if err != nil {
				return err
			}
			if format.Family == "video" {
				return fmt.Errorf("image derivatives require an image or animation output format")
			}
			newFileID, err := createUpscaledImageDerivative(ctx, target.ID, source, client, options, format)
			if err != nil {
				return fmt.Errorf("upscaling image %d: %w", target.ID, err)
			}
			generated = append(generated, newFileID)
			progress.Increment()
		}
		if len(generated) > 0 {
			return generateConvertedMedia(context.WithoutCancel(ctx), generated)
		}
		return nil
	}))
	writeVisualSimilarityJSON(w, map[string]int{"jobID": jobID})
}

func validateDerivativeUpscaler(capabilities mediaconvert.Capabilities, options mediaconvert.Options) error {
	for _, upscaler := range capabilities.Upscalers {
		if upscaler.ID != options.Upscaler {
			continue
		}
		if !upscaler.Available {
			if upscaler.Notice != "" {
				return fmt.Errorf("%s", upscaler.Notice)
			}
			return fmt.Errorf("upscaler %s is unavailable", options.Upscaler)
		}
		if options.Hardware == "cpu" && !upscaler.CPU {
			return fmt.Errorf("%s does not support CPU upscaling", upscaler.Label)
		}
		return nil
	}
	return fmt.Errorf("upscaler %s is unavailable on the selected worker", options.Upscaler)
}

func imageUpscaleSource(ctx context.Context, imageID int) (models.File, error) {
	mgr := manager.GetInstance()
	var source models.File
	err := txn.WithReadTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		im, err := mgr.Repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if im == nil || im.PrimaryFileID == nil {
			return fmt.Errorf("image has no primary file")
		}
		files, err := mgr.Repository.File.Find(ctx, *im.PrimaryFileID)
		if err != nil {
			return err
		}
		if len(files) != 1 {
			return fmt.Errorf("image primary file was not found")
		}
		if _, ok := files[0].(*models.ImageFile); !ok {
			return fmt.Errorf("image primary file is not a still-image file")
		}
		if files[0].Base().FrameCount > 1 {
			return fmt.Errorf("non-destructive upscaling currently supports still images only")
		}
		if files[0].Base().ZipFileID != nil {
			return fmt.Errorf("extract archived media before upscaling")
		}
		source = files[0].Clone()
		source.Base().Fingerprints = append(models.Fingerprints(nil), files[0].Base().Fingerprints...)
		return nil
	})
	return source, err
}

func createUpscaledImageDerivative(ctx context.Context, sourceImageID int, source models.File, client mediaconvert.Client, options mediaconvert.Options, format mediaconvert.Format) (models.FileID, error) {
	sourceImageFile, ok := source.(*models.ImageFile)
	if !ok {
		return 0, fmt.Errorf("source is not an image file")
	}
	base := source.Base()
	stat, err := os.Lstat(base.Path)
	if err != nil {
		return 0, err
	}
	if !stat.Mode().IsRegular() {
		return 0, fmt.Errorf("only regular image files can be upscaled")
	}
	if base.Size != stat.Size() {
		return 0, fmt.Errorf("source size changed; rescan before upscaling")
	}
	sourceMD5, err := mediaconvert.MD5(base.Path)
	if err != nil {
		return 0, err
	}
	if stored := base.Fingerprints.GetString(models.FingerprintTypeMD5); stored != "" && stored != sourceMD5 {
		return 0, fmt.Errorf("source changed; rescan before upscaling")
	}

	token := mediaconvert.NewID()
	directory := filepath.Dir(base.Path)
	stage := filepath.Join(directory, ".stash-upscale-"+token+"."+format.Extension)
	defer os.Remove(stage)
	result, err := client.Convert(ctx, base.Path, stage, options)
	if err != nil {
		return 0, err
	}
	stageStat, err := os.Stat(stage)
	if err != nil {
		return 0, err
	}
	if result.Size <= 0 || result.Size != stageStat.Size() || result.Format != format.Extension || result.Frames != 1 || result.Width <= 0 || result.Height <= 0 {
		return 0, fmt.Errorf("upscaler returned invalid output metadata")
	}
	if sourceImageFile.Width > 0 && sourceImageFile.Height > 0 {
		expectedWidth := sourceImageFile.Width * options.UpscaleScale
		expectedHeight := sourceImageFile.Height * options.UpscaleScale
		if result.Width != expectedWidth || result.Height != expectedHeight {
			return 0, fmt.Errorf("upscaler returned %dx%d; expected %dx%d", result.Width, result.Height, expectedWidth, expectedHeight)
		}
	}
	if result.Upscaler != options.Upscaler {
		return 0, fmt.Errorf("worker did not report the requested upscaler")
	}
	if currentMD5, err := mediaconvert.MD5(base.Path); err != nil || currentMD5 != sourceMD5 {
		return 0, fmt.Errorf("source changed during upscaling; derivative was not registered")
	}
	outputMD5, err := mediaconvert.MD5(stage)
	if err != nil {
		return 0, err
	}

	stem := base.Path[:len(base.Path)-len(filepath.Ext(base.Path))]
	destination := fmt.Sprintf("%s.upscaled-%s-%dx-%s.%s", stem, options.Upscaler, options.UpscaleScale, token[:8], format.Extension)
	if _, err := os.Lstat(destination); err == nil {
		return 0, fmt.Errorf("upscaled destination already exists")
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if err := os.Rename(stage, destination); err != nil {
		return 0, err
	}
	keepDestination := false
	defer func() {
		if !keepDestination {
			_ = os.Remove(destination)
		}
	}()
	_ = os.Chmod(destination, stat.Mode().Perm())
	published, err := os.Stat(destination)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	fingerprints := models.Fingerprints{
		{Type: models.FingerprintTypeMD5, Fingerprint: outputMD5},
		{Type: "source_md5", Fingerprint: sourceMD5},
	}
	if value, err := oshash.FromFilePath(destination); err == nil {
		fingerprints = append(fingerprints, models.Fingerprint{Type: models.FingerprintTypeOshash, Fingerprint: value})
	}
	imageFormat := result.VideoCodec
	if imageFormat == "" {
		imageFormat = result.Format
	}
	derived := &models.ImageFile{
		BaseFile: &models.BaseFile{
			DirEntry:       models.DirEntry{ModTime: published.ModTime()},
			Path:           destination,
			Basename:       filepath.Base(destination),
			ParentFolderID: base.ParentFolderID,
			Fingerprints:   fingerprints,
			Size:           result.Size,
			FrameCount:     1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		Format: imageFormat,
		Width:  result.Width,
		Height: result.Height,
	}

	mgr := manager.GetInstance()
	err = txn.WithTxn(ctx, mgr.Repository.TxnManager, func(ctx context.Context) error {
		sourceImage, err := mgr.Repository.Image.Find(ctx, sourceImageID)
		if err != nil {
			return err
		}
		if sourceImage == nil || sourceImage.PrimaryFileID == nil || *sourceImage.PrimaryFileID != base.ID {
			return fmt.Errorf("source image changed during upscaling; derivative was not registered")
		}
		if err := sourceImage.LoadURLs(ctx, mgr.Repository.Image); err != nil {
			return err
		}
		if err := sourceImage.LoadPerformerIDs(ctx, mgr.Repository.Image); err != nil {
			return err
		}
		if err := sourceImage.LoadTagIDs(ctx, mgr.Repository.Image); err != nil {
			return err
		}
		if err := sourceImage.LoadGalleryIDs(ctx, mgr.Repository.Image); err != nil {
			return err
		}
		customFields, err := mgr.Repository.Image.GetCustomFields(ctx, sourceImage.ID)
		if err != nil {
			return err
		}
		if err := mgr.Repository.File.Create(ctx, derived); err != nil {
			return err
		}

		clone := *sourceImage
		clone.ID = 0
		clone.PrimaryFileID = nil
		clone.Path = ""
		clone.Checksum = ""
		clone.CreatedAt = now
		clone.UpdatedAt = now
		input := &models.CreateImageInput{
			Image:        &clone,
			FileIDs:      []models.FileID{derived.Base().ID},
			CustomFields: customFields,
		}
		if err := mgr.Repository.Image.Create(ctx, input); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	keepDestination = true
	return derived.Base().ID, nil
}
