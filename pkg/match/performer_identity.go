package match

import (
	"context"
	"strings"
	"unicode"

	"github.com/stashapp/stash/pkg/models"
)

// performerAliasAutoTagQueryer is implemented by stores that can cheaply find
// performer candidates from aliases as well as canonical names. Keeping this
// optional preserves compatibility with lightweight test readers.
type performerAliasAutoTagQueryer interface {
	QueryAliasesForAutoTag(ctx context.Context, words []string) ([]*models.Performer, error)
}

// NormalizeIdentityPart normalizes disambiguation-style identity metadata so
// punctuation acts like a separator while compact forms still match. For
// example, "Re:Zero" normalizes to "Re Zero".
func NormalizeIdentityPart(value string) string {
	var ret strings.Builder
	separator := false

	for _, r := range strings.TrimSpace(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			ret.WriteRune(r)
			separator = false
			continue
		}

		if ret.Len() > 0 && !separator {
			ret.WriteByte(' ')
			separator = true
		}
	}

	return strings.TrimSpace(ret.String())
}

// PathMatchesIdentityPart matches identity metadata such as a performer
// disambiguation while treating punctuation as separators. For example,
// "Re:Zero" matches "Re Zero", "Re-Zero", and "ReZero" in a path.
func PathMatchesIdentityPart(path, value string) bool {
	value = NormalizeIdentityPart(value)
	if value == "" {
		return false
	}

	return nameMatchesPath(value, NormalizeIdentityPart(path)) != -1
}

func performerIdentityKey(performer *models.Performer) string {
	return strings.ToLower(strings.TrimSpace(performer.Name)) + "\x00" + strings.ToLower(NormalizeIdentityPart(performer.Disambiguation))
}

// PerformerCanonicalMatchesPath matches a performer by canonical identity.
// A disambiguated performer must match both its name and disambiguation. A
// bare-name performer can be suppressed when that canonical name is ambiguous.
func PerformerCanonicalMatchesPath(performer *models.Performer, path string, canonicalNameAmbiguous bool) bool {
	if performer == nil || nameMatchesPath(performer.Name, path) == -1 {
		return false
	}

	if strings.TrimSpace(performer.Disambiguation) == "" {
		return !canonicalNameAmbiguous
	}

	return PathMatchesIdentityPart(path, performer.Disambiguation)
}

// PathToPerformersIdentityAware is the native auto-tag performer matcher used
// by file-based auto-tagging. Disambiguation is part of the canonical identity,
// while aliases are explicit independent match names.
func PathToPerformersIdentityAware(ctx context.Context, path string, reader models.PerformerAutoTagQueryer, cache *Cache, trimExt bool) ([]*models.Performer, error) {
	words := getPathWords(path, trimExt)
	performers, err := getPerformers(ctx, words, reader, cache)
	if err != nil {
		return nil, err
	}

	if aliasReader, ok := reader.(performerAliasAutoTagQueryer); ok {
		aliasCandidates, err := aliasReader.QueryAliasesForAutoTag(ctx, words)
		if err != nil {
			return nil, err
		}
		performers = append(performers, aliasCandidates...)
	}

	candidates := make([]*models.Performer, 0, len(performers))
	seen := make(map[int]struct{}, len(performers))
	for _, performer := range performers {
		if performer == nil {
			continue
		}
		if _, exists := seen[performer.ID]; exists {
			continue
		}
		seen[performer.ID] = struct{}{}
		candidates = append(candidates, performer)
	}

	nameOwners := make(map[string]map[int]struct{})
	identityOwners := make(map[string]map[int]struct{})
	aliasesByID := make(map[int][]string, len(candidates))

	for _, performer := range candidates {
		nameKey := strings.ToLower(strings.TrimSpace(performer.Name))
		if nameOwners[nameKey] == nil {
			nameOwners[nameKey] = make(map[int]struct{})
		}
		nameOwners[nameKey][performer.ID] = struct{}{}

		identityKey := performerIdentityKey(performer)
		if identityOwners[identityKey] == nil {
			identityOwners[identityKey] = make(map[int]struct{})
		}
		identityOwners[identityKey][performer.ID] = struct{}{}

		aliases, err := reader.GetAliases(ctx, performer.ID)
		if err != nil {
			return nil, err
		}
		aliasesByID[performer.ID] = aliases
	}

	ret := make([]*models.Performer, 0, len(candidates))
	for _, performer := range candidates {
		matchedAlias := false
		for _, alias := range aliasesByID[performer.ID] {
			if nameMatchesPath(alias, path) != -1 {
				matchedAlias = true
				break
			}
		}

		nameKey := strings.ToLower(strings.TrimSpace(performer.Name))
		canonicalAmbiguous := false
		if strings.TrimSpace(performer.Disambiguation) == "" {
			canonicalAmbiguous = len(nameOwners[nameKey]) > 1
		} else {
			canonicalAmbiguous = len(identityOwners[performerIdentityKey(performer)]) > 1
		}

		if matchedAlias || PerformerCanonicalMatchesPath(performer, path, canonicalAmbiguous) {
			ret = append(ret, performer)
		}
	}

	return ret, nil
}
