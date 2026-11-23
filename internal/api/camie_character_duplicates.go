package api

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

// reversedCamieCharacterName returns a two-part name in the other order
// ("Hatsune Miku" ↔ "Miku Hatsune"): sites and models disagree on
// given/family name order. Other names return "".
func reversedCamieCharacterName(name string) string {
	parts := strings.Fields(strings.ReplaceAll(name, "_", " "))
	if len(parts) != 2 || strings.EqualFold(parts[0], parts[1]) {
		return ""
	}
	return parts[1] + " " + parts[0]
}

// findCamieReversedNamePerformer finds the one existing Character whose name
// is the prediction's name reversed. A disambiguated prediction only matches
// the same disambiguation. More than one candidate is not a match.
func findCamieReversedNamePerformer(ctx context.Context, repository models.Repository, characterName, disambiguation string) (*models.Performer, error) {
	reversed := reversedCamieCharacterName(characterName)
	if reversed == "" {
		return nil, nil
	}
	matches, err := repository.Performer.FindByNames(ctx, []string{reversed}, true)
	if err != nil {
		return nil, err
	}
	var found *models.Performer
	for _, match := range matches {
		if match == nil || !strings.EqualFold(strings.Join(strings.Fields(match.Name), " "), reversed) {
			continue
		}
		if disambiguation != "" && !strings.EqualFold(strings.TrimSpace(match.Disambiguation), disambiguation) {
			continue
		}
		if found != nil && found.ID != match.ID {
			return nil, nil
		}
		found = match
	}
	return found, nil
}

// findCamiePerformerFallback runs after exact name matching fails: aliases
// first, then the reversed name order.
func findCamiePerformerFallback(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, error) {
	performer, err := findCamiePerformerAliasPrediction(ctx, repository, prediction)
	if err != nil || performer != nil {
		return performer, err
	}
	characterName, disambiguation := camieCharacterIdentity(prediction)
	return findCamieReversedNamePerformer(ctx, repository, characterName, disambiguation)
}

// camieCharacterDuplicateKey is the same for a Character name in either
// order, so "hatsune_miku" and "Miku Hatsune" count as one prediction.
func camieCharacterDuplicateKey(prediction camietagger.Tag) string {
	characterName, disambiguation := camieCharacterIdentity(prediction)
	parts := strings.Fields(strings.ToLower(strings.ReplaceAll(characterName, "_", " ")))
	if len(parts) == 2 {
		sort.Strings(parts)
	}
	return strings.Join(parts, " ") + "\x00" + strings.ToLower(strings.TrimSpace(disambiguation))
}

var (
	camieSoloCountTags  = map[string]bool{"1girl": true, "1boy": true, "1other": true}
	camieGroupCountTags = regexp.MustCompile(`^(?:\d+\+?(?:girls|boys|others)|multiple_(?:girls|boys|others))$`)
)

// camieDepictsOneCharacter reports whether the general tags say the Image
// shows a single person: solo, or exactly one of 1girl/1boy/1other without a
// group count such as 2girls.
func camieDepictsOneCharacter(predictions []camietagger.Tag) bool {
	solo, singles := false, 0
	for _, prediction := range predictions {
		if prediction.Category != "general" {
			continue
		}
		name := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(prediction.Name, "_", " ")), "_"))
		switch {
		case name == "solo":
			solo = true
		case camieSoloCountTags[name]:
			singles++
		case camieGroupCountTags.MatchString(name):
			return false
		}
	}
	return solo || singles == 1
}

// withoutSoloConflictCharacters drops model-inferred Characters when the tags
// say the Image shows one person but the models suggest several: they are
// usually the same Character under different names. Characters from
// authoritative sources (filename, MD5 match) are kept; a solo Image can show
// a cosplayed Character as well.
func withoutSoloConflictCharacters(merged []camietagger.Tag, authoritative ...[]camietagger.Tag) ([]camietagger.Tag, []string) {
	trusted := map[string]bool{}
	for _, predictions := range authoritative {
		for _, prediction := range predictions {
			prediction = normalizeCamiePrediction(prediction)
			if prediction.Category == "character" {
				trusted[camieCharacterDuplicateKey(prediction)] = true
			}
		}
	}
	var inferred []string
	for _, prediction := range merged {
		if prediction.Category == "character" && !trusted[camieCharacterDuplicateKey(prediction)] {
			inferred = append(inferred, prediction.Name)
		}
	}
	if len(inferred) < 2 || !camieDepictsOneCharacter(merged) {
		return merged, nil
	}
	kept := make([]camietagger.Tag, 0, len(merged))
	for _, prediction := range merged {
		if prediction.Category == "character" && !trusted[camieCharacterDuplicateKey(prediction)] {
			continue
		}
		kept = append(kept, prediction)
	}
	return kept, inferred
}
