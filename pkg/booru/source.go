// Package booru provides bounded read-only searches across booru API families.
// Remote post identity never substitutes for a native catalogue media ID.
package booru

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const MaxPage = 750
const MaxLimit = 50

var sourceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Source is server-owned configuration. Credentials must never be serialized
// into the public Sources response, browser URLs, or transport errors.
type Source struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Engine     string   `json:"engine"`
	URL        string   `json:"url"`
	MediaHosts []string `json:"mediaHosts,omitempty"`
	Login      string   `json:"login,omitempty"`
	APIKey     string   `json:"apiKey,omitempty"`
	UserID     string   `json:"userID,omitempty"`
	Disabled   bool     `json:"disabled,omitempty"`
}

type PublicSource struct {
	Instance              string `json:"instance"`
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Engine                string `json:"engine"`
	URL                   string `json:"url"`
	QueryHelp             string `json:"queryHelp"`
	CredentialsConfigured bool   `json:"credentialsConfigured"`
	Authentication        string `json:"authentication"`
	CategorizedTags       bool   `json:"categorizedTags"`
}

func Defaults() []Source {
	return []Source{
		{ID: "danbooru", Name: "Danbooru", Engine: "danbooru", URL: "https://danbooru.donmai.us", MediaHosts: []string{"donmai.us"}},
		{ID: "safebooru-donmai", Name: "Safebooru (Danbooru)", Engine: "danbooru", URL: "https://safebooru.donmai.us", MediaHosts: []string{"donmai.us"}},
		{ID: "gelbooru", Name: "Gelbooru", Engine: "gelbooru", URL: "https://gelbooru.com"},
		{ID: "safebooru", Name: "Safebooru", Engine: "gelbooru", URL: "https://safebooru.org"},
		{ID: "rule34", Name: "Rule34", Engine: "gelbooru", URL: "https://rule34.xxx"},
		{ID: "yandere", Name: "Yande.re", Engine: "moebooru", URL: "https://yande.re"},
		{ID: "konachan", Name: "Konachan", Engine: "moebooru", URL: "https://konachan.com"},
		{ID: "e621", Name: "e621", Engine: "e621", URL: "https://e621.net"},
		{ID: "e926", Name: "e926", Engine: "e621", URL: "https://e926.net", MediaHosts: []string{"e621.net"}},
		{ID: "derpibooru", Name: "Derpibooru", Engine: "philomena", URL: "https://derpibooru.org", MediaHosts: []string{"derpicdn.net"}},
	}
}

// Configure overlays complete source definitions on presets. An ID-only disabled
// entry removes a preset. No arbitrary scripts, HTML scraping or browser secrets.
func Configure(data []byte) ([]Source, error) {
	sources := Defaults()
	if len(data) > 0 {
		var configured []Source
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&configured); err != nil {
			return nil, fmt.Errorf("invalid booru-browser.json: %w", err)
		}
		if configured == nil || decoder.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("booru-browser.json must contain one source array")
		}
		if len(configured) > 64 {
			return nil, fmt.Errorf("booru-browser.json supports at most 64 entries")
		}
		seen := map[string]bool{}
		for _, source := range configured {
			if !sourceID.MatchString(source.ID) || seen[source.ID] {
				return nil, fmt.Errorf("source IDs must be unique lowercase letters, digits, underscores or hyphens")
			}
			seen[source.ID] = true
			sources = slices.DeleteFunc(sources, func(existing Source) bool { return existing.ID == source.ID })
			if !source.Disabled {
				sources = append(sources, source)
			}
		}
	}
	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return nil, fmt.Errorf("source %s: %w", source.ID, err)
		}
	}
	return sources, nil
}

func (s Source) Validate() error {
	if !sourceID.MatchString(s.ID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 120 {
		return fmt.Errorf("invalid source ID or name")
	}
	if !slices.Contains([]string{"danbooru", "gelbooru", "moebooru", "e621", "philomena"}, s.Engine) {
		return fmt.Errorf("unsupported API engine")
	}
	if len(s.URL) > 2048 || len(s.MediaHosts) > 64 {
		return fmt.Errorf("source URL is limited to 2048 bytes and at most 64 image hosts")
	}
	u, err := url.Parse(s.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !publicHostname(u.Hostname()) || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("source URL must be public HTTPS without credentials, query, fragment or custom port")
	}
	for _, host := range s.MediaHosts {
		if len(host) > 253 || !publicHostname(host) || strings.ContainsAny(host, "/:@*?#") {
			return fmt.Errorf("mediaHosts must contain public hostnames without wildcards")
		}
	}
	for _, credential := range []string{s.APIKey, s.Login, s.UserID} {
		if len(credential) > 4096 || strings.ContainsAny(credential, "\r\n\x00") {
			return fmt.Errorf("credentials must be at most 4096 bytes without control characters")
		}
	}
	if (s.Engine == "gelbooru" && s.Login != "") || (s.Engine != "gelbooru" && s.UserID != "") || (s.Engine == "philomena" && s.Login != "") {
		return fmt.Errorf("credentials do not match the selected engine")
	}
	if s.Engine == "moebooru" && (s.Login != "" || s.APIKey != "" || s.UserID != "") {
		return fmt.Errorf("moebooru supports anonymous search only in this version")
	}
	if s.Engine == "gelbooru" && ((s.APIKey == "") != (s.UserID == "")) {
		return fmt.Errorf("DAPI credentials need both apiKey and userID")
	}
	if (s.Engine == "danbooru" || s.Engine == "e621") && ((s.APIKey == "") != (s.Login == "")) {
		return fmt.Errorf("API credentials need both apiKey and login")
	}
	return nil
}

func (s Source) Public() PublicSource {
	help := "Space-separated tags; underscores inside names. Native provider syntax and account search limits apply."
	auth := "Optional login and API key, configured in Source settings."
	switch s.Engine {
	case "gelbooru":
		auth = "API key and user ID may be required; configure them in Source settings."
	case "moebooru":
		auth = "Anonymous search; legacy password login is not supported."
	case "philomena":
		help = "Native Philomena syntax: tags may contain spaces; combine tags with commas or AND."
		auth = "Optional API key in Source settings; provider content filters still apply."
	}
	return PublicSource{Instance: s.Instance(), ID: s.ID, Name: s.Name, Engine: s.Engine, URL: s.URL, QueryHelp: help,
		CredentialsConfigured: s.APIKey != "", Authentication: auth, CategorizedTags: s.Engine == "danbooru" || s.Engine == "e621"}
}

func publicHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return publicAddress(addr)
	}
	return !strings.ContainsAny(host, " \\%\t\r\n")
}

var reservedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2002::/16"),
}

func publicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsGlobalUnicast() && !addr.IsPrivate() && !addr.IsLoopback() && !addr.IsLinkLocalUnicast() &&
		!slices.ContainsFunc(reservedNetworks, func(prefix netip.Prefix) bool { return prefix.Contains(addr) })
}

func (s Source) mediaURL(raw string) string {
	if raw == "" || len(raw) > 8192 {
		return ""
	}
	base, _ := url.Parse(strings.TrimRight(s.URL, "/") + "/")
	u, err := base.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || !publicHostname(u.Hostname()) || (u.Port() != "" && u.Port() != "443") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	hosts := append([]string{base.Hostname()}, s.MediaHosts...)
	if !slices.ContainsFunc(hosts, func(allowed string) bool {
		return host == strings.ToLower(allowed) || strings.HasSuffix(host, "."+strings.ToLower(allowed))
	}) {
		return ""
	}
	for _, key := range []string{"api_key", "key", "token", "password", "password_hash", "login", "user_id"} {
		if u.Query().Has(key) {
			return ""
		}
	}
	u.Fragment = ""
	return u.String()
}

// Instance excludes credentials and display IDs. Changing the API engine or URL
// isolates review history; aliases of the same normalized instance share it.
func (s Source) Instance() string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Port() != "" && u.Port() != "443" {
		host += ":" + u.Port()
	}
	sum := sha256.Sum256([]byte(s.Engine + "\nhttps://" + host + strings.TrimRight(u.EscapedPath(), "/")))
	return hex.EncodeToString(sum[:])
}
