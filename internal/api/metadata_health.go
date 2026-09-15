package api

import (
	"sort"
	"strings"
)

type metadataHealthEntity struct {
	ID   int
	Kind string
	Name string
}

type metadataHealthFinding struct {
	Code      string
	Kind      string
	Value     string
	EntityIDs []int
}

const metadataHealthDuplicateCanonical = "duplicate-canonical-name"

func normalizeMetadataHealthName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(value, "_", " ")), " "))
}

// findDuplicateCanonicalMetadataEntities is the first read-only Metadata
// Health diagnostic. It reports duplicate canonical names within each native
// entity kind without mutating or merging anything.
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
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Value < findings[j].Value
	})
	return findings
}
