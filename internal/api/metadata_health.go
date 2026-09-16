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

type metadataHealthEntity struct {
	ID   int
	Kind string
	Name string
}

type metadataHealthFinding struct {
	Code      string `json:"code"`
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	EntityIDs []int  `json:"entityIDs"`
	MediaID   int    `json:"mediaID,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type metadataHealthArtistRelation struct {
	MediaKind     string
	MediaID       int
	LegacyStudio  *int
	NativeArtists []int
}

type metadataHealthResponse struct {
	Findings []metadataHealthFinding `json:"findings"`
	Counts   map[string]int          `json:"counts"`
}

const (
	metadataHealthDuplicateCanonical        = "duplicate-canonical-name"
	metadataHealthCopyrightTagCollision     = "legacy-copyright-tag-collision"
	metadataHealthLegacyArtistMismatch      = "legacy-artist-mismatch"
	metadataHealthLegacyArtistMissing       = "legacy-artist-missing-native"
	metadataHealthNativeArtistMissingLegacy = "native-artist-missing-legacy"
)

func normalizeMetadataHealthName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(value, "_", " ")), " "))
}

// findDuplicateCanonicalMetadataEntities reports duplicate canonical names
// within each native entity kind without mutating or merging anything.
func findDuplicateCanonicalMetadataEntities(entities []metadataHealthEntity) []metadataHealthFinding {
	type bucketKey struct {
		kind string
		name string
	}
	buckets := make(map[bucketKey][]int)
	for _, entity := range entities {
		kind := strings.ToLower(strings.TrimSpace(entity.Kind))
		name := normalizeMetadataHealthName(entity.Name)
		if entity.ID <= 0 || kind == "" || name == "" {
			continue
		}
		key := bucketKey{kind: kind, name: name}
		buckets[key] = append(buckets[key], entity.ID)
	}

	findings := make([]metadataHealthFinding, 0)
	for key, ids := range buckets {
		if len(ids) < 2 {
			continue
		}
		sort.Ints(ids)
		findings = append(findings, metadataHealthFinding{
			Code:      metadataHealthDuplicateCanonical,
			Kind:      key.kind,
			Value:     key.name,
			EntityIDs: ids,
		})
	}
	sortMetadataHealthFindings(findings)
	return findings
}

// findCopyrightTagCollisions flags legacy Tags whose canonical name is also a
// native Copyright. This is diagnostic only: same-name data may be intentional,
// so Metadata Health never converts or deletes either entity automatically.
func findCopyrightTagCollisions(tags, copyrights []metadataHealthEntity) []metadataHealthFinding {
	copyrightByName := make(map[string][]int)
	for _, entity := range copyrights {
		name := normalizeMetadataHealthName(entity.Name)
		if entity.ID > 0 && name != "" {
			copyrightByName[name] = append(copyrightByName[name], entity.ID)
		}
	}

	findings := make([]metadataHealthFinding, 0)
	for _, tag := range tags {
		name := normalizeMetadataHealthName(tag.Name)
		copyrightIDs := copyrightByName[name]
		if tag.ID <= 0 || name == "" || len(copyrightIDs) == 0 {
			continue
		}
		ids := append([]int{tag.ID}, copyrightIDs...)
		sort.Ints(ids[1:])
		findings = append(findings, metadataHealthFinding{
			Code:      metadataHealthCopyrightTagCollision,
			Kind:      "copyright",
			Value:     name,
			EntityIDs: ids,
			Detail:    "legacy Tag shares a canonical name with a native Copyright",
		})
	}
	sortMetadataHealthFindings(findings)
	return findings
}

func findArtistRelationshipDrift(relations []metadataHealthArtistRelation) []metadataHealthFinding {
	findings := make([]metadataHealthFinding, 0)
	for _, relation := range relations {
		kind := strings.ToLower(strings.TrimSpace(relation.MediaKind))
		if relation.MediaID <= 0 || (kind != "image" && kind != "video") {
			continue
		}

		native := make(map[int]struct{}, len(relation.NativeArtists))
		for _, id := range relation.NativeArtists {
			if id > 0 {
				native[id] = struct{}{}
			}
		}

		switch {
		case relation.LegacyStudio != nil && *relation.LegacyStudio > 0 && len(native) == 0:
			findings = append(findings, metadataHealthFinding{
				Code:      metadataHealthLegacyArtistMissing,
				Kind:      kind,
				MediaID:   relation.MediaID,
				EntityIDs: []int{*relation.LegacyStudio},
				Detail:    "legacy StudioID is set but native Artist relationships are empty",
			})
		case relation.LegacyStudio == nil && len(native) > 0:
			ids := sortedMetadataHealthIDs(native)
			findings = append(findings, metadataHealthFinding{
				Code:      metadataHealthNativeArtistMissingLegacy,
				Kind:      kind,
				MediaID:   relation.MediaID,
				EntityIDs: ids,
				Detail:    "native Artist relationships exist but legacy StudioID is empty",
			})
		case relation.LegacyStudio != nil && *relation.LegacyStudio > 0:
			if _, ok := native[*relation.LegacyStudio]; !ok {
				ids := append([]int{*relation.LegacyStudio}, sortedMetadataHealthIDs(native)...)
				findings = append(findings, metadataHealthFinding{
					Code:      metadataHealthLegacyArtistMismatch,
					Kind:      kind,
					MediaID:   relation.MediaID,
					EntityIDs: ids,
					Detail:    "legacy StudioID is not one of the native Artists",
				})
			}
		}
	}
	sortMetadataHealthFindings(findings)
	return findings
}

func sortedMetadataHealthIDs(values map[int]struct{}) []int {
	ids := make([]int, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func sortMetadataHealthFindings(findings []metadataHealthFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Value != findings[j].Value {
			return findings[i].Value < findings[j].Value
		}
		return findings[i].MediaID < findings[j].MediaID
	})
}

func metadataHealthStudioIDs(studios []*models.Studio) []int {
	ids := make([]int, 0, len(studios))
	for _, studio := range studios {
		if studio != nil && studio.ID > 0 {
			ids = append(ids, studio.ID)
		}
	}
	sort.Ints(ids)
	return ids
}

func collectMetadataHealth(ctx context.Context, repository models.Repository) (metadataHealthResponse, error) {
	performers, err := repository.Performer.All(ctx)
	if err != nil {
		return metadataHealthResponse{}, err
	}
	studios, err := repository.Studio.All(ctx)
	if err != nil {
		return metadataHealthResponse{}, err
	}
	tags, err := repository.Tag.All(ctx)
	if err != nil {
		return metadataHealthResponse{}, err
	}
	copyrights, _, err := repository.Copyright.Query(ctx, nil)
	if err != nil {
		return metadataHealthResponse{}, err
	}

	entities := make([]metadataHealthEntity, 0, len(performers)+len(studios)+len(tags)+len(copyrights))
	copyrightEntities := make([]metadataHealthEntity, 0, len(copyrights))
	tagEntities := make([]metadataHealthEntity, 0, len(tags))
	for _, entity := range performers {
		if entity != nil {
			entities = append(entities, metadataHealthEntity{ID: entity.ID, Kind: "character", Name: entity.Name})
		}
	}
	for _, entity := range studios {
		if entity != nil {
			entities = append(entities, metadataHealthEntity{ID: entity.ID, Kind: "artist", Name: entity.Name})
		}
	}
	for _, entity := range copyrights {
		if entity != nil {
			value := metadataHealthEntity{ID: entity.ID, Kind: "copyright", Name: entity.Name}
			entities = append(entities, value)
			copyrightEntities = append(copyrightEntities, value)
		}
	}
	for _, entity := range tags {
		if entity != nil {
			value := metadataHealthEntity{ID: entity.ID, Kind: "tag", Name: entity.Name}
			entities = append(entities, value)
			tagEntities = append(tagEntities, value)
		}
	}

	findings := findDuplicateCanonicalMetadataEntities(entities)
	findings = append(findings, findCopyrightTagCollisions(tagEntities, copyrightEntities)...)

	images, err := repository.Image.All(ctx)
	if err != nil {
		return metadataHealthResponse{}, err
	}
	scenes, err := repository.Scene.All(ctx)
	if err != nil {
		return metadataHealthResponse{}, err
	}
	relations := make([]metadataHealthArtistRelation, 0, len(images)+len(scenes))
	for _, image := range images {
		if image == nil {
			continue
		}
		artists, err := repository.ImageArtist.FindByImageID(ctx, image.ID)
		if err != nil {
			return metadataHealthResponse{}, fmt.Errorf("loading Image %d Artists: %w", image.ID, err)
		}
		relations = append(relations, metadataHealthArtistRelation{
			MediaKind:     "image",
			MediaID:       image.ID,
			LegacyStudio:  image.StudioID,
			NativeArtists: metadataHealthStudioIDs(artists),
		})
	}
	for _, scene := range scenes {
		if scene == nil {
			continue
		}
		artists, err := repository.SceneArtist.FindBySceneID(ctx, scene.ID)
		if err != nil {
			return metadataHealthResponse{}, fmt.Errorf("loading Video %d Artists: %w", scene.ID, err)
		}
		relations = append(relations, metadataHealthArtistRelation{
			MediaKind:     "video",
			MediaID:       scene.ID,
			LegacyStudio:  scene.StudioID,
			NativeArtists: metadataHealthStudioIDs(artists),
		})
	}
	findings = append(findings, findArtistRelationshipDrift(relations)...)
	sortMetadataHealthFindings(findings)

	counts := make(map[string]int)
	for _, finding := range findings {
		counts[finding.Code]++
	}
	return metadataHealthResponse{Findings: findings, Counts: counts}, nil
}

func (rs imageRoutes) MetadataHealth(w http.ResponseWriter, r *http.Request) {
	repository := manager.GetInstance().Repository
	var response metadataHealthResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		response, err = collectMetadataHealth(ctx, repository)
		return err
	}); err != nil {
		http.Error(w, fmt.Sprintf("scanning metadata health: %v", err), http.StatusInternalServerError)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
