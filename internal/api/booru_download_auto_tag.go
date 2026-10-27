package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/visualembedding"
)

// Tagging is a best-effort post-import step. A model outage or ambiguous
// identity never makes the successfully imported Stash media disappear.
func (rs booruBrowserRoutes) autoTagDownloadedImage(ctx context.Context, settings booruDownloadSettings, result *booruLibraryResult) bool {
	if result == nil || result.Kind != "image" {
		return false
	}
	id, err := strconv.Atoi(result.ID)
	if err == nil && id <= 0 {
		err = fmt.Errorf("invalid imported Image ID")
	}
	if err == nil {
		run := rs.autoTagImage
		if run == nil {
			run = runBooruAutomaticImageTagging
		}
		err = run(ctx, id, settings.AutoTagCamie, settings.AutoTagEva02)
	}
	if err != nil {
		logger.Warnf("Booru download auto-tagging: image %s: %v", result.ID, err)
		result.Notice = strings.TrimSpace(strings.Join([]string{result.Notice, "Automatic Image tagging failed; retry the download to reattempt."}, " "))
		return false
	}
	result.Notice = strings.TrimSpace(strings.Join([]string{result.Notice, "Automatic Image tagging completed."}, " "))
	return true
}

// Reuse the actual Metadata Image Review inference, merge, validation and
// native apply code. Current Camie/EVA02 thresholds and filename authority
// come from the existing Image Tagging settings; existing artists are kept.
func runBooruAutomaticImageTagging(ctx context.Context, imageID int, useCamie, useEva02 bool) error {
	if !useCamie && !useEva02 {
		return fmt.Errorf("automatic image tagging has no enabled models")
	}
	config, err := loadCamieConfig()
	if err != nil {
		return fmt.Errorf("loading Image Tagging defaults: %w", err)
	}
	image, err := loadCamieBulkImageSource(ctx, manager.GetInstance(), imageID)
	if err != nil {
		return fmt.Errorf("loading imported Image: %w", err)
	}
	var camieClient camietagger.Tagger
	var evaClient visualembedding.Embedder
	if useCamie {
		camieClient, _, _, err = newCamieTagger()
		if err != nil {
			return err
		}
		defer camieClient.Close()
		status, statusErr := camieClient.Status(ctx)
		if statusErr != nil {
			return statusErr
		}
		if !status.Installed {
			return fmt.Errorf("camie model is not installed on the selected worker")
		}
	}
	if useEva02 {
		evaClient, _, _, err = newVisualSimilarityEmbedder()
		if err != nil {
			return err
		}
		defer evaClient.Close()
		status, statusErr := evaClient.Status(ctx)
		if statusErr != nil {
			return statusErr
		}
		if !status.Installed {
			return fmt.Errorf("EVA02 model is not installed on the selected worker")
		}
	}
	request := bulkImageTagRequest{
		camieBulkRequestV2: camieBulkRequestV2{
			Threshold: config.Threshold, Limit: config.Limit,
			ApplyCharacters: true, ApplyArtist: true,
			ApplyCopyright: true, ApplyTags: true,
		},
		UseCamie: useCamie, UseEva02: useEva02,
		Eva02Threshold: config.Eva02Threshold,
	}
	predictions, err := inferBulkImageTags(ctx, image.Path, config, request, camieClient, evaClient)
	if err != nil {
		return err
	}
	predictions = filterCamieBulkPredictionsV2(predictions, request.camieBulkRequestV2)
	predictions, err = validateCamiePredictionsV2(predictions)
	if err != nil || len(predictions) == 0 {
		return err
	}
	_, err = applyCamieMetadataV2(ctx, imageID, predictions, false)
	return err
}
