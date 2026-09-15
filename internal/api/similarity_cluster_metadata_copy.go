package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

const maxSimilarityMetadataCopyTargets = 1000

type similarityMetadataCopyRequest struct {
	SourceImageID  int   `json:"sourceImageID"`
	TargetImageIDs []int `json:"targetImageIDs"`
}

type similarityMetadataCopyCounts struct {
	Characters int `json:"characters"`
	Artists    int `json:"artists"`
	Copyrights int `json:"copyrights"`
	Tags       int `json:"tags"`
}

type similarityMetadataCopyResult struct {
	ImageID int                          `json:"imageID"`
	Added   similarityMetadataCopyCounts `json:"added"`
	Error   string                       `json:"error,omitempty"`
}

type similarityMetadataCopyResponse struct {
	SourceImageID int                            `json:"sourceImageID"`
	Results       []similarityMetadataCopyResult `json:"results"`
}

type similarityMetadataSnapshot struct {
	performerIDs []int
	artistIDs    []int
	copyrightIDs []int
	tagIDs       []int
}

func (rs imageRoutes) CopySimilarityClusterMetadata(w http.ResponseWriter, r *http.Request) {
	var request similarityMetadataCopyRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decoding similarity metadata copy request: %v", err), http.StatusBadRequest)
		return
	}
	if request.SourceImageID <= 0 {
		http.Error(w, "sourceImageID must be a positive image ID", http.StatusBadRequest)
		return
	}

	targetIDs, err := normalizeSimilarityMetadataCopyTargets(request.SourceImageID, request.TargetImageIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repository := manager.GetInstance().Repository
	source, err := loadSimilarityMetadataSnapshot(r.Context(), repository, request.SourceImageID)
	if err != nil {
		http.Error(w, fmt.Sprintf("loading source image %d metadata: %v", request.SourceImageID, err), http.StatusNotFound)
		return
	}

	response := similarityMetadataCopyResponse{
		SourceImageID: request.SourceImageID,
		Results:       make([]similarityMetadataCopyResult, 0, len(targetIDs)),
	}
	for _, targetID := range targetIDs {
		result := similarityMetadataCopyResult{ImageID: targetID}
		added, err := copySimilarityMetadataToImage(r.Context(), repository, targetID, source)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Added = added
		}
		response.Results = append(response.Results, result)
	}

	writeVisualSimilarityJSON(w, response)
}

func normalizeSimilarityMetadataCopyTargets(sourceID int, raw []int) ([]int, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("select at least one target image")
	}
	if len(raw) > maxSimilarityMetadataCopyTargets {
		return nil, fmt.Errorf("cannot copy metadata to more than %d images at once", maxSimilarityMetadataCopyTargets)
	}

	seen := make(map[int]struct{}, len(raw))
	ret := make([]int, 0, len(raw))
	for _, id := range raw {
		if id <= 0 {
			return nil, fmt.Errorf("target image IDs must be positive")
		}
		if id == sourceID {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	if len(ret) == 0 {
		return nil, fmt.Errorf("select at least one target image other than the source")
	}
	sort.Ints(ret)
	return ret, nil
}

func loadSimilarityMetadataSnapshot(ctx context.Context, repository *models.Repository, imageID int) (similarityMetadataSnapshot, error) {
	var snapshot similarityMetadataSnapshot
	err := repository.WithReadTxn(ctx, func(ctx context.Context) error {
		image, err := repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("image not found")
		}

		performerIDs, err := repository.Image.GetPerformerIDs(ctx, imageID)
		if err != nil {
			return err
		}
		tagIDs, err := repository.Image.GetTagIDs(ctx, imageID)
		if err != nil {
			return err
		}
		artists, err := repository.ImageArtist.FindByImageID(ctx, imageID)
		if err != nil {
			return err
		}
		copyrights, err := repository.Copyright.FindByImageID(ctx, imageID)
		if err != nil {
			return err
		}

		snapshot.performerIDs = positiveUniqueIDs(performerIDs)
		snapshot.tagIDs = positiveUniqueIDs(tagIDs)
		snapshot.artistIDs = make([]int, 0, len(artists))
		for _, artist := range artists {
			if artist != nil {
				snapshot.artistIDs = append(snapshot.artistIDs, artist.ID)
			}
		}
		snapshot.artistIDs = positiveUniqueIDs(snapshot.artistIDs)
		snapshot.copyrightIDs = make([]int, 0, len(copyrights))
		for _, copyright := range copyrights {
			if copyright != nil {
				snapshot.copyrightIDs = append(snapshot.copyrightIDs, copyright.ID)
			}
		}
		snapshot.copyrightIDs = positiveUniqueIDs(snapshot.copyrightIDs)
		return nil
	})
	return snapshot, err
}

func copySimilarityMetadataToImage(ctx context.Context, repository *models.Repository, imageID int, source similarityMetadataSnapshot) (similarityMetadataCopyCounts, error) {
	var added similarityMetadataCopyCounts
	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		image, err := repository.Image.Find(ctx, imageID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("image %d not found", imageID)
		}

		performerIDs, err := repository.Image.GetPerformerIDs(ctx, imageID)
		if err != nil {
			return err
		}
		tagIDs, err := repository.Image.GetTagIDs(ctx, imageID)
		if err != nil {
			return err
		}
		artists, err := repository.ImageArtist.FindByImageID(ctx, imageID)
		if err != nil {
			return err
		}
		copyrights, err := repository.Copyright.FindByImageID(ctx, imageID)
		if err != nil {
			return err
		}

		existingArtistIDs := make([]int, 0, len(artists))
		for _, artist := range artists {
			if artist != nil {
				existingArtistIDs = append(existingArtistIDs, artist.ID)
			}
		}
		existingCopyrightIDs := make([]int, 0, len(copyrights))
		for _, copyright := range copyrights {
			if copyright != nil {
				existingCopyrightIDs = append(existingCopyrightIDs, copyright.ID)
			}
		}

		missingPerformers := missingPositiveIDs(performerIDs, source.performerIDs)
		missingTags := missingPositiveIDs(tagIDs, source.tagIDs)
		missingArtists := missingPositiveIDs(existingArtistIDs, source.artistIDs)
		missingCopyrights := missingPositiveIDs(existingCopyrightIDs, source.copyrightIDs)

		if len(missingPerformers) > 0 || len(missingTags) > 0 {
			partial := models.NewImagePartial()
			if len(missingPerformers) > 0 {
				partial.PerformerIDs = &models.UpdateIDs{IDs: missingPerformers, Mode: models.RelationshipUpdateModeAdd}
			}
			if len(missingTags) > 0 {
				partial.TagIDs = &models.UpdateIDs{IDs: missingTags, Mode: models.RelationshipUpdateModeAdd}
			}
			if _, err := repository.Image.UpdatePartial(ctx, imageID, partial); err != nil {
				return err
			}
		}
		if len(missingArtists) > 0 {
			if err := repository.ImageArtist.AddImageArtists(ctx, imageID, missingArtists); err != nil {
				return err
			}
		}
		if len(missingCopyrights) > 0 {
			if err := repository.Copyright.AddImageCopyrights(ctx, imageID, missingCopyrights); err != nil {
				return err
			}
		}

		added = similarityMetadataCopyCounts{
			Characters: len(missingPerformers),
			Artists:    len(missingArtists),
			Copyrights: len(missingCopyrights),
			Tags:       len(missingTags),
		}
		return nil
	})
	return added, err
}

func positiveUniqueIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	ret := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret
}

func missingPositiveIDs(existing, source []int) []int {
	seen := make(map[int]struct{}, len(existing)+len(source))
	for _, id := range existing {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	ret := make([]int, 0, len(source))
	for _, id := range source {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret
}
