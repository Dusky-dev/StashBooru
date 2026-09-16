package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

const (
	metadataHealthSameNameCharacterContext = "same-name-character-context"
	metadataHealthOrphanCharacterRelation  = "orphan-character-relation"
	metadataHealthOrphanTagRelation        = "orphan-tag-relation"
	metadataHealthOrphanLegacyArtist       = "orphan-legacy-artist"
	metadataHealthTagMigrationAmbiguous    = "legacy-tag-migration-ambiguous"
)

type metadataHealthCharacterContext struct {
	ID           int
	Name         string
	CopyrightIDs []int
}

type metadataHealthMediaRelations struct {
	MediaKind    string
	MediaID      int
	CharacterIDs []int
	TagIDs       []int
	LegacyArtist *int
}

type metadataHealthMigrationEntity struct {
	ID      int
	Kind    string
	Name    string
	Aliases []string
}

type metadataHealthLegacyTag struct {
	ID      int
	Name    string
	Aliases []string
}

func findSameNameCharacterContext(characters []metadataHealthCharacterContext) []metadataHealthFinding {
	buckets := make(map[string][]metadataHealthCharacterContext)
	for _, character := range characters {
		name := normalizeMetadataHealthName(character.Name)
		if character.ID <= 0 || name == "" {
			continue
		}
		character.CopyrightIDs = metadataHealthSortedUniquePositiveIDs(character.CopyrightIDs)
		buckets[name] = append(buckets[name], character)
	}

	findings := make([]metadataHealthFinding, 0)
	for name, matches := range buckets {
		if len(matches) < 2 {
			continue
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
		ids := make([]int, 0, len(matches))
		parts := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.ID)
			context := "none"
			if len(match.CopyrightIDs) > 0 {
				context = fmt.Sprint(match.CopyrightIDs)
			}
			parts = append(parts, fmt.Sprintf("Character #%d Copyrights %s", match.ID, context))
		}
		findings = append(findings, metadataHealthFinding{
			Code:      metadataHealthSameNameCharacterContext,
			Kind:      "character",
			Value:     name,
			EntityIDs: ids,
			Detail:    strings.Join(parts, "; "),
		})
	}
	sortMetadataHealthFindings(findings)
	return findings
}

func findOrphanMetadataRelations(relations []metadataHealthMediaRelations, knownCharacters, knownTags, knownArtists map[int]struct{}) []metadataHealthFinding {
	findings := make([]metadataHealthFinding, 0)
	for _, relation := range relations {
		kind := strings.ToLower(strings.TrimSpace(relation.MediaKind))
		if relation.MediaID <= 0 || (kind != "image" && kind != "video") {
			continue
		}

		missingCharacters := metadataHealthMissingIDs(relation.CharacterIDs, knownCharacters)
		if len(missingCharacters) > 0 {
			findings = append(findings, metadataHealthFinding{
				Code:      metadataHealthOrphanCharacterRelation,
				Kind:      kind,
				MediaID:   relation.MediaID,
				EntityIDs: missingCharacters,
				Detail:    "relationship references Character IDs that no longer exist",
			})
		}

		missingTags := metadataHealthMissingIDs(relation.TagIDs, knownTags)
		if len(missingTags) > 0 {
			findings = append(findings, metadataHealthFinding{
				Code:      metadataHealthOrphanTagRelation,
				Kind:      kind,
				MediaID:   relation.MediaID,
				EntityIDs: missingTags,
				Detail:    "relationship references Tag IDs that no longer exist",
			})
		}

		if relation.LegacyArtist != nil && *relation.LegacyArtist > 0 {
			if _, ok := knownArtists[*relation.LegacyArtist]; !ok {
				findings = append(findings, metadataHealthFinding{
					Code:      metadataHealthOrphanLegacyArtist,
					Kind:      kind,
					MediaID:   relation.MediaID,
					EntityIDs: []int{*relation.LegacyArtist},
					Detail:    "legacy StudioID references an Artist that no longer exists",
				})
			}
		}
	}
	sortMetadataHealthFindings(findings)
	return findings
}

func findLegacyTagMigrationAmbiguity(tags []metadataHealthLegacyTag, targets []metadataHealthMigrationEntity) []metadataHealthFinding {
	type targetRef struct {
		kind string
		id   int
		name string
	}
	byValue := make(map[string]map[string]targetRef)
	for _, target := range targets {
		kind := strings.ToLower(strings.TrimSpace(target.Kind))
		if target.ID <= 0 || kind == "" {
			continue
		}
		values := append([]string{target.Name}, target.Aliases...)
		for _, raw := range values {
			value := normalizeMetadataHealthName(raw)
			if value == "" {
				continue
			}
			if byValue[value] == nil {
				byValue[value] = make(map[string]targetRef)
			}
			key := fmt.Sprintf("%s:%d", kind, target.ID)
			byValue[value][key] = targetRef{kind: kind, id: target.ID, name: target.Name}
		}
	}

	findings := make([]metadataHealthFinding, 0)
	seen := make(map[string]struct{})
	for _, tag := range tags {
		if tag.ID <= 0 {
			continue
		}
		values := append([]string{tag.Name}, tag.Aliases...)
		for _, raw := range values {
			value := normalizeMetadataHealthName(raw)
			refs := byValue[value]
			if value == "" || len(refs) < 2 {
				continue
			}
			findingKey := fmt.Sprintf("%d:%s", tag.ID, value)
			if _, ok := seen[findingKey]; ok {
				continue
			}
			seen[findingKey] = struct{}{}

			candidates := make([]targetRef, 0, len(refs))
			for _, ref := range refs {
				candidates = append(candidates, ref)
			}
			sort.Slice(candidates, func(i, j int) bool {
				if candidates[i].kind != candidates[j].kind {
					return candidates[i].kind < candidates[j].kind
				}
				return candidates[i].id < candidates[j].id
			})
			parts := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				parts = append(parts, fmt.Sprintf("%s #%d %q", candidate.kind, candidate.id, candidate.name))
			}
			findings = append(findings, metadataHealthFinding{
				Code:      metadataHealthTagMigrationAmbiguous,
				Kind:      "tag-migration",
				Value:     value,
				EntityIDs: []int{tag.ID},
				Detail:    fmt.Sprintf("legacy Tag #%d %q could resolve to multiple native targets: %s", tag.ID, tag.Name, strings.Join(parts, ", ")),
			})
		}
	}
	sortMetadataHealthFindings(findings)
	return findings
}

func metadataHealthSortedUniquePositiveIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	ret := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	for id := range seen {
		ret = append(ret, id)
	}
	sort.Ints(ret)
	return ret
}

func metadataHealthMissingIDs(ids []int, known map[int]struct{}) []int {
	missing := make([]int, 0)
	for _, id := range metadataHealthSortedUniquePositiveIDs(ids) {
		if _, ok := known[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func collectMetadataHealthExtended(ctx context.Context, repository models.Repository) ([]metadataHealthFinding, error) {
	performers, err := repository.Performer.All(ctx)
	if err != nil {
		return nil, err
	}
	studios, err := repository.Studio.All(ctx)
	if err != nil {
		return nil, err
	}
	tags, err := repository.Tag.All(ctx)
	if err != nil {
		return nil, err
	}
	copyrights, _, err := repository.Copyright.Query(ctx, nil)
	if err != nil {
		return nil, err
	}

	knownCharacters := make(map[int]struct{}, len(performers))
	knownArtists := make(map[int]struct{}, len(studios))
	knownTags := make(map[int]struct{}, len(tags))
	characterContexts := make([]metadataHealthCharacterContext, 0, len(performers))
	migrationTargets := make([]metadataHealthMigrationEntity, 0, len(performers)+len(studios)+len(copyrights))
	legacyTags := make([]metadataHealthLegacyTag, 0, len(tags))

	for _, performer := range performers {
		if performer == nil || performer.ID <= 0 {
			continue
		}
		knownCharacters[performer.ID] = struct{}{}
		if err := performer.LoadAliases(ctx, repository.Performer); err != nil {
			return nil, fmt.Errorf("loading Character %d aliases: %w", performer.ID, err)
		}
		copyrightList, err := repository.Copyright.FindByPerformerID(ctx, performer.ID)
		if err != nil {
			return nil, fmt.Errorf("loading Character %d Copyrights: %w", performer.ID, err)
		}
		copyrightIDs := make([]int, 0, len(copyrightList))
		for _, copyright := range copyrightList {
			if copyright != nil {
				copyrightIDs = append(copyrightIDs, copyright.ID)
			}
		}
		characterContexts = append(characterContexts, metadataHealthCharacterContext{
			ID: performer.ID, Name: performer.Name, CopyrightIDs: copyrightIDs,
		})
		migrationTargets = append(migrationTargets, metadataHealthMigrationEntity{
			ID: performer.ID, Kind: "character", Name: performer.Name, Aliases: append([]string(nil), performer.Aliases.List()...),
		})
	}
	for _, studio := range studios {
		if studio == nil || studio.ID <= 0 {
			continue
		}
		knownArtists[studio.ID] = struct{}{}
		if err := studio.LoadAliases(ctx, repository.Studio); err != nil {
			return nil, fmt.Errorf("loading Artist %d aliases: %w", studio.ID, err)
		}
		migrationTargets = append(migrationTargets, metadataHealthMigrationEntity{
			ID: studio.ID, Kind: "artist", Name: studio.Name, Aliases: append([]string(nil), studio.Aliases.List()...),
		})
	}
	for _, copyright := range copyrights {
		if copyright == nil || copyright.ID <= 0 {
			continue
		}
		migrationTargets = append(migrationTargets, metadataHealthMigrationEntity{
			ID: copyright.ID, Kind: "copyright", Name: copyright.Name, Aliases: append([]string(nil), copyright.Aliases...),
		})
	}
	for _, tag := range tags {
		if tag == nil || tag.ID <= 0 {
			continue
		}
		knownTags[tag.ID] = struct{}{}
		if err := tag.LoadAliases(ctx, repository.Tag); err != nil {
			return nil, fmt.Errorf("loading Tag %d aliases: %w", tag.ID, err)
		}
		legacyTags = append(legacyTags, metadataHealthLegacyTag{
			ID: tag.ID, Name: tag.Name, Aliases: append([]string(nil), tag.Aliases.List()...),
		})
	}

	findings := findSameNameCharacterContext(characterContexts)
	findings = append(findings, findLegacyTagMigrationAmbiguity(legacyTags, migrationTargets)...)

	images, err := repository.Image.All(ctx)
	if err != nil {
		return nil, err
	}
	scenes, err := repository.Scene.All(ctx)
	if err != nil {
		return nil, err
	}
	relations := make([]metadataHealthMediaRelations, 0, len(images)+len(scenes))
	for _, image := range images {
		if image == nil {
			continue
		}
		characterIDs, err := repository.Image.GetPerformerIDs(ctx, image.ID)
		if err != nil {
			return nil, fmt.Errorf("loading Image %d Characters: %w", image.ID, err)
		}
		tagIDs, err := repository.Image.GetTagIDs(ctx, image.ID)
		if err != nil {
			return nil, fmt.Errorf("loading Image %d Tags: %w", image.ID, err)
		}
		relations = append(relations, metadataHealthMediaRelations{
			MediaKind: "image", MediaID: image.ID, CharacterIDs: characterIDs, TagIDs: tagIDs, LegacyArtist: image.StudioID,
		})
	}
	for _, scene := range scenes {
		if scene == nil {
			continue
		}
		characterIDs, err := repository.Scene.GetPerformerIDs(ctx, scene.ID)
		if err != nil {
			return nil, fmt.Errorf("loading Video %d Characters: %w", scene.ID, err)
		}
		tagIDs, err := repository.Scene.GetTagIDs(ctx, scene.ID)
		if err != nil {
			return nil, fmt.Errorf("loading Video %d Tags: %w", scene.ID, err)
		}
		relations = append(relations, metadataHealthMediaRelations{
			MediaKind: "video", MediaID: scene.ID, CharacterIDs: characterIDs, TagIDs: tagIDs, LegacyArtist: scene.StudioID,
		})
	}
	findings = append(findings, findOrphanMetadataRelations(relations, knownCharacters, knownTags, knownArtists)...)
	sortMetadataHealthFindings(findings)
	return findings, nil
}

func (rs imageRoutes) MetadataHealthComplete(w http.ResponseWriter, r *http.Request) {
	repository := manager.GetInstance().Repository
	var response metadataHealthResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		base, err := collectMetadataHealth(ctx, repository)
		if err != nil {
			return err
		}
		extended, err := collectMetadataHealthExtended(ctx, repository)
		if err != nil {
			return err
		}
		base.Findings = append(base.Findings, extended...)
		sortMetadataHealthFindings(base.Findings)
		base.Counts = make(map[string]int)
		for _, finding := range base.Findings {
			base.Counts[finding.Code]++
		}
		response = base
		return nil
	}); err != nil {
		http.Error(w, fmt.Sprintf("scanning metadata health: %v", err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
