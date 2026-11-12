package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stashapp/stash/pkg/models"
)

// Upload post-processing runs only when explicitly requested for an item.
// Failed optional processing does not roll back a successful native import.
func applyMediaUploadDefaults(ctx context.Context, result *booruLibraryResult, tagging, conversion bool) {
	if result == nil {
		return
	}
	var notices []string
	if tagging {
		id, err := strconv.Atoi(result.ID)
		switch {
		case err != nil || id <= 0:
			err = fmt.Errorf("invalid imported media ID")
		case result.Kind == "image":
			// Camie's threshold, limit, filename authority and identity linking
			// come from the saved Image Tagging configuration.
			err = runBooruAutomaticImageTagging(ctx, id, true, false)
		case result.Kind == "scene":
			var config camieConfig
			config, err = loadCamieConfig()
			if err == nil {
				// Video tagging uses the existing conservative batch review rules:
				// only unambiguous filename metadata is auto-applied.
				outcome := processSceneTaggingBatchItem(ctx, id, config, sceneTaggingBatchRequest{AutoApply: true})
				switch {
				case outcome.Error != "":
					err = fmt.Errorf("%s", outcome.Error)
				case outcome.Applied == nil:
					notices = append(notices, "Video tagging found no unambiguous saved-default metadata to apply.")
				}
			}
		default:
			err = fmt.Errorf("unsupported media type")
		}
		switch {
		case err != nil:
			logger.Warnf("Upload tagging for %s #%s: %v", result.Kind, result.ID, err)
			notices = append(notices, "Tagging with saved defaults failed: "+err.Error())
		case result.Kind == "image":
			notices = append(notices, "Tagged with saved Image Tagging defaults.")
		case len(notices) == 0:
			notices = append(notices, "Applied saved Video Tagging defaults.")
		}
	}
	if conversion {
		notice, err := convertUploadedMediaWithDefaults(ctx, result)
		switch {
		case err != nil:
			logger.Warnf("Upload conversion for %s #%s: %v", result.Kind, result.ID, err)
			notices = append(notices, "Conversion with saved defaults failed: "+err.Error())
		case notice != "":
			notices = append(notices, notice)
		default:
			notices = append(notices, "Converted with saved Media converter defaults.")
		}
	}
	if len(notices) > 0 {
		result.Notice = strings.TrimSpace(strings.Join([]string{result.Notice, strings.Join(notices, " ")}, " "))
	}
}

// Share the native converter's saved rules, backend, quality and safety
// policy with manual uploads. Disabled format rules intentionally skip.
func convertUploadedMediaWithDefaults(ctx context.Context, result *booruLibraryResult) (string, error) {
	id, err := strconv.Atoi(result.ID)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("invalid imported media ID")
	}
	target := conversionTarget{Kind: result.Kind, ID: id}
	fileID, err := conversionTargetFile(ctx, target)
	if err != nil {
		return "", err
	}
	conversionMutations.Lock()
	defer conversionMutations.Unlock()

	store := conversionStore()
	config, err := store.Config()
	if err != nil {
		return "", err
	}
	skipNotice, err := booruDisabledConversionNotice(ctx, store, config, fileID)
	if err != nil || skipNotice != "" {
		return skipNotice, err
	}
	client, caps, backendNotice, err := conversionClient(ctx, config.Backend)
	if err != nil {
		return "", err
	}
	request := conversionRequest{UseEncodingDefaults: true}
	request.Options = mediaconvert.Options{Format: "auto", Hardware: "auto"}
	options, format, err := resolveConversionOptions(ctx, store, config, fileID, target, request, caps)
	if err != nil {
		return "", err
	}
	record, err := store.ConvertWithSavings(ctx, fileID, "upload-"+mediaconvert.NewID(), client, options, format, config.Savings)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("converter returned no result; original remains imported")
	}
	if record.Status == "skipped" {
		return "Conversion skipped: " + record.Error, nil
	}
	if record.Status != "complete" {
		return "", fmt.Errorf("conversion did not complete; original remains imported")
	}
	result.Converted = true
	if f := record.After.File(); f != nil {
		result.Path = f.Base().Path
	}
	if err := generateConvertedMedia(ctx, []models.FileID{fileID}); err != nil {
		return "Converted; thumbnail regeneration requires retry.", nil
	}
	if backendNotice != "" {
		return "Converted using saved defaults. " + backendNotice, nil
	}
	return "", nil
}
