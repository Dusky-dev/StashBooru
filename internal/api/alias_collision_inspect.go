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

type aliasInspectMatch struct {
	Kind       string   `json:"kind"`
	EntityID   int      `json:"entityID"`
	Name       string   `json:"name"`
	MatchKinds []string `json:"matchKinds"`
}

type aliasInspectResponse struct {
	Input      string              `json:"input"`
	Normalized string              `json:"normalized"`
	Matches    []aliasInspectMatch `json:"matches"`
	Ambiguous  bool                `json:"ambiguous"`
}

func inspectAliasValue(value string, entities []aliasCollisionEntity) aliasInspectResponse {
	normalized := normalizeAliasCollisionValue(value)
	response := aliasInspectResponse{
		Input:      strings.TrimSpace(value),
		Normalized: normalized,
		Matches:    []aliasInspectMatch{},
	}
	if normalized == "" {
		return response
	}

	for _, entity := range entities {
		if entity.ID <= 0 || strings.TrimSpace(entity.Kind) == "" {
			continue
		}
		matchKinds := make([]string, 0, 2)
		if normalizeAliasCollisionValue(entity.Name) == normalized {
			matchKinds = append(matchKinds, "canonical")
		}
		for _, alias := range entity.Aliases {
			if normalizeAliasCollisionValue(alias) == normalized {
				matchKinds = append(matchKinds, "alias")
				break
			}
		}
		if len(matchKinds) == 0 {
			continue
		}
		response.Matches = append(response.Matches, aliasInspectMatch{
			Kind:       strings.ToLower(strings.TrimSpace(entity.Kind)),
			EntityID:   entity.ID,
			Name:       entity.Name,
			MatchKinds: matchKinds,
		})
	}

	sort.Slice(response.Matches, func(i, j int) bool {
		if response.Matches[i].Kind != response.Matches[j].Kind {
			return response.Matches[i].Kind < response.Matches[j].Kind
		}
		return response.Matches[i].EntityID < response.Matches[j].EntityID
	})
	response.Ambiguous = len(response.Matches) > 1
	return response
}

func loadAliasCollisionEntities(ctx context.Context, repository models.Repository) ([]aliasCollisionEntity, error) {
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

	entities := make([]aliasCollisionEntity, 0, len(performers)+len(studios)+len(tags)+len(copyrights))
	for _, performer := range performers {
		if performer == nil {
			continue
		}
		if err := performer.LoadAliases(ctx, repository.Performer); err != nil {
			return nil, fmt.Errorf("loading Character %d aliases: %w", performer.ID, err)
		}
		entities = append(entities, aliasCollisionEntity{
			ID: performer.ID, Kind: "character", Name: performer.Name, Aliases: append([]string(nil), performer.Aliases.List()...),
		})
	}
	for _, studio := range studios {
		if studio == nil {
			continue
		}
		if err := studio.LoadAliases(ctx, repository.Studio); err != nil {
			return nil, fmt.Errorf("loading Artist %d aliases: %w", studio.ID, err)
		}
		entities = append(entities, aliasCollisionEntity{
			ID: studio.ID, Kind: "artist", Name: studio.Name, Aliases: append([]string(nil), studio.Aliases.List()...),
		})
	}
	for _, tag := range tags {
		if tag == nil {
			continue
		}
		if err := tag.LoadAliases(ctx, repository.Tag); err != nil {
			return nil, fmt.Errorf("loading Tag %d aliases: %w", tag.ID, err)
		}
		entities = append(entities, aliasCollisionEntity{
			ID: tag.ID, Kind: "tag", Name: tag.Name, Aliases: append([]string(nil), tag.Aliases.List()...),
		})
	}
	for _, copyright := range copyrights {
		if copyright == nil {
			continue
		}
		entities = append(entities, aliasCollisionEntity{
			ID: copyright.ID, Kind: "copyright", Name: copyright.Name, Aliases: append([]string(nil), copyright.Aliases...),
		})
	}
	return entities, nil
}

func (rs imageRoutes) AliasCollisionInspect(w http.ResponseWriter, r *http.Request) {
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	if value == "" {
		http.Error(w, "value is required", http.StatusBadRequest)
		return
	}
	if len(value) > 512 {
		http.Error(w, "value is too long", http.StatusBadRequest)
		return
	}

	repository := manager.GetInstance().Repository
	var response aliasInspectResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		entities, err := loadAliasCollisionEntities(ctx, repository)
		if err != nil {
			return err
		}
		response = inspectAliasValue(value, entities)
		return nil
	}); err != nil {
		http.Error(w, fmt.Sprintf("inspecting alias value: %v", err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
