package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
)

const (
	defaultLibrarySimilarityNeighbors = 8
	defaultLibrarySimilarityMinVotes  = 2
	maxLibrarySimilarityNeighbors     = 50
)

type librarySimilarityMetadataVote struct {
	NeighborID int
	Similarity float64
	Prediction camietagger.Tag
}

type librarySimilarityMetadataConsensus struct {
	Prediction     camietagger.Tag `json:"prediction"`
	Votes          int             `json:"votes"`
	MeanSimilarity float64         `json:"meanSimilarity"`
}

type librarySimilarityMetadataResponse struct {
	Source     string                               `json:"source"`
	MediaType  string                               `json:"mediaType"`
	Reference  int                                  `json:"referenceID"`
	Neighbors  int                                  `json:"neighbors"`
	MinVotes   int                                  `json:"minVotes"`
	Tags       []camietagger.Tag                    `json:"tags"`
	Consensus  []librarySimilarityMetadataConsensus `json:"consensus"`
}

func librarySimilarityMetadataKey(prediction camietagger.Tag) string {
	prediction = normalizeCamiePrediction(prediction)
	return prediction.Category + "\x00" + strings.ToLower(strings.TrimSpace(prediction.Name))
}

// aggregateLibrarySimilarityMetadata converts metadata from nearest indexed
// library neighbors into review-only consensus suggestions. One neighbor gets
// at most one vote for a canonical prediction, preventing tag duplication on a
// single media item from inflating consensus.
func aggregateLibrarySimilarityMetadata(votes []librarySimilarityMetadataVote, minVotes int) []librarySimilarityMetadataConsensus {
	if minVotes < 1 {
		minVotes = 1
	}

	type aggregate struct {
		prediction camietagger.Tag
		neighbors  map[int]struct{}
		total      float64
	}
	byKey := make(map[string]*aggregate)
	for _, vote := range votes {
		if vote.NeighborID <= 0 || vote.Similarity < 0 || vote.Similarity > 1 {
			continue
		}
		prediction := normalizeCamiePrediction(vote.Prediction)
		key := librarySimilarityMetadataKey(prediction)
		if strings.TrimSpace(prediction.Name) == "" || strings.TrimSpace(prediction.Category) == "" {
			continue
		}
		entry := byKey[key]
		if entry == nil {
			entry = &aggregate{prediction: prediction, neighbors: make(map[int]struct{})}
			byKey[key] = entry
		}
		if _, duplicate := entry.neighbors[vote.NeighborID]; duplicate {
			continue
		}
		entry.neighbors[vote.NeighborID] = struct{}{}
		entry.total += vote.Similarity
	}

	result := make([]librarySimilarityMetadataConsensus, 0, len(byKey))
	for _, entry := range byKey {
		count := len(entry.neighbors)
		if count < minVotes {
			continue
		}
		prediction := entry.prediction
		prediction.Source = "library-similarity"
		prediction.Score = entry.total / float64(count)
		result = append(result, librarySimilarityMetadataConsensus{
			Prediction:     prediction,
			Votes:          count,
			MeanSimilarity: prediction.Score,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Votes != result[j].Votes {
			return result[i].Votes > result[j].Votes
		}
		if result[i].MeanSimilarity != result[j].MeanSimilarity {
			return result[i].MeanSimilarity > result[j].MeanSimilarity
		}
		return librarySimilarityMetadataKey(result[i].Prediction) < librarySimilarityMetadataKey(result[j].Prediction)
	})
	return result
}

func librarySimilarityScore(distance float64) float64 {
	score := 1 - distance
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func librarySimilarityCharacterPrediction(entity *models.Performer) camietagger.Tag {
	name := strings.TrimSpace(entity.Name)
	if disambiguation := strings.TrimSpace(entity.Disambiguation); disambiguation != "" {
		name = fmt.Sprintf("%s (%s)", name, disambiguation)
	}
	return camietagger.Tag{
		Name:         name,
		RawName:      name,
		Category:     "character",
		Score:        1,
		Source:       "library-similarity",
		TargetPath:   fmt.Sprintf("/performers/%d", entity.ID),
		TargetExists: true,
	}
}

func librarySimilarityStudioPrediction(entity *models.Studio) camietagger.Tag {
	return camietagger.Tag{
		Name:         entity.Name,
		RawName:      entity.Name,
		Category:     "artist",
		Score:        1,
		Source:       "library-similarity",
		TargetPath:   fmt.Sprintf("/studios/%d", entity.ID),
		TargetExists: true,
	}
}

func librarySimilarityCopyrightPrediction(entity *models.Copyright) camietagger.Tag {
	return camietagger.Tag{
		Name:         entity.Name,
		RawName:      entity.Name,
		Category:     "copyright",
		Score:        1,
		Source:       "library-similarity",
		TargetPath:   fmt.Sprintf("/copyrights/%d", entity.ID),
		TargetExists: true,
	}
}

func librarySimilarityTagPrediction(entity *models.Tag) camietagger.Tag {
	return camietagger.Tag{
		Name:         entity.Name,
		RawName:      entity.Name,
		Category:     "general",
		Score:        1,
		Source:       "library-similarity",
		TargetPath:   fmt.Sprintf("/tags/%d", entity.ID),
		TargetExists: true,
	}
}

func librarySimilarityImagePredictions(ctx context.Context, repository models.Repository, imageID int) ([]camietagger.Tag, error) {
	image, err := repository.Image.Find(ctx, imageID)
	if err != nil {
		return nil, err
	}
	if image == nil {
		return nil, fmt.Errorf("image %d not found", imageID)
	}
	if err := image.LoadPerformerIDs(ctx, repository.Image); err != nil {
		return nil, err
	}
	if err := image.LoadTagIDs(ctx, repository.Image); err != nil {
		return nil, err
	}

	performers, err := repository.Performer.FindMany(ctx, image.PerformerIDs.List())
	if err != nil {
		return nil, err
	}
	artists, err := repository.ImageArtist.FindByImageID(ctx, imageID)
	if err != nil {
		return nil, err
	}
	copyrights, err := repository.Copyright.FindByImageID(ctx, imageID)
	if err != nil {
		return nil, err
	}
	tags, err := repository.Tag.FindMany(ctx, image.TagIDs.List())
	if err != nil {
		return nil, err
	}

	predictions := make([]camietagger.Tag, 0, len(performers)+len(artists)+len(copyrights)+len(tags))
	for _, entity := range performers {
		predictions = append(predictions, librarySimilarityCharacterPrediction(entity))
	}
	for _, entity := range artists {
		predictions = append(predictions, librarySimilarityStudioPrediction(entity))
	}
	for _, entity := range copyrights {
		predictions = append(predictions, librarySimilarityCopyrightPrediction(entity))
	}
	for _, entity := range tags {
		predictions = append(predictions, librarySimilarityTagPrediction(entity))
	}
	return predictions, nil
}

func librarySimilarityScenePredictions(ctx context.Context, repository models.Repository, sceneID int) ([]camietagger.Tag, error) {
	scene, err := repository.Scene.Find(ctx, sceneID)
	if err != nil {
		return nil, err
	}
	if scene == nil {
		return nil, fmt.Errorf("video %d not found", sceneID)
	}
	if err := scene.LoadPerformerIDs(ctx, repository.Scene); err != nil {
		return nil, err
	}
	if err := scene.LoadTagIDs(ctx, repository.Scene); err != nil {
		return nil, err
	}

	performers, err := repository.Performer.FindMany(ctx, scene.PerformerIDs.List())
	if err != nil {
		return nil, err
	}
	artists, err := repository.SceneArtist.FindBySceneID(ctx, sceneID)
	if err != nil {
		return nil, err
	}
	copyrights, err := repository.Copyright.FindBySceneID(ctx, sceneID)
	if err != nil {
		return nil, err
	}
	tags, err := repository.Tag.FindMany(ctx, scene.TagIDs.List())
	if err != nil {
		return nil, err
	}

	predictions := make([]camietagger.Tag, 0, len(performers)+len(artists)+len(copyrights)+len(tags))
	for _, entity := range performers {
		predictions = append(predictions, librarySimilarityCharacterPrediction(entity))
	}
	for _, entity := range artists {
		predictions = append(predictions, librarySimilarityStudioPrediction(entity))
	}
	for _, entity := range copyrights {
		predictions = append(predictions, librarySimilarityCopyrightPrediction(entity))
	}
	for _, entity := range tags {
		predictions = append(predictions, librarySimilarityTagPrediction(entity))
	}
	return predictions, nil
}

func librarySimilarityOptions(r *http.Request) (int, int, error) {
	neighbors := defaultLibrarySimilarityNeighbors
	minVotes := defaultLibrarySimilarityMinVotes
	if raw := strings.TrimSpace(r.URL.Query().Get("neighbors")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxLibrarySimilarityNeighbors {
			return 0, 0, fmt.Errorf("library similarity neighbors must be between 1 and %d", maxLibrarySimilarityNeighbors)
		}
		neighbors = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("minVotes")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > neighbors {
			return 0, 0, fmt.Errorf("library similarity minVotes must be between 1 and %d", neighbors)
		}
		minVotes = value
	}
	if minVotes > neighbors {
		minVotes = neighbors
	}
	return neighbors, minVotes, nil
}

func collectLibrarySimilarityMetadata(
	ctx context.Context,
	repository models.Repository,
	mediaType string,
	referenceID int,
	neighbors int,
	minVotes int,
) (librarySimilarityMetadataResponse, error) {
	var matches []sqlite.VisualSimilarityMatch
	var err error
	switch mediaType {
	case "image":
		matches, err = sqlite.VisualEmbeddings.FindSimilarImages(ctx, referenceID, neighbors)
	case "video":
		matches, err = sqlite.VisualEmbeddings.FindSimilarScenes(ctx, referenceID, neighbors)
	default:
		return librarySimilarityMetadataResponse{}, fmt.Errorf("unsupported library similarity media type %q", mediaType)
	}
	if err != nil {
		return librarySimilarityMetadataResponse{}, err
	}

	votes := make([]librarySimilarityMetadataVote, 0)
	for _, match := range matches {
		var predictions []camietagger.Tag
		switch mediaType {
		case "image":
			predictions, err = librarySimilarityImagePredictions(ctx, repository, match.ID)
		case "video":
			predictions, err = librarySimilarityScenePredictions(ctx, repository, match.ID)
		}
		if err != nil {
			return librarySimilarityMetadataResponse{}, fmt.Errorf("loading %s similarity neighbor %d metadata: %w", mediaType, match.ID, err)
		}
		similarity := librarySimilarityScore(match.Distance)
		for _, prediction := range predictions {
			votes = append(votes, librarySimilarityMetadataVote{
				NeighborID: match.ID,
				Similarity: similarity,
				Prediction: prediction,
			})
		}
	}

	consensus := aggregateLibrarySimilarityMetadata(votes, minVotes)
	tags := make([]camietagger.Tag, 0, len(consensus))
	for index := range consensus {
		tags = append(tags, consensus[index].Prediction)
	}
	tags = enrichNativeCamiePredictionTargets(ctx, tags)
	for index := range consensus {
		if index < len(tags) {
			consensus[index].Prediction = tags[index]
		}
	}

	return librarySimilarityMetadataResponse{
		Source:    "library-similarity",
		MediaType: mediaType,
		Reference: referenceID,
		Neighbors: len(matches),
		MinVotes:  minVotes,
		Tags:      tags,
		Consensus: consensus,
	}, nil
}

func (rs imageRoutes) ImageLibrarySimilarityMetadata(w http.ResponseWriter, r *http.Request) {
	neighbors, minVotes, err := librarySimilarityOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	image := r.Context().Value(imageKey).(*models.Image)
	repository := manager.GetInstance().Repository
	var response librarySimilarityMetadataResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var collectErr error
		response, collectErr = collectLibrarySimilarityMetadata(ctx, repository, "image", image.ID, neighbors, minVotes)
		return collectErr
	}); err != nil {
		http.Error(w, fmt.Sprintf("loading library-similarity metadata: %v", err), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, response)
}

func (rs sceneRoutes) SceneLibrarySimilarityMetadata(w http.ResponseWriter, r *http.Request) {
	neighbors, minVotes, err := librarySimilarityOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	scene := r.Context().Value(sceneKey).(*models.Scene)
	repository := manager.GetInstance().Repository
	var response librarySimilarityMetadataResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var collectErr error
		response, collectErr = collectLibrarySimilarityMetadata(ctx, repository, "video", scene.ID, neighbors, minVotes)
		return collectErr
	}); err != nil {
		http.Error(w, fmt.Sprintf("loading library-similarity metadata: %v", err), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
