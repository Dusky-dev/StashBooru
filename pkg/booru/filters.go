package booru

import (
	"net/url"
	"strings"
)

type SearchOptions struct {
	Sort   string
	Rating string
}

func (o SearchOptions) Validate() bool {
	return (o.Sort == "" || o.Sort == "newest" || o.Sort == "popular" || o.Sort == "updated") &&
		(o.Rating == "" || o.Rating == "safe" || o.Rating == "sensitive" || o.Rating == "questionable" || o.Rating == "explicit")
}

func (s Source) filterQuery(query string, options SearchOptions) (string, error) {
	if !options.Validate() {
		return "", &Error{Code: "invalid-query", Message: "Invalid sort or rating filter."}
	}
	if options.Sort == "updated" && (s.Engine == "gelbooru" || s.Engine == "philomena") {
		return "", &Error{Code: "unsupported-filter", Message: "Updated sorting is unavailable for this source. Choose provider default, newest or popular."}
	}
	if options.Rating == "sensitive" && !s.SupportsSensitiveRating() {
		return "", &Error{Code: "unsupported-filter", Message: "This source does not expose a separate Sensitive rating."}
	}
	if s.Engine == "philomena" {
		if options.Rating != "" {
			rating := options.Rating
			if rating == "questionable" {
				rating = "questionable OR suggestive"
			}
			query = "(" + query + ") AND (" + rating + ")"
		}
		return query, nil
	}
	if options.Sort != "" {
		order := "id_desc"
		if options.Sort == "popular" {
			order = "score"
		}
		if options.Sort == "updated" {
			order = "change"
			if s.Engine == "moebooru" {
				order = "change_desc"
			}
		}
		if s.Engine == "gelbooru" {
			query += " sort:" + strings.TrimSuffix(order, "_desc") + ":desc"
		} else {
			query += " order:" + order
		}
	}
	if options.Rating != "" {
		rating := map[string]string{"safe": "s", "sensitive": "s", "questionable": "q", "explicit": "e"}[options.Rating]
		if s.Engine == "danbooru" && options.Rating == "safe" {
			rating = "g"
		}
		if s.Engine == "gelbooru" && options.Rating == "safe" {
			rating = "general"
			if strings.Contains(s.URL, "rule34.xxx") || strings.Contains(s.URL, "safebooru.org") {
				rating = "safe"
			}
		}
		if s.Engine == "gelbooru" && options.Rating == "sensitive" {
			rating = "sensitive"
		}
		query += " rating:" + rating
	}
	return query, nil
}

func (s Source) SupportsSensitiveRating() bool {
	if s.Engine == "danbooru" {
		return true
	}
	if s.Engine != "gelbooru" {
		return false
	}
	u, err := url.Parse(s.URL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "rule34.xxx", "api.rule34.xxx", "safebooru.org", "www.safebooru.org":
		return false
	}
	return true
}
