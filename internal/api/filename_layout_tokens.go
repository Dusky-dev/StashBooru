package api

import (
	"net/http"
	"path/filepath"
	"strings"
)

// filenameLayoutToken is one token of the tagging filename layout and the
// text it matched, in layout order.
type filenameLayoutToken struct {
	Token string `json:"token"`
	Value string `json:"value"`
}

type filenameLayoutTokensResponse struct {
	Layout   string                `json:"layout"`
	Filename string                `json:"filename"`
	Tokens   []filenameLayoutToken `json:"tokens"`
}

// filenameLayoutTokens matches the filename the filename tagging source
// reads against the configured layout. No match gives no tokens.
func filenameLayoutTokens(path, layout string) ([]filenameLayoutToken, error) {
	compiled, err := compileCamieFilenameLayout(layout)
	if err != nil {
		return nil, err
	}
	match := compiled.expression.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return []filenameLayoutToken{}, nil
	}
	tokens := make([]filenameLayoutToken, 0, len(compiled.groups))
	for index := 1; index < len(match); index++ {
		for token, group := range compiled.groups {
			if group == index {
				tokens = append(tokens, filenameLayoutToken{Token: token, Value: strings.TrimSpace(match[index])})
			}
		}
	}
	return tokens, nil
}

// ImageFilenameTokens shows how the tagging filename layout reads this
// Image's filename, for the details panel.
func (rs imageRoutes) ImageFilenameTokens(w http.ResponseWriter, r *http.Request) {
	path, err := imageTaggingPrimaryPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	config, err := loadCamieConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tokens, err := filenameLayoutTokens(path, config.FilenameLayout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeVisualSimilarityJSON(w, filenameLayoutTokensResponse{Layout: config.FilenameLayout, Filename: filepath.Base(path), Tokens: tokens})
}
