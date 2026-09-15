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

type aliasCollisionEntity struct {
	ID      int
	Kind    string
	Name    string
	Aliases []string
}

type aliasCollisionReference struct {
	EntityID   int      `json:"entityID"`
	Name       string   `json:"name"`
	MatchKinds []string `json:"matchKinds"`
}

type aliasCollision struct {
	Kind       string                    `json:"kind"`
	Value      string                    `json:"value"`
	References []aliasCollisionReference `json:"references"`
}

type aliasCollisionResponse struct {
	Collisions []aliasCollision `json:"collisions"`
	Counts     map[string]int   `json:"counts"`
}

func normalizeAliasCollisionValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(value, "_", " ")), " "))
}

// findAliasCollisions reports strings that can resolve to more than one native
// entity of the same kind. Canonical-name/alias and alias/alias collisions are
// both retained, while duplicate aliases on the same entity do not create a
// false collision.
func findAliasCollisions(entities []aliasCollisionEntity) []aliasCollision {
	type collisionKey struct {
		kind  string
		value string
	}
	type referenceAccumulator struct {
		name       string
		matchKinds map[string]struct{}
	}

	buckets := make(map[collisionKey]map[int]*referenceAccumulator)
	add := func(entity aliasCollisionEntity, value, matchKind string) {
		kind := strings.ToLower(strings.TrimSpace(entity.Kind))
		normalized := normalizeAliasCollisionValue(value)
		if entity.ID <= 0 || kind == "" || normalized == "" {
			return
		}
		key := collisionKey{kind: kind, value: normalized}
		if buckets[key] == nil {
			buckets[key] = make(map[int]*referenceAccumulator)
		}
		entry := buckets[key][entity.ID]
		if entry == nil {
			entry = &referenceAccumulator{name: entity.Name, matchKinds: make(map[string]struct{})}
			buckets[key][entity.ID] = entry
		}
		entry.matchKinds[matchKind] = struct{}{}
	}

	for _, entity := range entities {
		add(entity, entity.Name, "canonical")
		for _, alias := range entity.Aliases {
			add(entity, alias, "alias")
		}
	}

	collisions := make([]aliasCollision, 0)
	for key, refs := range buckets {
		if len(refs) < 2 {
			continue
		}
		collision := aliasCollision{Kind: key.kind, Value: key.value}
		for id, ref := range refs {
			matchKinds := make([]string, 0, len(ref.matchKinds))
			for matchKind := range ref.matchKinds {
				matchKinds = append(matchKinds, matchKind)
			}
			sort.Strings(matchKinds)
			collision.References = append(collision.References, aliasCollisionReference{
				EntityID:   id,
				Name:       ref.name,
				MatchKinds: matchKinds,
			})
		}
		sort.Slice(collision.References, func(i, j int) bool {
			return collision.References[i].EntityID < collision.References[j].EntityID
		})
		collisions = append(collisions, collision)
	}

	sort.Slice(collisions, func(i, j int) bool {
		if collisions[i].Kind != collisions[j].Kind {
			return collisions[i].Kind < collisions[j].Kind
		}
		return collisions[i].Value < collisions[j].Value
	})
	return collisions
}

func collectAliasCollisions(ctx context.Context, repository models.Repository) (aliasCollisionResponse, error) {
	performers, err := repository.Performer.All(ctx)
	if err != nil {
		return aliasCollisionResponse{}, err
	}
	studios, err := repository.Studio.All(ctx)
	if err != nil {
		return aliasCollisionResponse{}, err
	}
	tags, err := repository.Tag.All(ctx)
	if err != nil {
		return aliasCollisionResponse{}, err
	}
	copyrights, _, err := repository.Copyright.Query(ctx, nil)
	if err != nil {
		return aliasCollisionResponse{}, err
	}

	entities := make([]aliasCollisionEntity, 0, len(performers)+len(studios)+len(tags)+len(copyrights))
	for _, performer := range performers {
		if performer == nil {
			continue
		}
		if err := performer.LoadAliases(ctx, repository.Performer); err != nil {
			return aliasCollisionResponse{}, fmt.Errorf("loading Character %d aliases: %w", performer.ID, err)
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
			return aliasCollisionResponse{}, fmt.Errorf("loading Artist %d aliases: %w", studio.ID, err)
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
			return aliasCollisionResponse{}, fmt.Errorf("loading Tag %d aliases: %w", tag.ID, err)
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

	collisions := findAliasCollisions(entities)
	counts := make(map[string]int)
	for _, collision := range collisions {
		counts[collision.Kind]++
	}
	return aliasCollisionResponse{Collisions: collisions, Counts: counts}, nil
}

func (rs imageRoutes) AliasCollisions(w http.ResponseWriter, r *http.Request) {
	repository := manager.GetInstance().Repository
	var response aliasCollisionResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		response, err = collectAliasCollisions(ctx, repository)
		return err
	}); err != nil {
		http.Error(w, fmt.Sprintf("inspecting alias collisions: %v", err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
