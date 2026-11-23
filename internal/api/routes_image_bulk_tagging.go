package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/pixaitagger"
	"github.com/stashapp/stash/pkg/visualembedding"
)

// Keep the legacy Camie-only endpoint intact for existing callers. The new
// image-only job requires explicit IDs and one confirmation.
type bulkImageTagRequest struct {
	camieBulkRequestV2
	Confirmed      bool    `json:"confirmed"`
	UseFilename    bool    `json:"useFilename"`
	UseBooru       bool    `json:"useBooru"`
	AddPostLinks   bool    `json:"addPostLinks"`
	UsePixai       bool    `json:"usePixai"`
	UseCamie       bool    `json:"useCamie"`
	UseEva02       bool    `json:"useEva02"`
	PixaiThreshold float64 `json:"pixaiThreshold"`
	Eva02Threshold float64 `json:"eva02Threshold"`
}

// PixAI has no saved threshold; bulk uses Image Tagging's default.
const defaultPixaiThreshold = 0.35

// bulkImageTagClients are the model workers a job started.
type bulkImageTagClients struct {
	pixai pixaitagger.Tagger
	camie camietagger.Tagger
	eva02 visualembedding.Embedder
}

func (c bulkImageTagClients) close() {
	if c.pixai != nil {
		c.pixai.Close()
	}
	if c.camie != nil {
		c.camie.Close()
	}
	if c.eva02 != nil {
		c.eva02.Close()
	}
}

func startBulkImageTagClients(ctx context.Context, request bulkImageTagRequest) (bulkImageTagClients, error) {
	var clients bulkImageTagClients
	if request.UsePixai {
		client, _, _, err := newPixaiTagger()
		if err != nil {
			return clients, fmt.Errorf("starting PixAI tagging worker: %w", err)
		}
		clients.pixai = client
		status, err := client.Status(ctx)
		if err != nil {
			return clients, fmt.Errorf("checking PixAI tagging worker: %w", err)
		}
		if !status.Installed {
			return clients, fmt.Errorf("PixAI is not installed on the selected worker")
		}
	}
	if request.UseCamie {
		client, _, _, err := newCamieTagger()
		if err != nil {
			return clients, fmt.Errorf("starting Camie tagging worker: %w", err)
		}
		clients.camie = client
		status, err := client.Status(ctx)
		if err != nil {
			return clients, fmt.Errorf("checking Camie tagging worker: %w", err)
		}
		if !status.Installed {
			return clients, fmt.Errorf("camie v2 model is not installed on the selected worker")
		}
	}
	if request.UseEva02 {
		client, _, _, err := newVisualSimilarityEmbedder()
		if err != nil {
			return clients, fmt.Errorf("starting EVA02 tagging worker: %w", err)
		}
		clients.eva02 = client
		status, err := client.Status(ctx)
		if err != nil {
			return clients, fmt.Errorf("checking EVA02 tagging worker: %w", err)
		}
		if !status.Installed {
			return clients, fmt.Errorf("EVA02 model is not installed on the selected worker")
		}
	}
	return clients, nil
}

// bulkImageTagSummary counts what a job did, for its closing log line.
type bulkImageTagSummary struct {
	applied, empty, failures              int
	booruMatched, booruMissing, booruSkip int
	booruUnchecked, linksAdded            int
	booruUncategorized                    int
}

func (s bulkImageTagSummary) log(useBooru bool) {
	logger.Infof("Image Tagging: tagged %d Image(s), %d had no matching metadata, %d failed", s.applied, s.empty, s.failures)
	if useBooru {
		logger.Infof("Image Tagging: MD5 lookup matched %d Image(s) and added %d post link(s); %d not found, %d not checked because a site failed or rate limited, %d skipped because they already link to a chosen site; %d matched tag(s) were not applied because the site gave no category", s.booruMatched, s.linksAdded, s.booruMissing, s.booruUnchecked, s.booruSkip, s.booruUncategorized)
	}
}

func (rs imageRoutes) BulkTagImages(w http.ResponseWriter, r *http.Request) {
	var request bulkImageTagRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding Image bulk tagging request: %v", err), http.StatusBadRequest)
		return
	}
	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("loading Image Tagging settings: %v", err), http.StatusInternalServerError)
		return
	}
	var providers []booruProvider
	if request.UseBooru {
		sources, err := loadBooruLookupBrowserSources()
		if err != nil {
			http.Error(w, fmt.Sprintf("loading Booru browser sources: %v", err), http.StatusInternalServerError)
			return
		}
		providers = booruLookupProviders(config.BooruLookupSources, sources, searchBooruSourceMD5)
	}
	if status, err := validateBulkImageTagRequest(&request, config, len(providers)); err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	request.ImageIDs = append([]int(nil), request.ImageIDs...)
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Bulk Image tagging...", job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		ids, err := resolveCamieBulkImageIDs(ctx, mgr, camieBulkSelection{ImageIDs: request.ImageIDs})
		if err != nil {
			return err
		}
		progress.SetTotal(len(ids))

		clients, err := startBulkImageTagClients(ctx, request)
		defer clients.close()
		if err != nil {
			return err
		}
		var lookup *bulkBooruLookup
		if request.UseBooru {
			lookup = newBulkBooruLookup(providers)
		}

		var summary bulkImageTagSummary
		for _, imageID := range ids {
			if job.IsCancelled(ctx) {
				return nil
			}
			source, sourceErr := loadCamieBulkImageSource(ctx, mgr, imageID)
			if sourceErr != nil {
				summary.failures++
				logger.Warnf("bulk tagging: loading Image %d: %v", imageID, sourceErr)
				progress.Increment()
				continue
			}

			var booruPredictions []camietagger.Tag
			var postURL string
			if lookup != nil {
				booruPredictions, postURL = lookupBulkImageBooru(ctx, lookup, source, &summary)
			}

			predictions, tagErr := inferBulkImageTags(ctx, imageID, source.Path, config, request, clients, booruPredictions)
			if tagErr == nil {
				predictions = filterCamieBulkPredictionsV2(predictions, request.camieBulkRequestV2)
				predictions, tagErr = validateCamiePredictionsV2(predictions)
			}
			if tagErr == nil && len(predictions) > 0 && request.ApplyCharacters {
				// Without a reviewer, never guess between same-name Characters.
				var enriched []camietagger.Tag
				enriched, tagErr = enrichNativeCamiePredictionTargets(ctx, predictions)
				if tagErr == nil {
					var ambiguous []string
					predictions, ambiguous = withoutAmbiguousBulkCharacters(predictions, enriched)
					if len(ambiguous) > 0 {
						logger.Warnf("Image Tagging: Image %d: skipped Character(s) %s because several existing Characters share the name; use individual review", imageID, strings.Join(ambiguous, ", "))
					}
				}
			}
			if tagErr == nil && len(predictions) > 0 {
				_, tagErr = applyCamieMetadataV2(ctx, imageID, predictions, false)
				if tagErr == nil {
					summary.applied++
				}
			} else if tagErr == nil {
				// A filename that does not match the layout is not an error.
				summary.empty++
			}
			if tagErr == nil && postURL != "" && request.AddPostLinks {
				added, linkErr := addImageTaggingURLs(ctx, imageID, []string{postURL})
				if linkErr != nil {
					tagErr = fmt.Errorf("adding post link: %w", linkErr)
				}
				summary.linksAdded += len(added)
			}
			if tagErr != nil {
				summary.failures++
				logger.Warnf("bulk tagging: Image %d failed: %v", imageID, tagErr)
			}
			progress.Increment()
		}
		if lookup != nil {
			summary.booruUncategorized = lookup.uncategorized
		}
		summary.log(request.UseBooru)
		if summary.failures > 0 {
			return fmt.Errorf("bulk tagging failed for %d Image(s); see logs for Image IDs", summary.failures)
		}
		return nil
	}))
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}

// lookupBulkImageBooru returns the matched post's metadata and link. A site
// failure is counted, never treated as an Image failure.
func lookupBulkImageBooru(ctx context.Context, lookup *bulkBooruLookup, source camieBulkImageSource, summary *bulkImageTagSummary) ([]camietagger.Tag, string) {
	if lookup.hasPostLink(source.URLs) {
		// Images downloaded from a chosen site already carry its metadata.
		summary.booruSkip++
		return nil, ""
	}
	provider, post, _, _, err := lookupConvertedMediaBooruMetadataWith(ctx, source.Path, lookup.lookup)
	switch {
	case err == nil:
		summary.booruMatched++
	case errors.Is(err, errBooruNoMatch):
		summary.booruMissing++
		return nil, ""
	default:
		summary.booruUnchecked++
		if !errors.Is(err, errBooruNotChecked) && ctx.Err() == nil {
			logger.Debugf("Image Tagging: MD5 lookup for Image %d: %v", source.ID, err)
		}
		return nil, ""
	}
	return lookup.bulkBooruPredictions(ctx, provider, post), booruPostURL(provider, post)
}

// withoutAmbiguousBulkCharacters drops bare Character predictions that match
// several existing Characters. enriched is the target-enriched copy of
// predictions, in the same order.
func withoutAmbiguousBulkCharacters(predictions, enriched []camietagger.Tag) ([]camietagger.Tag, []string) {
	if len(enriched) != len(predictions) {
		return predictions, nil
	}
	kept := make([]camietagger.Tag, 0, len(predictions))
	var ambiguous []string
	for index, prediction := range predictions {
		if enriched[index].Category == "character" && len(enriched[index].TargetCandidates) > 1 {
			ambiguous = append(ambiguous, prediction.Name)
			continue
		}
		kept = append(kept, prediction)
	}
	return kept, ambiguous
}

// validateBulkImageTagRequest checks an explicit bulk selection and fills
// default model options. Local filename metadata and MD5 lookup need only the
// one explicit confirmation; model inference stays behind the bulk model
// opt-in. lookupSites is the number of usable MD5 lookup sites.
func validateBulkImageTagRequest(request *bulkImageTagRequest, config camieConfig, lookupSites int) (int, error) {
	if !request.Confirmed {
		return http.StatusBadRequest, fmt.Errorf("confirm this bulk tagging selection before starting")
	}
	if request.All || len(request.ImageIDs) < 2 || len(request.ImageIDs) > 20000 {
		return http.StatusBadRequest, fmt.Errorf("select between 2 and 20000 explicit Image IDs for bulk tagging")
	}
	useModels := request.UsePixai || request.UseCamie || request.UseEva02
	if !request.UseFilename && !request.UseBooru && !useModels {
		return http.StatusBadRequest, fmt.Errorf("choose at least one source for bulk tagging")
	}
	if !request.ApplyCharacters && !request.ApplyArtist && !request.ApplyCopyright && !request.ApplyTags {
		return http.StatusBadRequest, fmt.Errorf("select at least one Image metadata category")
	}
	if request.UseFilename && !config.FilenameEnabled {
		return http.StatusConflict, fmt.Errorf("reading metadata from filenames is off in tagging settings")
	}
	if request.UseBooru && lookupSites == 0 {
		return http.StatusConflict, fmt.Errorf("no MD5 lookup sites are available; choose them in tagging setup")
	}
	if useModels && !config.BulkTaggingEnabled {
		return http.StatusConflict, fmt.Errorf("bulk model tagging is disabled in tagging settings")
	}
	if !useModels {
		return 0, nil
	}
	if request.Threshold == 0 {
		request.Threshold = config.Threshold
	}
	if request.Eva02Threshold == 0 {
		request.Eva02Threshold = config.Eva02Threshold
	}
	if request.PixaiThreshold == 0 {
		request.PixaiThreshold = defaultPixaiThreshold
	}
	if request.Limit == 0 {
		request.Limit = config.Limit
	}
	for _, threshold := range []float64{request.Threshold, request.Eva02Threshold, request.PixaiThreshold} {
		if threshold <= 0 || threshold >= 1 {
			return http.StatusBadRequest, fmt.Errorf("model thresholds must be greater than 0 and less than 1")
		}
	}
	if request.Limit < 1 || request.Limit > camietagger.MaxLimit {
		return http.StatusBadRequest, fmt.Errorf("per-category limit must be between 1 and %d", camietagger.MaxLimit)
	}
	return 0, nil
}

func inferBulkImageTags(
	ctx context.Context,
	imageID int,
	path string,
	config camieConfig,
	request bulkImageTagRequest,
	clients bulkImageTagClients,
	booruPredictions []camietagger.Tag,
) ([]camietagger.Tag, error) {
	var filenameTags, pixaiTags, camieTags, evaTags []camietagger.Tag
	var err error

	if request.UseFilename {
		filenameTags, err = parseCamieFilename(path, config.FilenameLayout)
		if err != nil {
			return nil, fmt.Errorf("parsing local filename metadata: %w", err)
		}
	}
	if request.UsePixai {
		predicted, pixaiErr := clients.pixai.Tag(ctx, path, request.PixaiThreshold, request.Limit)
		if pixaiErr != nil {
			return nil, fmt.Errorf("PixAI inference: %w", pixaiErr)
		}
		for _, item := range predicted {
			pixaiTags = append(pixaiTags, camietagger.Tag{Name: item.Name, Category: item.Category, Score: item.Score, Source: "pixai"})
		}
	}
	if request.UseCamie {
		camieTags, err = clients.camie.Tag(ctx, path, request.Threshold, request.Limit)
		if err != nil {
			return nil, fmt.Errorf("camie inference: %w", err)
		}
	}
	if request.UseEva02 {
		tagPredictions, evaErr := clients.eva02.Tag(ctx, path, request.Eva02Threshold, request.Limit)
		if evaErr != nil {
			return nil, fmt.Errorf("EVA02 inference: %w", evaErr)
		}
		for _, prediction := range tagPredictions {
			converted, ok := eva02TagPredictionToCamie(prediction)
			if ok {
				evaTags = append(evaTags, converted)
			}
		}
	}
	merged := mergeBulkImageTags(filenameTags, booruPredictions, pixaiTags, camieTags, evaTags)
	merged, dropped := withoutSoloConflictCharacters(merged, filenameTags, booruPredictions)
	if len(dropped) > 0 {
		logger.Warnf("Image Tagging: Image %d is tagged as showing one person, but the models suggested %d Characters (%s); they were skipped, review the Image individually", imageID, len(dropped), strings.Join(dropped, ", "))
	}
	return merged, nil
}

// Source priority follows Image Tagging: filename → MD5 match → PixAI →
// Camie → EVA02. Filename and an exact MD5 match are authoritative: their
// identities block conflicting inferred identities in the same category. An
// already-selected higher-priority prediction keeps its score and provenance.
func mergeBulkImageTags(filename, booru []camietagger.Tag, models ...[]camietagger.Tag) []camietagger.Tag {
	var merged []camietagger.Tag
	seen := map[string]bool{}
	authoritative := map[string]bool{}
	layer := func(predictions []camietagger.Tag, claimsIdentity bool) {
		claimed := map[string]bool{}
		for _, prediction := range predictions {
			prediction = normalizeCamiePrediction(prediction)
			if prediction.Name == "" {
				continue
			}
			key := prediction.Category + "\x00" + strings.ToLower(prediction.Name)
			if prediction.Category == "character" {
				// The same Character in the other name order is a duplicate.
				key = "character\x00" + camieCharacterDuplicateKey(prediction)
			}
			if seen[key] || authoritative[prediction.Category] {
				continue
			}
			seen[key] = true
			merged = append(merged, prediction)
			if claimsIdentity && camieFilenameAuthoritativeCategory(prediction.Category) {
				claimed[prediction.Category] = true
			}
		}
		for category := range claimed {
			authoritative[category] = true
		}
	}
	layer(filename, true)
	layer(booru, true)
	for _, predictions := range models {
		layer(predictions, false)
	}
	return merged
}
