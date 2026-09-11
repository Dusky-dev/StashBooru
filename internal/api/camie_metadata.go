package api

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/performer"
	"github.com/stashapp/stash/pkg/studio"
	"github.com/stashapp/stash/pkg/tag"
)

const (
	camieCopyrightRootName  = "Copyright"
	camieCopyrightRootAlias = "__stashbooru_copyright_root__"
)

type camieFilenameLayout struct {
	expression *regexp.Regexp
	groups     map[string]int
}

var (
	camieFilenameTokenPattern           = regexp.MustCompile(`%([a-zA-Z0-9_]+)%`)
	camieCharacterDisambiguationPattern = regexp.MustCompile(`^(.+?)\s*\(([^()]*)\)\s*$`)
)

func compileCamieFilenameLayout(layout string) (*camieFilenameLayout, error) {
	layout = strings.TrimSpace(layout)
	if layout == "" {
		return nil, fmt.Errorf("filename layout cannot be empty")
	}

	allowed := map[string]bool{
		"artist":    true,
		"copyright": true,
		"character": true,
		"md5":       true,
		"ext":       true,
	}
	seen := map[string]bool{}
	groups := map[string]int{}
	var expression strings.Builder
	expression.WriteString("^")

	matches := camieFilenameTokenPattern.FindAllStringSubmatchIndex(layout, -1)
	cursor := 0
	groupIndex := 0
	for _, match := range matches {
		expression.WriteString(regexp.QuoteMeta(layout[cursor:match[0]]))
		token := layout[match[2]:match[3]]
		if !allowed[token] {
			return nil, fmt.Errorf("unsupported filename token %%%s%%", token)
		}
		if seen[token] {
			return nil, fmt.Errorf("filename token %%%s%% may only appear once", token)
		}
		seen[token] = true
		groupIndex++
		groups[token] = groupIndex
		switch token {
		case "md5":
			expression.WriteString(`([0-9A-Fa-f]{32})`)
		case "ext":
			expression.WriteString(`([^./\\]+)`)
		default:
			expression.WriteString(`(.+?)`)
		}
		cursor = match[1]
	}
	expression.WriteString(regexp.QuoteMeta(layout[cursor:]))
	expression.WriteString("$")

	if len(matches) == 0 {
		return nil, fmt.Errorf("filename layout must contain at least one supported token")
	}
	compiled, err := regexp.Compile(expression.String())
	if err != nil {
		return nil, fmt.Errorf("compiling filename layout: %w", err)
	}
	return &camieFilenameLayout{expression: compiled, groups: groups}, nil
}

func parseCamieFilename(path string, layout string) ([]camietagger.Tag, error) {
	compiled, err := compileCamieFilenameLayout(layout)
	if err != nil {
		return nil, err
	}
	filename := filepath.Base(path)
	match := compiled.expression.FindStringSubmatch(filename)
	if match == nil {
		return nil, nil
	}

	var predictions []camietagger.Tag
	for _, category := range []string{"artist", "copyright", "character"} {
		index, ok := compiled.groups[category]
		if !ok || index >= len(match) {
			continue
		}
		value := strings.TrimSpace(match[index])
		if value == "" {
			continue
		}
		predictions = append(predictions, camietagger.Tag{
			Name:     value,
			Category: category,
			Score:    1,
			Source:   "filename",
		})
	}
	return predictions, nil
}

func canonicalCamieName(name string, category string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	name = strings.Join(strings.Fields(name), " ")
	if category == "character" || category == "copyright" {
		return titleCaseCamieName(name)
	}
	return name
}

func titleCaseCamieName(value string) string {
	var result strings.Builder
	result.Grow(len(value))
	capitalize := true
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		if capitalize && unicode.IsLetter(r) {
			r = unicode.ToUpper(r)
			capitalize = false
		}
		result.WriteRune(r)
		if unicode.IsSpace(r) || strings.ContainsRune("([{/\\-", r) {
			capitalize = true
		}
	}
	return result.String()
}

func normalizeCamiePrediction(prediction camietagger.Tag) camietagger.Tag {
	prediction.Category = strings.ToLower(strings.TrimSpace(prediction.Category))
	if prediction.Category == "" {
		prediction.Category = "general"
	}
	if prediction.RawName == "" {
		prediction.RawName = strings.TrimSpace(prediction.Name)
	}
	prediction.Name = canonicalCamieName(prediction.RawName, prediction.Category)
	if prediction.Source == "" {
		prediction.Source = "model"
	}
	return prediction
}

func camieFilenameAuthoritativeCategory(category string) bool {
	switch category {
	case "character", "artist", "copyright":
		return true
	default:
		return false
	}
}

func mergeCamiePredictions(modelPredictions, filenamePredictions []camietagger.Tag) []camietagger.Tag {
	merged := make([]camietagger.Tag, 0, len(modelPredictions)+len(filenamePredictions))
	seen := make(map[string]int, len(modelPredictions)+len(filenamePredictions))
	authoritativeCategories := make(map[string]bool, 3)

	add := func(prediction camietagger.Tag) {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name == "" {
			return
		}
		key := prediction.Category + "\x00" + strings.ToLower(prediction.Name)
		if index, ok := seen[key]; ok {
			current := merged[index]
			if prediction.Score > current.Score {
				current.Score = prediction.Score
			}
			if current.RawName == "" {
				current.RawName = prediction.RawName
			}
			if current.Source != prediction.Source && prediction.Source != "" {
				current.Source = "model+filename"
			}
			merged[index] = current
			return
		}
		seen[key] = len(merged)
		merged = append(merged, prediction)
	}

	for _, prediction := range filenamePredictions {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name == "" {
			continue
		}
		if camieFilenameAuthoritativeCategory(prediction.Category) {
			authoritativeCategories[prediction.Category] = true
		}
		add(prediction)
	}

	for _, prediction := range modelPredictions {
		prediction = normalizeCamiePrediction(prediction)
		if prediction.Name == "" {
			continue
		}
		key := prediction.Category + "\x00" + strings.ToLower(prediction.Name)
		if authoritativeCategories[prediction.Category] {
			if _, ok := seen[key]; !ok {
				continue
			}
		}
		add(prediction)
	}
	return merged
}

func camieAliases(prediction camietagger.Tag) []string {
	raw := strings.TrimSpace(prediction.RawName)
	if raw == "" || strings.EqualFold(raw, prediction.Name) {
		return []string{}
	}
	return []string{raw}
}

func camieCharacterIdentity(prediction camietagger.Tag) (string, string) {
	prediction = normalizeCamiePrediction(prediction)
	name := strings.TrimSpace(prediction.Name)
	match := camieCharacterDisambiguationPattern.FindStringSubmatch(name)
	if len(match) != 3 {
		return name, ""
	}

	characterName := strings.TrimSpace(match[1])
	disambiguation := strings.TrimSpace(match[2])
	if characterName == "" || disambiguation == "" {
		return name, ""
	}
	return characterName, disambiguation
}

func camieCharacterAliases(prediction camietagger.Tag, characterName string) []string {
	prediction = normalizeCamiePrediction(prediction)
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(characterName)): true}
	aliases := make([]string, 0, 2)
	for _, candidate := range []string{prediction.Name, prediction.RawName} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		aliases = append(aliases, candidate)
	}
	return aliases
}

func findCamiePerformerPrediction(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, error) {
	prediction = normalizeCamiePrediction(prediction)
	characterName, disambiguation := camieCharacterIdentity(prediction)

	names := []string{prediction.Name}
	if prediction.RawName != "" && !strings.EqualFold(prediction.RawName, prediction.Name) {
		names = append(names, prediction.RawName)
	}
	if characterName != "" && !strings.EqualFold(characterName, prediction.Name) {
		names = append(names, characterName)
	}

	matches, err := repository.Performer.FindByNames(ctx, names, true)
	if err != nil {
		return nil, err
	}
	if disambiguation == "" {
		if len(matches) > 0 {
			return matches[0], nil
		}
		return nil, nil
	}

	for _, match := range matches {
		if strings.EqualFold(strings.TrimSpace(match.Disambiguation), disambiguation) {
			return match, nil
		}
	}
	for _, match := range matches {
		if strings.EqualFold(strings.TrimSpace(match.Name), prediction.Name) {
			return match, nil
		}
	}
	return nil, nil
}

func findCamieTag(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Tag, error) {
	for _, name := range []string{prediction.Name, prediction.RawName} {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		existing, err := repository.Tag.FindByName(ctx, name, true)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		existing, err = repository.Tag.FindByAlias(ctx, name, true)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	return nil, nil
}

func ensureCamieCopyrightRoot(ctx context.Context, repository models.Repository) (*models.Tag, error) {
	root, err := repository.Tag.FindByAlias(ctx, camieCopyrightRootAlias, true)
	if err != nil {
		return nil, err
	}
	if root != nil {
		return root, nil
	}

	root, err = repository.Tag.FindByName(ctx, camieCopyrightRootName, true)
	if err != nil {
		return nil, err
	}
	if root != nil {
		if err := root.LoadAliases(ctx, repository.Tag); err != nil {
			return nil, err
		}
		aliases := append([]string{}, root.Aliases.List()...)
		aliases = append(aliases, camieCopyrightRootAlias)
		if err := repository.Tag.UpdateAliases(ctx, root.ID, aliases); err != nil {
			return nil, err
		}
		return root, nil
	}

	newTag := models.CreateTagInput{Tag: &models.Tag{}}
	*newTag.Tag = models.NewTag()
	newTag.Name = camieCopyrightRootName
	newTag.Aliases = models.NewRelatedStrings([]string{camieCopyrightRootAlias})
	newTag.ParentIDs = models.NewRelatedIDs([]int{})
	newTag.ChildIDs = models.NewRelatedIDs([]int{})
	if err := tag.ValidateCreate(ctx, *newTag.Tag, repository.Tag); err != nil {
		return nil, err
	}
	if err := repository.Tag.Create(ctx, &newTag); err != nil {
		return nil, err
	}
	return newTag.Tag, nil
}

func findOrCreateCamieCopyright(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Tag, bool, error) {
	prediction = normalizeCamiePrediction(prediction)
	root, err := ensureCamieCopyrightRoot(ctx, repository)
	if err != nil {
		return nil, false, err
	}

	existing, err := findCamieTag(ctx, repository, prediction)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if err := existing.LoadParentIDs(ctx, repository.Tag); err != nil {
			return nil, false, err
		}
		parents := append([]int{}, existing.ParentIDs.List()...)
		found := false
		for _, id := range parents {
			if id == root.ID {
				found = true
				break
			}
		}
		if !found {
			parents = append(parents, root.ID)
			if err := repository.Tag.UpdateParentTags(ctx, existing.ID, parents); err != nil {
				return nil, false, err
			}
		}
		return existing, false, nil
	}

	newTag := models.CreateTagInput{Tag: &models.Tag{}}
	*newTag.Tag = models.NewTag()
	newTag.Name = prediction.Name
	newTag.Aliases = models.NewRelatedStrings(camieAliases(prediction))
	newTag.ParentIDs = models.NewRelatedIDs([]int{root.ID})
	newTag.ChildIDs = models.NewRelatedIDs([]int{})
	if err := tag.ValidateCreate(ctx, *newTag.Tag, repository.Tag); err != nil {
		return nil, false, err
	}
	if err := repository.Tag.Create(ctx, &newTag); err != nil {
		return nil, false, err
	}
	return newTag.Tag, true, nil
}

func findOrCreateCamiePerformerPrediction(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Performer, bool, error) {
	prediction = normalizeCamiePrediction(prediction)
	existing, err := findCamiePerformerPrediction(ctx, repository, prediction)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	characterName, disambiguation := camieCharacterIdentity(prediction)
	newPerformer := models.NewPerformer()
	newPerformer.Name = characterName
	newPerformer.Disambiguation = disambiguation
	newPerformer.Aliases = models.NewRelatedStrings(camieCharacterAliases(prediction, characterName))
	newPerformer.URLs = models.NewRelatedStrings([]string{})
	if err := performer.ValidateCreate(ctx, newPerformer, repository.Performer); err != nil {
		return nil, false, err
	}
	input := &models.CreatePerformerInput{Performer: &newPerformer}
	if err := repository.Performer.Create(ctx, input); err != nil {
		return nil, false, err
	}
	return &newPerformer, true, nil
}

func findOrCreateCamieStudioPrediction(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Studio, bool, error) {
	prediction = normalizeCamiePrediction(prediction)
	for _, name := range []string{prediction.Name, prediction.RawName} {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		existing, err := repository.Studio.FindByName(ctx, name, true)
		if err != nil {
			return nil, false, err
		}
		if existing != nil {
			return existing, false, nil
		}
	}

	newStudio := models.NewCreateStudioInput()
	newStudio.Name = prediction.Name
	newStudio.Aliases = models.NewRelatedStrings(camieAliases(prediction))
	newStudio.URLs = models.NewRelatedStrings([]string{})
	if err := studio.ValidateCreate(ctx, newStudio, repository.Studio); err != nil {
		return nil, false, err
	}
	if err := repository.Studio.Create(ctx, &newStudio); err != nil {
		return nil, false, err
	}
	return newStudio.Studio, true, nil
}

func findOrCreateCamieTagPrediction(ctx context.Context, repository models.Repository, prediction camietagger.Tag) (*models.Tag, bool, error) {
	prediction = normalizeCamiePrediction(prediction)
	existing, err := findCamieTag(ctx, repository, prediction)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	newTag := models.CreateTagInput{Tag: &models.Tag{}}
	*newTag.Tag = models.NewTag()
	newTag.Name = prediction.Name
	newTag.Aliases = models.NewRelatedStrings(camieAliases(prediction))
	newTag.ParentIDs = models.NewRelatedIDs([]int{})
	newTag.ChildIDs = models.NewRelatedIDs([]int{})
	if err := tag.ValidateCreate(ctx, *newTag.Tag, repository.Tag); err != nil {
		return nil, false, err
	}
	if err := repository.Tag.Create(ctx, &newTag); err != nil {
		return nil, false, err
	}
	return newTag.Tag, true, nil
}

func enrichCamiePredictionTargets(ctx context.Context, predictions []camietagger.Tag) []camietagger.Tag {
	repository := manager.GetInstance().Repository
	result := make([]camietagger.Tag, len(predictions))
	copy(result, predictions)

	_ = repository.WithTxn(ctx, func(ctx context.Context) error {
		for index := range result {
			prediction := normalizeCamiePrediction(result[index])
			var targetID int
			switch prediction.Category {
			case "character":
				performerEntity, _ := findCamiePerformerPrediction(ctx, repository, prediction)
				if performerEntity != nil {
					targetID = performerEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/performers/%d", targetID)
				} else {
					characterName, _ := camieCharacterIdentity(prediction)
					prediction.TargetPath = "/performers?q=" + url.QueryEscape(characterName)
				}
			case "artist":
				for _, name := range []string{prediction.Name, prediction.RawName} {
					if strings.TrimSpace(name) == "" {
						continue
					}
					studioEntity, _ := repository.Studio.FindByName(ctx, name, true)
					if studioEntity != nil {
						targetID = studioEntity.ID
						break
					}
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/studios/%d", targetID)
				} else {
					prediction.TargetPath = "/studios?q=" + url.QueryEscape(prediction.Name)
				}
			case "copyright":
				tagEntity, _ := findCamieTag(ctx, repository, prediction)
				if tagEntity != nil {
					targetID = tagEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/tags/%d", targetID)
				} else {
					prediction.TargetPath = "/copyrights?q=" + url.QueryEscape(prediction.Name)
				}
			default:
				tagEntity, _ := findCamieTag(ctx, repository, prediction)
				if tagEntity != nil {
					targetID = tagEntity.ID
				}
				if targetID > 0 {
					prediction.TargetPath = fmt.Sprintf("/tags/%d", targetID)
				} else {
					prediction.TargetPath = "/tags?q=" + url.QueryEscape(prediction.Name)
				}
			}
			prediction.TargetExists = targetID > 0
			result[index] = prediction
		}
		return nil
	})
	return result
}
