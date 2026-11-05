package booru

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// SharedLinkSource is a dedicated server-owned identity for recognized share
// links found inside an existing Pawchive post. It is not selectable as a search
// source and does not authorize arbitrary browser-supplied URLs.
func SharedLinkSource() Source {
	return Source{
		ID: "shared-links", Name: "Shared links", Engine: "danbooru",
		URL: "https://www.dropbox.com",
		MediaHosts: []string{"dropboxusercontent.com"},
	}
}

var sharedLinkID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,200}$`)
var sharedLinkKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,200}$`)

func safeSharedFilename(raw string) string {
	name, err := url.PathUnescape(path.Base(raw))
	if err != nil || name == "." || name == "/" || len(name) > 240 ||
		strings.ContainsAny(name, "\\\r\n\x00") {
		return ""
	}
	return name
}

func sharedFormat(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp", "tif", "tiff", "jxl", "avif",
		"mp4", "webm", "zip", "cbz", "7z":
		return ext
	}
	return ""
}

// ValidMegaSharedFileURL limits the external decryption helper to a complete,
// unencrypted-public file link supplied by the user; folder links are never
// expanded or downloaded implicitly.
func ValidMegaSharedFileURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "mega.nz" ||
		u.Port() != "" || u.User != nil || u.RawQuery != "" {
		return false
	}
	if strings.HasPrefix(u.Path, "/file/") &&
		sharedLinkID.MatchString(strings.TrimPrefix(u.Path, "/file/")) &&
		sharedLinkKey.MatchString(u.Fragment) {
		return true
	}
	parts := strings.Split(u.Fragment, "!")
	return (u.Path == "/" || u.Path == "") && len(parts) == 3 &&
		parts[0] == "" && sharedLinkID.MatchString(parts[1]) &&
		sharedLinkKey.MatchString(parts[2])
}

// SharedLinkPost validates a recognized Dropbox or MEGA share and produces one
// virtual media item. This is called on URLs extracted from Pawchive content;
// it never performs a network request.
func SharedLinkPost(raw string) (Post, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\x00") {
		return Post{}, fmt.Errorf("invalid Dropbox or MEGA share link")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil ||
		u.Port() != "" || u.Opaque != "" {
		return Post{}, fmt.Errorf("share link must use public HTTPS without credentials or a custom port")
	}
	post := Post{SourceID: SharedLinkSource().ID, Tags: []Tag{}}
	var original string
	switch strings.ToLower(u.Hostname()) {
	case "dropbox.com", "www.dropbox.com":
		if u.Fragment != "" {
			return Post{}, fmt.Errorf("Dropbox share links cannot contain fragments")
		}
		parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
		folder := false
		switch {
		case len(parts) >= 3 && parts[0] == "scl" && parts[1] == "fi" && sharedLinkID.MatchString(parts[2]):
			// Current Dropbox file-sharing links.
		case len(parts) >= 3 && parts[0] == "scl" && parts[1] == "fo" && sharedLinkID.MatchString(parts[2]):
			folder = true
		case len(parts) >= 2 && parts[0] == "s" && sharedLinkID.MatchString(parts[1]):
		case len(parts) >= 2 && parts[0] == "sh" && sharedLinkID.MatchString(parts[1]):
			folder = true
		default:
			return Post{}, fmt.Errorf("unsupported Dropbox share link; use a /s/, /sh/, /scl/fi/ or /scl/fo/ link")
		}
		query := u.Query()
		for key := range query {
			if key != "rlkey" && key != "dl" && key != "raw" {
				return Post{}, fmt.Errorf("Dropbox link contains an unsupported query parameter")
			}
		}
		rlkey := query.Get("rlkey")
		if rlkey != "" && !sharedLinkID.MatchString(rlkey) {
			return Post{}, fmt.Errorf("invalid Dropbox sharing key")
		}
		out := &url.URL{Scheme: "https", Host: "www.dropbox.com", Path: u.Path}
		q := url.Values{"dl": {"1"}}
		if rlkey != "" {
			q.Set("rlkey", rlkey)
		}
		out.RawQuery = q.Encode()
		original = out.String()
		post.PostURL = (&url.URL{Scheme: "https", Host: "www.dropbox.com", Path: u.Path}).String()
		post.Filename = safeSharedFilename(u.EscapedPath())
		if folder {
			post.Format = "zip"
			post.Filename = "Dropbox folder.zip"
			post.MediaNotice = "Linked Dropbox folder: select a supported image or video from the bounded ZIP archive below."
		} else {
			post.Format = sharedFormat(post.Filename)
			if post.Format == "" || post.Format == "zip" || post.Format == "cbz" || post.Format == "7z" {
				return Post{}, fmt.Errorf("Dropbox share does not identify a supported image or video")
			}
			if post.Format != "mp4" && post.Format != "webm" {
				post.PreviewURL, post.SampleURL = original, original
			}
		}
		post.Title = post.Filename
	case "mega.nz":
		if !ValidMegaSharedFileURL(raw) {
			return Post{}, fmt.Errorf("not a supported public MEGA file link")
		}
		original = raw
		public := *u
		public.Fragment = ""
		post.PostURL = public.String()
		post.Title = "MEGA shared file"
		post.MediaNotice = "Linked MEGA file; select Download to Stash to decrypt it. The Stash server requires megatools."
	default:
		return Post{}, fmt.Errorf("supported link hosts are Dropbox and MEGA only")
	}
	sum := sha256.Sum256([]byte(original))
	post.ID = "p" + hex.EncodeToString(sum[:16])
	post.OriginalURL = original
	post.DownloadAvailable = true
	return post, nil
}

const maxLinkedMedia = 16

// Shared links may occur as HTML hrefs, bare URLs, Markdown links or BBCode
// inside Pawchive's content. A bounded scan of provider text is sufficient:
// never crawl pages or follow arbitrary external redirects during search.
var sharedLinkPattern = regexp.MustCompile(`https://[^\\s<>"']+`)

// LinkedMediaFromPawchiveContent discovers up to 16 valid shared files/folders,
// without fetching any of them. MEGA folder links are reported but not fetched:
// streaming a folder would be an unbounded download.
func LinkedMediaFromPawchiveContent(content, pawchivePostURL string) ([]Post, int) {
	if len(content) > 64<<10 {
		content = content[:64<<10]
	}
	content = html.UnescapeString(content)
	found := make([]Post, 0)
	seen := make(map[string]bool)
	folders := 0
	for _, candidate := range sharedLinkPattern.FindAllString(content, 80) {
		candidate = strings.TrimRight(candidate, ".,;:!?)]}")
		if len(candidate) > 2048 {
			continue
		}
		u, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "mega.nz" && strings.HasPrefix(u.Path, "/folder/") &&
			sharedLinkID.MatchString(strings.TrimPrefix(u.Path, "/folder/")) &&
			sharedLinkKey.MatchString(u.Fragment) {
			if !seen[candidate] {
				seen[candidate] = true
				folders++
			}
			continue
		}
		post, err := SharedLinkPost(candidate)
		if err != nil || seen[post.ID] {
			continue
		}
		seen[post.ID] = true
		post.PostURL = pawchivePostURL // retain the originating post as provenance
		if host == "mega.nz" {
			post.Title = "MEGA linked file"
		} else if post.Format == "zip" {
			post.Title = "Dropbox linked folder"
		} else {
			post.Title = "Dropbox: " + post.Filename
		}
		found = append(found, post)
		if len(found) == maxLinkedMedia {
			break
		}
	}
	return found, folders
}
