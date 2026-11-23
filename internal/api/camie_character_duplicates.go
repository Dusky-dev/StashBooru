package api

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

// loadDuplicateDetectionConfig reads the saved duplicate detection settings.
// Tests replace it, since they run without a configuration directory.
var loadDuplicateDetectionConfig = func() (duplicateDetectionConfig, error) {
	config, err := loadCamieConfig()
	return config.DuplicateDetection, err
}

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
	if settings, err := loadDuplicateDetectionConfig(); err != nil || !settings.reversedNames() {
		return nil, err
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

// camieSeveralFigureTags are Danbooru tags that mean a solo Image can still
// carry several Character tags: one person cosplaying another Character,
// several versions of one person, or a focus on one of several people.
var camieSeveralFigureTags = map[string]bool{
	"solo_focus":       true,
	"cosplay":          true,
	"multiple_views":   true,
	"multiple_persona": true,
	"dual_persona":     true,
	"character_doll":   true,
	"crossover":        true,
	"costume_switch":   true,
	"split_screen":     true,
	"reference_sheet":  true,
	"character_sheet":  true,
	"fusion":           true,
	"genderswap":       true,
	"alternate_form":   true,
	"doppelganger":     true,
	"clone":            true,
}

var camieGroupCountTags = regexp.MustCompile(`^(?:\d+\+?(?:girls|boys|others)|multiple_(?:girls|boys|others))$`)

func camieGeneralTagKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(name, "_", " ")), "_"))
}

// camieDepictsOneCharacter reports whether the general tags clearly say the
// Image shows a single person: `solo`, with no group count (2girls, …) and no
// tag that allows several Characters on one person (cosplay, solo_focus, …).
// A count tag such as 1girl alone is not enough: other people may be untagged.
func camieDepictsOneCharacter(predictions []camietagger.Tag) bool {
	solo := false
	for _, prediction := range predictions {
		if prediction.Category != "general" {
			continue
		}
		name := camieGeneralTagKey(prediction.Name)
		switch {
		case name == "solo":
			solo = true
		case camieGroupCountTags.MatchString(name), camieSeveralFigureTags[name]:
			return false
		}
	}
	return solo
}

// camieTrustedCharacterSource reports whether a prediction came from an
// authoritative source (filename or an exact MD5 match).
func camieTrustedCharacterSource(source string) bool {
	return source == "filename" || strings.HasPrefix(source, "booru:")
}

// withoutSoloConflictCharacters drops model-suggested Characters when the
// tags say the Image shows one person but the models suggest several
// different Characters. enriched is predictions with targets resolved, in the
// same order: names that resolve to one existing Character count once.
// Characters from authoritative sources are always kept.
// onePerson is camieDepictsOneCharacter of every merged prediction, including
// general tags the job does not apply.
func withoutSoloConflictCharacters(predictions, enriched []camietagger.Tag, onePerson bool) ([]camietagger.Tag, []string) {
	if !onePerson || len(enriched) != len(predictions) {
		return predictions, nil
	}
	distinct := map[string]bool{}
	var inferred []string
	for index, prediction := range predictions {
		if prediction.Category != "character" || camieTrustedCharacterSource(prediction.Source) {
			continue
		}
		key := "name:" + camieCharacterDuplicateKey(prediction)
		if target := enriched[index]; target.TargetExists && target.TargetPath != "" {
			key = "target:" + target.TargetPath
		}
		distinct[key] = true
		inferred = append(inferred, prediction.Name)
	}
	if len(distinct) < 2 {
		return predictions, nil
	}
	kept := make([]camietagger.Tag, 0, len(predictions))
	for _, prediction := range predictions {
		if prediction.Category == "character" && !camieTrustedCharacterSource(prediction.Source) {
			continue
		}
		kept = append(kept, prediction)
	}
	return kept, inferred
}
