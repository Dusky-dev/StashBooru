package booru

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var filenameToken = regexp.MustCompile(`%([a-z_][a-z0-9_]*)%`)
var filenameTokens = map[string]bool{"id": true, "asset_id": true, "md5": true, "ext": true, "artist": true, "copyright": true, "character": true, "filename": true, "rating": true, "website": true, "websitename": true, "width": true, "height": true, "author": true, "authorid": true, "position": true, "score": true, "date": true, "current_date": true, "name": true, "title": true, "num": true}

func ValidateFilenameTemplate(format string) error {
	if format == "" {
		return nil
	}
	if len(format) > 512 || strings.HasPrefix(format, "/") || strings.ContainsAny(format, "\\:") || strings.IndexFunc(format, unicode.IsControl) >= 0 || !strings.HasSuffix(format, ".%ext%") || len(strings.Split(format, "/")) > 8 {
		return fmt.Errorf("use a relative template up to 512 bytes and eight components, ending in .%%ext%%")
	}
	for _, part := range strings.Split(format, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("filename template cannot contain empty or parent path components")
		}
	}
	matches := filenameToken.FindAllStringSubmatch(format, -1)
	for _, m := range matches {
		if !filenameTokens[m[1]] {
			return fmt.Errorf("unsupported filename token: %s", m[1])
		}
	}
	if strings.Contains(filenameToken.ReplaceAllString(format, ""), "%") {
		return fmt.Errorf("invalid filename token; use %%artist%%, %%copyright%%, %%character%%, %%id%%, %%md5%% and %%ext%%")
	}
	return nil
}
func filenameComponent(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, value)
	value = strings.Trim(value, " .")
	if value == "" || value == "." || value == ".." {
		return "unknown"
	}
	stem := strings.ToUpper(strings.Split(value, ".")[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9') {
		value = "_" + value
	}
	return value
}

// RenderFilename implements Grabber's common %token% vocabulary. Values never
// introduce directories: only literal template slashes create folder levels.
func RenderFilename(format string, s Source, p Post, md5, extension string, num int) (string, error) {
	if err := ValidateFilenameTemplate(format); err != nil {
		return "", err
	}
	if format == "" {
		return "", fmt.Errorf("empty template")
	}
	id := p.RemoteID
	if id == "" {
		id = p.ID
	}
	filename := p.Filename
	if filename == "" {
		if u, e := url.Parse(p.OriginalURL); e == nil {
			filename = path.Base(u.Path)
		}
	}
	filename = strings.TrimSuffix(filename, path.Ext(filename))
	website, _ := url.Parse(s.URL)
	values := map[string]string{"id": id, "asset_id": p.ID, "md5": md5, "ext": extension, "filename": filename, "rating": p.Rating, "website": website.Hostname(), "websitename": s.Name, "width": strconv.FormatInt(p.Width, 10), "height": strconv.FormatInt(p.Height, 10), "author": p.Author, "authorid": p.AuthorID, "position": strconv.Itoa(max(1, p.Position)), "score": strconv.FormatInt(p.Score, 10), "name": p.Title, "title": p.Title, "num": strconv.Itoa(num), "current_date": time.Now().UTC().Format("01-02-2006 15.04")}
	if date, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
		values["date"] = date.UTC().Format("01-02-2006 15.04")
	}
	for _, category := range []string{"artist", "copyright", "character"} {
		tags := []string{}
		for _, tag := range p.Tags {
			if tag.Category == category {
				value := tag.Label
				if value == "" {
					value = tag.Name
				}
				tags = append(tags, value)
			}
		}
		values[category] = strings.Join(tags, ", ")
	}
	parts := strings.Split(format, "/")
	for i, part := range parts {
		rendered := filenameToken.ReplaceAllStringFunc(part, func(token string) string { return filenameComponent(values[strings.Trim(token, "%")]) })
		parts[i] = filenameComponent(rendered)
		if len(parts[i]) > 200 {
			return "", fmt.Errorf("rendered filename component exceeds 200 bytes; choose shorter tokens")
		}
	}
	value := strings.Join(parts, "/")
	if len(value) > 1200 {
		return "", fmt.Errorf("rendered path is too long")
	}
	return value, nil
}
