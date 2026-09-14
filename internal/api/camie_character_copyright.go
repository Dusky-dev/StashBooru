package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type camieCopyrightContext struct {
	names map[string]struct{}
	ids   map[int]struct{}
}

func newCamieCopyrightContext() camieCopyrightContext {
	return camieCopyrightContext{
		names: make(map[string]struct{}),
		ids:   make(map[int]struct{}),
	}
}

func camieCopyrightLookupKey(value string) string {
	value = canonicalCamieName(value, "copyright")
	return strings.ToLower(strings.TrimSpace(value))
}

func (c *camieCopyrightContext) addName(value string) {
	if key := camieCopyrightLookupKey(value); key != "" {
		c.names[key] = struct{}{}
	}
}

func (c *camieCopyrightContext) addCopyright(entity *models.Copyright) {
	if entity == nil {
		return
	}
	c.ids[entity.ID] = struct{}{}
	c.addName(entity.Name)
	for _, alias := range entity.Aliases {
		c.addName(alias)
	}
}

func (c camieCopyrightContext) matchesName(value string) bool {
	_, ok := c.names[camieCopyrightLookupKey(value)]
	return ok
}

func (c camieCopyrightContext) empty() bool {
	return len(c.names) == 0 && len(c.ids) == 0
}

func camieCopyrightSourceTrustedForCharacterIdentity(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		// Preserve compatibility with explicit/manual callers that predate
		// source provenance. Current model endpoints always identify themselves.
		return true
	}
	for _, part := range strings.Split(source, "+") {
		part = strings.TrimSpace(part)
		if part == "filename" || part == "local" || part == "existing" || strings.HasPrefix(part, "booru:") {
			return true
		}
	}
	return false
}

func buildCamieCopyrightContext(ctx context.Context, repository models.Repository, predictions []camietagger.Tag, extraName string) (camieCopyrightContext, error) {
	result := newCamieCopyrightContext()
	if strings.TrimSpace(extraName) != "" {
		result.addName(extraName)
		entity, err := findNativeCamieCopyright(ctx, repository, camietagger.Tag{
			Name:     extraName,
			RawName:  extraName,
			Category: "copyright",
		})
		if err != nil {
			return result, err
		}
		result.addCopyright(entity)
	}

	for _, rawPrediction := range predictions {
		prediction := normalizeCamiePrediction(rawPrediction)
		if prediction.Category != "copyright" || !camieCopyrightSourceTrustedForCharacterIdentity(rawPrediction.Source) {
			continue
		}
		result.addName(prediction.Name)
		result.addName(prediction.RawName)
		entity, err := findNativeCamieCopyright(ctx, repository, prediction)
		if err != nil {
			return result, err
		}
		result.addCopyright(entity)
	}
	return result, nil
}

func camiePerformerMatchesCopyrightContext(ctx context.Context, repository models.Repository, performerEntity *models.Performer, copyrightContext camieCopyrightContext) (bool, error) {
	if performerEntity == nil || copyrightContext.empty() {
		return false, nil
	}

	if copyrightContext.matchesName(performerEntity.Disambiguation) {
		return true, nil
	}

	aliases, err := repository.Performer.GetAliases(ctx, performerEntity.ID)
	if err != nil {
		return false, err
	}
	for _, alias := range aliases {
		_, aliasDisambiguation := camieCharacterIdentity(camietagger.Tag{
			Name:     alias,
			RawName:  alias,
			Category: "character",
		})
		if aliasDisambiguation != "" && copyrightContext.matchesName(aliasDisambiguation) {
			return true, nil
		}
	}

	copyrights, err := repository.Copyright.FindByPerformerID(ctx, performerEntity.ID)
	if err != nil {
		return false, err
	}
	for _, copyrightEntity := range copyrights {
		if copyrightEntity == nil {
			continue
		}
		if _, ok := copyrightContext.ids[copyrightEntity.ID]; ok {
			return true, nil
		}
		if copyrightContext.matchesName(copyrightEntity.Name) {
			return true, nil
		}
		for _, alias := range copyrightEntity.Aliases {
			if copyrightContext.matchesName(alias) {
				return true, nil
			}
		}
	}
	return false, nil
}

func camieSameNamePerformers(ctx context.Context, repository models.Repository, characterName string) ([]*models.Performer, error) {
	matches, err := repository.Performer.FindByNames(ctx, []string{characterName}, true)
	if err != nil {
		return nil, err
	}
	result := make([]*models.Performer, 0, len(matches))
	seen := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		if match == nil || !strings.EqualFold(strings.TrimSpace(match.Name), strings.TrimSpace(characterName)) {
			continue
		}
		if _, ok := seen[match.ID]; ok {
			continue
		}
		seen[match.ID] = struct{}{}
		result = append(result, match)
	}
	return result, nil
}

// findCamieExplicitCharacterTarget honors an exact Character target selected in
// the review UI. The target ID is accepted only when it still resolves to the
// same canonical Character name, so a stale or manipulated target cannot turn
// one Character prediction into an unrelated Character.
func findCamieExplicitCharacterTarget(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, error) {
	prediction = normalizeCamiePrediction(prediction)
	if prediction.Category != "character" || !prediction.TargetExists {
		return nil, nil
	}

	targetMatch := camiePerformerTargetPattern.FindStringSubmatch(strings.TrimSpace(prediction.TargetPath))
	if len(targetMatch) != 2 {
		return nil, nil
	}
	targetID, err := strconv.Atoi(targetMatch[1])
	if err != nil {
		return nil, nil
	}
	target, err := repository.Performer.Find(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, nil
	}

	characterName, _ := camieCharacterIdentity(prediction)
	if !strings.EqualFold(strings.TrimSpace(target.Name), strings.TrimSpace(characterName)) {
		return nil, nil
	}
	return target, nil
}

func camieAmbiguousCharacterError(characterName string, matches []*models.Performer) error {
	labels := make([]string, 0, len(matches))
	for _, match := range matches {
		if match == nil {
			continue
		}
		label := strings.TrimSpace(match.Name)
		if disambiguation := strings.TrimSpace(match.Disambiguation); disambiguation != "" {
			label = fmt.Sprintf("%s (%s)", label, disambiguation)
		}
		labels = append(labels, label)
	}
	return fmt.Errorf("character %q matches multiple existing Characters (%s); select/provide a Copyright (series) or use a disambiguated Character name", characterName, strings.Join(labels, ", "))
}

// findCamiePerformerPredictionWithCopyrightContext makes same-name Character
// resolution deterministic. An exact Character disambiguation wins first. If
// the source only provides a bare name, native Copyright relations,
// disambiguation values, and disambiguated aliases are used to pick a single
// same-name Character. We never fall back to the first database row.
func findCamiePerformerPredictionWithCopyrightContext(ctx context.Context, repository models.Repository, prediction camietagger.Tag, predictions []camietagger.Tag) (*models.Performer, error) {
	prediction = normalizeCamiePrediction(prediction)
	characterName, disambiguation := camieCharacterIdentity(prediction)

	explicitTarget, err := findCamieExplicitCharacterTarget(ctx, repository, prediction)
	if err != nil {
		return nil, err
	}
	if explicitTarget != nil {
		return explicitTarget, nil
	}

	matches, err := camieSameNamePerformers(ctx, repository, characterName)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return findCamiePerformerPrediction(ctx, repository, prediction)
	}

	if disambiguation != "" {
		exact := make([]*models.Performer, 0, 1)
		for _, match := range matches {
			if strings.EqualFold(strings.TrimSpace(match.Disambiguation), strings.TrimSpace(disambiguation)) {
				exact = append(exact, match)
			}
		}
		if len(exact) == 1 {
			return exact[0], nil
		}
		if len(exact) > 1 {
			return nil, camieAmbiguousCharacterError(characterName, exact)
		}
	}

	copyrightContext, err := buildCamieCopyrightContext(ctx, repository, predictions, disambiguation)
	if err != nil {
		return nil, err
	}
	contextMatches := make([]*models.Performer, 0, len(matches))
	if !copyrightContext.empty() {
		for _, match := range matches {
			matched, err := camiePerformerMatchesCopyrightContext(ctx, repository, match, copyrightContext)
			if err != nil {
				return nil, err
			}
			if matched {
				contextMatches = append(contextMatches, match)
			}
		}
		if len(contextMatches) == 1 {
			return contextMatches[0], nil
		}
		if len(contextMatches) > 1 {
			return nil, camieAmbiguousCharacterError(characterName, contextMatches)
		}
	}

	if disambiguation != "" {
		// A bare same-name Character remains a review suggestion rather than an
		// automatic match for a disambiguated prediction. The existing resolver
		// keeps explicit target confirmation and alias matching intact.
		return findCamiePerformerPrediction(ctx, repository, prediction)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, camieAmbiguousCharacterError(characterName, matches)
}

func findOrCreateCamiePerformerPredictionWithCopyrightContext(ctx context.Context, repository models.Repository, prediction camietagger.Tag, predictions []camietagger.Tag) (*models.Performer, bool, error) {
	existing, err := findCamiePerformerPredictionWithCopyrightContext(ctx, repository, prediction, predictions)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	return findOrCreateCamiePerformerPrediction(ctx, repository, prediction)
}

func camieCopyrightMatchesValue(entity *models.Copyright, value string) bool {
	if entity == nil || strings.TrimSpace(value) == "" {
		return false
	}
	key := camieCopyrightLookupKey(value)
	if camieCopyrightLookupKey(entity.Name) == key {
		return true
	}
	for _, alias := range entity.Aliases {
		if camieCopyrightLookupKey(alias) == key {
			return true
		}
	}
	return false
}

func findCamieCopyrightForCharacter(ctx context.Context, repository models.Repository, prediction camietagger.Tag, performerEntity *models.Performer, selected []*models.Copyright) (*models.Copyright, error) {
	_, disambiguation := camieCharacterIdentity(prediction)
	if disambiguation == "" && performerEntity != nil {
		disambiguation = strings.TrimSpace(performerEntity.Disambiguation)
	}
	if disambiguation != "" {
		var matched *models.Copyright
		for _, copyrightEntity := range selected {
			if !camieCopyrightMatchesValue(copyrightEntity, disambiguation) {
				continue
			}
			if matched != nil && matched.ID != copyrightEntity.ID {
				return nil, nil
			}
			matched = copyrightEntity
		}
		if matched != nil {
			return matched, nil
		}

		// An explicit/reliable Character disambiguation outranks a generic
		// single-Copyright assumption. It may safely attach to an already
		// existing Copyright through its canonical name or alias.
		existing, err := findNativeCamieCopyright(ctx, repository, camietagger.Tag{
			Name:     disambiguation,
			RawName:  disambiguation,
			Category: "copyright",
		})
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}

		// Do not attach a conflicting sole Copyright (for example Fire Emblem)
		// to an explicitly disambiguated Character such as Lana (Pokemon).
		return nil, nil
	}

	if len(selected) == 1 {
		return selected[0], nil
	}
	return nil, nil
}

// linkCamieCharacterCopyrights persists the series context learned while
// tagging. One selected Copyright is safe for Characters without their own
// series disambiguation. With crossover/multi-Copyright metadata, or when a
// Character already identifies its series, only a matching Copyright is linked.
func linkCamieCharacterCopyrights(ctx context.Context, repository models.Repository, characterPredictions []camietagger.Tag, performerIDs, copyrightIDs []int) error {
	if len(characterPredictions) == 0 || len(performerIDs) == 0 {
		return nil
	}

	selected := []*models.Copyright{}
	if len(copyrightIDs) > 0 {
		entities, err := repository.Copyright.FindMany(ctx, copyrightIDs)
		if err != nil {
			return err
		}
		selected = entities
	}

	limit := len(characterPredictions)
	if len(performerIDs) < limit {
		limit = len(performerIDs)
	}
	for index := 0; index < limit; index++ {
		performerEntity, err := repository.Performer.Find(ctx, performerIDs[index])
		if err != nil {
			return err
		}
		if performerEntity == nil {
			continue
		}
		copyrightEntity, err := findCamieCopyrightForCharacter(ctx, repository, characterPredictions[index], performerEntity, selected)
		if err != nil {
			return err
		}
		if copyrightEntity == nil {
			continue
		}

		existing, err := repository.Copyright.FindByPerformerID(ctx, performerEntity.ID)
		if err != nil {
			return err
		}
		alreadyLinked := false
		for _, current := range existing {
			if current != nil && current.ID == copyrightEntity.ID {
				alreadyLinked = true
				break
			}
		}
		if alreadyLinked {
			continue
		}
		if err := repository.Copyright.AddPerformerCopyrights(ctx, performerEntity.ID, []int{copyrightEntity.ID}); err != nil {
			return err
		}
	}
	return nil
}
