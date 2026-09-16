package api

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/models"
)

const (
	defaultVideoTaggingFrameSamples = 3
	maxVideoTaggingFrameSamples     = 5
)

type videoFrameMetadataResponse struct {
	Backend     string            `json:"backend"`
	Model       string            `json:"model"`
	Threshold   float64           `json:"threshold"`
	Limit       int               `json:"limit"`
	SampleTimes []float64         `json:"sampleTimes"`
	Tags        []camietagger.Tag `json:"tags"`
}

// videoTaggingFrameSampleTimes returns evenly distributed representative frame
// timestamps away from the exact beginning and end of a video. Frame analysis
// remains explicit; this helper only defines the deterministic sampling plan.
func videoTaggingFrameSampleTimes(durationSeconds float64, requested int) []float64 {
	if durationSeconds <= 0 {
		return nil
	}
	if requested <= 0 {
		requested = defaultVideoTaggingFrameSamples
	}
	if requested > maxVideoTaggingFrameSamples {
		requested = maxVideoTaggingFrameSamples
	}

	step := durationSeconds / float64(requested+1)
	times := make([]float64, requested)
	for index := range times {
		times[index] = step * float64(index+1)
	}
	return times
}

func videoFramePredictionValue(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "_", " ")
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func videoFramePredictionIdentityKeys(prediction camietagger.Tag) []string {
	prediction = normalizeCamiePrediction(prediction)
	category := strings.ToLower(strings.TrimSpace(prediction.Category))
	if category == "" {
		category = "general"
	}

	seen := map[string]struct{}{}
	keys := make([]string, 0, 2)
	for _, raw := range []string{prediction.Name, prediction.RawName} {
		value := videoFramePredictionValue(raw)
		if value == "" {
			continue
		}
		key := category + "\x00" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func videoFramePredictionKey(prediction camietagger.Tag) string {
	keys := videoFramePredictionIdentityKeys(prediction)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// aggregateVideoFramePredictions gives each sampled frame at most one vote per
// canonical prediction. Missing-frame votes contribute zero to the aggregate
// score, so repeated predictions naturally outrank one-frame false positives.
func aggregateVideoFramePredictions(frameTags [][]camietagger.Tag, limit int) []camietagger.Tag {
	if len(frameTags) == 0 {
		return nil
	}

	type aggregate struct {
		prediction camietagger.Tag
		scoreSum   float64
		votes      int
	}
	aggregates := map[string]*aggregate{}

	for _, tags := range frameTags {
		bestForFrame := map[string]camietagger.Tag{}
		for _, raw := range tags {
			prediction := normalizeCamiePrediction(raw)
			key := videoFramePredictionKey(prediction)
			if key == "" {
				continue
			}
			current, ok := bestForFrame[key]
			if !ok || prediction.Score > current.Score {
				bestForFrame[key] = prediction
			}
		}

		for key, prediction := range bestForFrame {
			entry := aggregates[key]
			if entry == nil {
				entry = &aggregate{prediction: prediction}
				aggregates[key] = entry
			} else if prediction.Score > entry.prediction.Score {
				entry.prediction = prediction
			}
			entry.scoreSum += prediction.Score
			entry.votes++
		}
	}

	result := make([]camietagger.Tag, 0, len(aggregates))
	for _, entry := range aggregates {
		prediction := entry.prediction
		prediction.Score = entry.scoreSum / float64(len(frameTags))
		prediction.Source = fmt.Sprintf("frame-analysis:%d/%d", entry.votes, len(frameTags))
		result = append(result, prediction)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		leftCategory := strings.ToLower(result[i].Category)
		rightCategory := strings.ToLower(result[j].Category)
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func filterVideoFramePredictionsForLocalPriority(local, frames []camietagger.Tag) []camietagger.Tag {
	identityCategories := map[string]struct{}{
		"character": {},
		"artist":    {},
		"copyright": {},
	}
	localCategories := map[string]struct{}{}
	localKeys := map[string]struct{}{}
	for _, prediction := range local {
		category := strings.ToLower(strings.TrimSpace(normalizeCamiePrediction(prediction).Category))
		if _, ok := identityCategories[category]; !ok {
			continue
		}
		localCategories[category] = struct{}{}
		for _, key := range videoFramePredictionIdentityKeys(prediction) {
			localKeys[key] = struct{}{}
		}
	}

	filtered := make([]camietagger.Tag, 0, len(frames))
	for _, prediction := range frames {
		category := strings.ToLower(strings.TrimSpace(normalizeCamiePrediction(prediction).Category))
		if _, identity := identityCategories[category]; !identity {
			filtered = append(filtered, prediction)
			continue
		}
		if _, authoritative := localCategories[category]; !authoritative {
			filtered = append(filtered, prediction)
			continue
		}

		for _, key := range videoFramePredictionIdentityKeys(prediction) {
			if _, same := localKeys[key]; same {
				filtered = append(filtered, prediction)
				break
			}
		}
	}
	return filtered
}

func parseVideoFrameTaggingOptions(r *http.Request) (int, float64, int, error) {
	samples := defaultVideoTaggingFrameSamples
	if raw := strings.TrimSpace(r.URL.Query().Get("samples")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxVideoTaggingFrameSamples {
			return 0, 0, 0, fmt.Errorf("samples must be between 1 and %d", maxVideoTaggingFrameSamples)
		}
		samples = parsed
	}

	threshold := camietagger.DefaultThreshold
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 || parsed >= 1 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, 0, 0, fmt.Errorf("threshold must be a number greater than 0 and less than 1")
		}
		threshold = parsed
	}

	limit := camietagger.DefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > camietagger.MaxLimit {
			return 0, 0, 0, fmt.Errorf("limit must be between 1 and %d", camietagger.MaxLimit)
		}
		limit = parsed
	}
	return samples, threshold, limit, nil
}

// SceneFrameMetadata is intentionally an explicit endpoint. Opening Video
// Tagging never extracts frames or starts Camie inference on its own.
func (rs sceneRoutes) SceneFrameMetadata(w http.ResponseWriter, r *http.Request) {
	scene := r.Context().Value(sceneKey).(*models.Scene)
	primary := scene.Files.Primary()
	if primary == nil || strings.TrimSpace(primary.Base().Path) == "" {
		http.Error(w, "video has no primary file", http.StatusNotFound)
		return
	}

	samples, threshold, limit, err := parseVideoFrameTaggingOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sampleTimes := videoTaggingFrameSampleTimes(primary.DurationFinite(), samples)
	if len(sampleTimes) == 0 {
		http.Error(w, "video duration is unavailable", http.StatusConflict)
		return
	}

	instance := manager.GetInstance()
	if instance.FFMpeg == nil {
		http.Error(w, "ffmpeg is unavailable", http.StatusServiceUnavailable)
		return
	}

	client, backend, _, err := newCamieTagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	status, err := client.Status(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("checking %s Camie worker: %v", backend, err), http.StatusBadGateway)
		return
	}
	if !status.Installed {
		http.Error(w, "Camie Tagger v2 is not installed", http.StatusConflict)
		return
	}

	tempDir, err := os.MkdirTemp("", "stash-video-tagging-frames-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("creating frame workspace: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	frameTags := make([][]camietagger.Tag, 0, len(sampleTimes))
	for index, timestamp := range sampleTimes {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%02d.jpg", index+1))
		args := ffmpeg.Args{}.
			LogLevel(ffmpeg.LogLevelError).
			Seek(timestamp).
			Input(primary.Base().Path).
			VideoFrames(1).
			SkipAudio().
			Overwrite().
			Output(framePath)
		if err := instance.FFMpeg.Generate(r.Context(), args); err != nil {
			http.Error(w, fmt.Sprintf("extracting frame at %.3fs: %v", timestamp, err), http.StatusBadGateway)
			return
		}

		tags, err := client.Tag(r.Context(), framePath, threshold, limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("tagging frame at %.3fs with %s Camie worker: %v", timestamp, backend, err), http.StatusBadGateway)
			return
		}
		frameTags = append(frameTags, tags)
	}

	predictions := aggregateVideoFramePredictions(frameTags, limit)
	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if config.FilenameEnabled {
		local, err := parseCamieFilename(primary.Base().Path, config.FilenameLayout)
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing local filename metadata: %v", err), http.StatusBadRequest)
			return
		}
		predictions = filterVideoFramePredictionsForLocalPriority(local, predictions)
	}
	predictions, err = enrichNativeCamiePredictionTargets(r.Context(), predictions)
	if err != nil {
		http.Error(w, fmt.Sprintf("enriching frame-assisted Video Tagging targets: %v", err), http.StatusInternalServerError)
		return
	}

	writeVisualSimilarityJSON(w, videoFrameMetadataResponse{
		Backend:     backend,
		Model:       camietagger.Model,
		Threshold:   threshold,
		Limit:       limit,
		SampleTimes: sampleTimes,
		Tags:        predictions,
	})
}
