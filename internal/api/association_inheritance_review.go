package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/models"
)

const (
	associationInheritancePageSize    = 100
	associationInheritanceSampleLimit = 20
	associationInheritanceMemberLimit = 32
)

type associationInheritanceDomainSummary struct {
	BeforeInherited int `json:"beforeInherited"`
	AfterInherited  int `json:"afterInherited"`
	Added           int `json:"added"`
	Removed         int `json:"removed"`
}

type associationInheritanceSample struct {
	MediaType string                       `json:"mediaType"`
	MediaID   int                          `json:"mediaID"`
	Label     string                       `json:"label"`
	Before    []MediaAssociationProvenance `json:"before"`
	After     []MediaAssociationProvenance `json:"after"`
	Truncated bool                         `json:"truncated"`
}

type associationInheritanceReport struct {
	Current        config.AssociationInheritanceSettings          `json:"current"`
	Proposed       config.AssociationInheritanceSettings          `json:"proposed"`
	ImagesReviewed int                                            `json:"imagesReviewed"`
	VideosReviewed int                                            `json:"videosReviewed"`
	AffectedMedia  int                                            `json:"affectedMedia"`
	ErrorCount     int                                            `json:"errorCount"`
	ErrorSamples   []string                                       `json:"errorSamples"`
	Domains        map[string]associationInheritanceDomainSummary `json:"domains"`
	Samples        []associationInheritanceSample                 `json:"samples"`
	Fingerprint    string                                         `json:"-"`
}

// scanAssociationInheritanceReview must run in a caller-owned transaction.
// Every typed media ID and its complete provenance enters the fingerprint;
// only a bounded sample is retained for display.
func scanAssociationInheritanceReview(
	ctx context.Context,
	repository models.Repository,
	current, proposed config.AssociationInheritanceSettings,
	progress func(processed, total int),
) (*associationInheritanceReport, error) {
	report := &associationInheritanceReport{
		Current: current, Proposed: proposed,
		ErrorSamples: []string{},
		Domains:      make(map[string]associationInheritanceDomainSummary),
		Samples:      []associationInheritanceSample{},
	}
	for _, domain := range []string{"character", "artist", "copyright", "tag"} {
		report.Domains[domain] = associationInheritanceDomainSummary{}
	}
	images, err := repository.Image.Count(ctx)
	if err != nil {
		return nil, err
	}
	videos, err := repository.Scene.Count(ctx)
	if err != nil {
		return nil, err
	}
	total := images + videos
	processed := 0
	if progress != nil {
		progress(processed, total)
	}
	hasher := sha256.New()
	encoder := json.NewEncoder(hasher)
	header := struct {
		Version  int
		Current  config.AssociationInheritanceSettings
		Proposed config.AssociationInheritanceSettings
	}{Version: 1, Current: current, Proposed: proposed}
	if err := encoder.Encode(header); err != nil {
		return nil, err
	}
	review := func(mediaType string, mediaID int, label string, direct directMediaAssociationIDs) error {
		_, before, err := effectiveAssociationProvenance(ctx, repository, direct, current)
		if err != nil {
			return err
		}
		_, after, err := effectiveAssociationProvenance(ctx, repository, direct, proposed)
		if err != nil {
			return err
		}
		item := struct {
			MediaType string
			MediaID   int
			Before    []MediaAssociationProvenance
			After     []MediaAssociationProvenance
		}{MediaType: mediaType, MediaID: mediaID, Before: before, After: after}
		if err := encoder.Encode(item); err != nil {
			return err
		}
		changed := summarizeAssociationInheritance(report.Domains, before, after)
		if changed {
			report.AffectedMedia++
		}
		if len(report.Samples) < associationInheritanceSampleLimit && (changed || hasInheritedAssociations(after)) {
			sample := associationInheritanceSample{MediaType: mediaType, MediaID: mediaID, Label: label, Before: before, After: after}
			if len(sample.Before) > associationInheritanceMemberLimit {
				sample.Before = sample.Before[:associationInheritanceMemberLimit]
				sample.Truncated = true
			}
			if len(sample.After) > associationInheritanceMemberLimit {
				sample.After = sample.After[:associationInheritanceMemberLimit]
				sample.Truncated = true
			}
			report.Samples = append(report.Samples, sample)
		}
		return nil
	}
	finishItem := func(mediaType string, id int, itemErr error) {
		if itemErr != nil {
			report.ErrorCount++
			if len(report.ErrorSamples) < associationInheritanceSampleLimit {
				report.ErrorSamples = append(report.ErrorSamples, fmt.Sprintf("%s %d: %v", mediaType, id, itemErr))
			}
		}
		processed++
		if progress != nil {
			progress(processed, total)
		}
	}

	pageSize := associationInheritancePageSize
	sortBy := "id"
	findFilter := &models.FindFilterType{PerPage: &pageSize, Sort: &sortBy}
	lastID := 0
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		options := models.ImageQueryOptions{ImageFilter: &models.ImageFilterType{
			ID: &models.IntCriterionInput{Value: lastID, Modifier: models.CriterionModifierGreaterThan},
		}}
		options.FindFilter = findFilter
		result, err := repository.Image.Query(ctx, options)
		if err != nil {
			return report, err
		}
		if len(result.IDs) == 0 {
			break
		}
		for _, id := range result.IDs {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			lastID = id
			report.ImagesReviewed++
			image, itemErr := repository.Image.Find(ctx, id)
			if itemErr == nil && image == nil {
				itemErr = errors.New("image no longer exists")
			}
			if itemErr == nil {
				var direct directMediaAssociationIDs
				direct, itemErr = imageDirectMediaAssociations(ctx, repository, image)
				if itemErr == nil {
					itemErr = review("image", id, image.GetTitle(), direct)
				}
			}
			finishItem("image", id, itemErr)
		}
	}
	lastID = 0
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		options := models.SceneQueryOptions{SceneFilter: &models.SceneFilterType{
			ID: &models.IntCriterionInput{Value: lastID, Modifier: models.CriterionModifierGreaterThan},
		}}
		options.FindFilter = findFilter
		result, err := repository.Scene.Query(ctx, options)
		if err != nil {
			return report, err
		}
		if len(result.IDs) == 0 {
			break
		}
		for _, id := range result.IDs {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			lastID = id
			report.VideosReviewed++
			scene, itemErr := repository.Scene.Find(ctx, id)
			if itemErr == nil && scene == nil {
				itemErr = errors.New("video no longer exists")
			}
			if itemErr == nil {
				var direct directMediaAssociationIDs
				direct, itemErr = sceneDirectMediaAssociations(ctx, repository, scene)
				if itemErr == nil {
					itemErr = review("video", id, scene.GetTitle(), direct)
				}
			}
			finishItem("video", id, itemErr)
		}
	}
	report.Fingerprint = hex.EncodeToString(hasher.Sum(nil))
	return report, ctx.Err()
}

func hasInheritedAssociations(provenance []MediaAssociationProvenance) bool {
	for _, membership := range provenance {
		for _, origin := range membership.Origins {
			if origin.Kind != "direct" {
				return true
			}
		}
	}
	return false
}

func summarizeAssociationInheritance(
	domains map[string]associationInheritanceDomainSummary,
	before, after []MediaAssociationProvenance,
) bool {
	beforeByID := make(map[mediaAssociationKey]MediaAssociationProvenance, len(before))
	afterByID := make(map[mediaAssociationKey]MediaAssociationProvenance, len(after))
	for _, item := range before {
		beforeByID[mediaAssociationKey{associationType: item.AssociationType, associationID: item.AssociationID}] = item
		summary := domains[item.AssociationType]
		if hasInheritedAssociations([]MediaAssociationProvenance{item}) {
			summary.BeforeInherited++
		}
		domains[item.AssociationType] = summary
	}
	for _, item := range after {
		afterByID[mediaAssociationKey{associationType: item.AssociationType, associationID: item.AssociationID}] = item
		summary := domains[item.AssociationType]
		if hasInheritedAssociations([]MediaAssociationProvenance{item}) {
			summary.AfterInherited++
		}
		domains[item.AssociationType] = summary
	}
	changed := false
	for key, item := range afterByID {
		previous, exists := beforeByID[key]
		if !exists {
			summary := domains[key.associationType]
			summary.Added++
			domains[key.associationType] = summary
			changed = true
			continue
		}
		if len(item.Origins) != len(previous.Origins) {
			changed = true
			continue
		}
		for index, origin := range item.Origins {
			if origin != previous.Origins[index] {
				changed = true
				break
			}
		}
	}
	for key := range beforeByID {
		if _, exists := afterByID[key]; !exists {
			summary := domains[key.associationType]
			summary.Removed++
			domains[key.associationType] = summary
			changed = true
		}
	}
	return changed
}

func applyAssociationInheritanceReview(
	ctx context.Context,
	repository models.Repository,
	cfg *config.Config,
	review *associationInheritanceReport,
	progress func(processed, total int),
) error {
	if review == nil || review.Fingerprint == "" || review.ErrorCount != 0 {
		return errors.New("a successful inheritance preview is required")
	}
	if cfg.GetAssociationInheritanceSettings() != review.Current {
		return errors.New("inheritance defaults changed since preview; run a new preview")
	}
	saved := false
	err := repository.WithTxn(ctx, func(ctx context.Context) error {
		// The write transaction prevents direct links, profiles and hierarchies
		// changing between validation and activation. It does not write media.
		fresh, err := scanAssociationInheritanceReview(ctx, repository, review.Current, review.Proposed, progress)
		if err != nil {
			return err
		}
		if fresh.ErrorCount != 0 || fresh.Fingerprint != review.Fingerprint {
			return errors.New("media associations or profiles changed since preview; run a new preview")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cfg.ApplyAssociationInheritanceSettings(review.Current, review.Proposed); err != nil {
			return err
		}
		saved = true
		return nil
	})
	// No native rows were changed. A successful atomic configuration save
	// remains activated if cancellation arrives during transaction release.
	if saved {
		return nil
	}
	return err
}
