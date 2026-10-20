package booru

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxTagCSVBytes = 64 << 20

type Suggestion struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int64  `json:"count"`
	Alias    string `json:"alias,omitempty"`
}
type tagKey struct {
	key string
	tag int
}
type TagIndex struct {
	tags []Suggestion
	keys []tagKey
}

func (t *TagIndex) Count() int { return len(t.tags) }

// Category uses only an exact canonical name, never a prefix or alias guess.
func (t *TagIndex) Category(name string) string {
	key := strings.ToLower(name)
	at := sort.Search(len(t.keys), func(i int) bool { return t.keys[i].key >= key })
	for ; at < len(t.keys) && t.keys[at].key == key; at++ {
		tag := t.tags[t.keys[at].tag]
		if strings.EqualFold(tag.Name, name) {
			return tag.Category
		}
	}
	return ""
}

// ReadTagCSV accepts the tagcomplete database format: name,type,count,aliases.
// Header rows from Danbooru tag exports are accepted when the required columns
// name/category/post_count are present. No catalogue metadata is modified.
// Repeated names retain the busiest row's count/category (first row on ties),
// and their aliases are combined within the normal per-tag/index limits.
func ReadTagCSV(input io.Reader) (*TagIndex, error) { return ReadTagCSVForEngine(input, "danbooru") }

func ReadTagCSVForEngine(input io.Reader, engine string) (*TagIndex, error) {
	reader := csv.NewReader(io.LimitReader(input, MaxTagCSVBytes+1))
	reader.FieldsPerRecord = -1
	index := &TagIndex{}
	seen := map[string]int{}
	aliasesByTag := map[int][]string{}
	nameCol, typeCol, countCol, aliasCol := 0, 1, 2, 3
	maxTags, maxKeys := 1_000_000, 2_000_000
	if engine == "gelbooru" {
		// Rule34/Gelbooru category exports may exceed a million unique names.
		// Only raise these caps for explicitly installed Gelbooru-family CSVs;
		// the 64 MiB input limit and per-tag alias bounds still apply.
		maxTags, maxKeys = 2_000_000, 3_000_000
	}
	numeric := map[string]string{"0": "general", "1": "artist", "3": "copyright", "4": "character", "5": "meta"}
	if engine == "gelbooru" {
		// Gelbooru/Rule34's type 2 marks deprecated tags. Keep them in
		// the index without misclassifying them as copyright.
		numeric["2"] = "unknown"
	}
	if engine == "e621" {
		numeric = map[string]string{"0": "general", "1": "artist", "2": "contributor", "3": "copyright", "4": "character", "5": "species", "6": "invalid", "7": "meta", "8": "lore"}
	}
	row := 0
	for {
		fields, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || reader.InputOffset() > MaxTagCSVBytes {
			return nil, fmt.Errorf("invalid CSV or database exceeds 64 MiB")
		}
		row++
		if row == 1 {
			fields[0] = strings.TrimPrefix(fields[0], "\ufeff")
			// Provider exports may place type/count before name (for example
			// Rule34's type,count,name,ambiguous,id snapshot). Identify a
			// header by column names rather than requiring name in column 1.
			hasName, hasType := false, false
			for _, field := range fields {
				switch field {
				case "name", "tag":
					hasName = true
				case "type", "category":
					hasType = true
				}
			}
			if hasName && hasType {
				nameCol, typeCol, countCol, aliasCol = -1, -1, -1, -1
				for col, value := range fields {
					switch value {
					case "name", "tag":
						nameCol = col
					case "type", "category":
						typeCol = col
					case "count", "post_count":
						countCol = col
					case "aliases":
						aliasCol = col
					}
				}
				if nameCol < 0 || typeCol < 0 {
					return nil, fmt.Errorf("CSV header requires name and category/type")
				}
				continue
			}
		}
		get := func(col int) string {
			if col < 0 || col >= len(fields) {
				return ""
			}
			return strings.TrimSpace(fields[col])
		}
		name := get(nameCol)
		if !validTagText(name) {
			return nil, fmt.Errorf("invalid tag name at row %d: expected 1–256 UTF-8 bytes without control characters", row)
		}
		category := get(typeCol)

		if name, ok := numeric[category]; ok {
			category = name
		}
		if !slices.Contains([]string{"general", "artist", "copyright", "character", "meta", "invalid", "lore", "species", "contributor", "circle", "faults", "unknown"}, category) {
			return nil, fmt.Errorf("unsupported tag category at row %d", row)
		}
		var count int64
		if raw := get(countCol); raw != "" {
			count, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || count < 0 {
				return nil, fmt.Errorf("invalid post count at row %d", row)
			}
		}
		if strings.ContainsFunc(get(aliasCol), unicode.IsControl) {
			return nil, fmt.Errorf("invalid alias at row %d", row)
		}
		aliases := strings.Split(get(aliasCol), ",")
		if len(aliases) > 32 {
			return nil, fmt.Errorf("too many aliases at row %d", row)
		}
		id, duplicate := seen[name]
		if !duplicate {
			if len(index.tags) >= maxTags {
				return nil, fmt.Errorf("database exceeds %d unique tags at row %d", maxTags, row)
			}
			id = len(index.tags)
			index.tags = append(index.tags, Suggestion{Name: name, Category: category, Count: count})
			seen[name] = id
			index.keys = append(index.keys, tagKey{strings.ToLower(name), id})
		} else if count > index.tags[id].Count {
			// Paged provider snapshots can repeat names with stale counts/types.
			// Keep one suggestion using the busiest row; ties keep the first row.
			index.tags[id].Count, index.tags[id].Category = count, category
		}
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if !validTagText(alias) {
				return nil, fmt.Errorf("invalid alias at row %d", row)
			}
			key := strings.ToLower(alias)
			if key == strings.ToLower(name) || slices.Contains(aliasesByTag[id], key) {
				continue
			}
			if len(aliasesByTag[id]) >= 32 {
				return nil, fmt.Errorf("too many aliases for tag at row %d", row)
			}
			aliasesByTag[id] = append(aliasesByTag[id], key)
			index.keys = append(index.keys, tagKey{key, id})
		}
		if len(index.keys) > maxKeys {
			return nil, fmt.Errorf("database exceeds two million names/aliases")
		}
	}
	if len(index.tags) == 0 {
		return nil, fmt.Errorf("CSV contains no tags")
	}
	sort.Slice(index.keys, func(i, j int) bool { return index.keys[i].key < index.keys[j].key })
	return index, nil
}
func validTagText(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}
func (t *TagIndex) Suggest(ctx context.Context, prefix string) []Suggestion {
	ret := []Suggestion{}
	if ctx.Err() != nil {
		return ret
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 || len(prefix) > 80 || !validTagText(prefix) {
		return ret
	}
	start := sort.Search(len(t.keys), func(i int) bool { return t.keys[i].key >= prefix })
	for i := start; i < len(t.keys) && strings.HasPrefix(t.keys[i].key, prefix); i++ {
		if i%1024 == 0 && ctx.Err() != nil {
			return []Suggestion{}
		}
		key := t.keys[i]
		tag := t.tags[key.tag]
		if key.key != strings.ToLower(tag.Name) {
			tag.Alias = key.key
		}
		if slices.ContainsFunc(ret, func(s Suggestion) bool { return s.Name == tag.Name }) {
			continue
		}
		at := sort.Search(len(ret), func(i int) bool {
			return ret[i].Count < tag.Count || (ret[i].Count == tag.Count && ret[i].Name > tag.Name)
		})
		if at < 20 {
			ret = slices.Insert(ret, at, tag)
			if len(ret) > 20 {
				ret = ret[:20]
			}
		}
	}
	return ret
}
