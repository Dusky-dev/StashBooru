package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath" 
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/job"
)

// Public internet URLs only. Dial-time resolution prevents DNS rebinding,
// and the same rules are applied to every redirect. Do not use a host proxy:
// proxy resolution could otherwise bypass the address safety check.
var deniedRemotePrefixes = func() []netip.Prefix {
	values := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "2001:db8::/32",
	}
	ret := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		ret = append(ret, netip.MustParsePrefix(v))
	}
	return ret
}()

func mediaUploadPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range deniedRemotePrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func parseMediaUploadURL(value string) (*url.URL, error) {
	if len(value) > 4096 {
		return nil, fmt.Errorf("URL is too long")
	}
	u, err := url.Parse(value)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") ||
		u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return nil, fmt.Errorf("provide a public http or https image/video URL")
	}
	port := u.Port()
	if port != "" && !((u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80")) {
		return nil, fmt.Errorf("only the standard HTTP/HTTPS ports are allowed")
	}
	if u.Fragment != "" {
		u.Fragment = ""
	}
	return u, nil
}

func newMediaUploadRemoteClient() *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DisableKeepAlives: true,
		DisableCompression: true,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout: 12 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid remote host")
			}
			resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(resolved) == 0 {
				return nil, fmt.Errorf("remote hostname could not be resolved")
			}
			// Reject mixed public/private DNS answers instead of choosing the
			// public answer today and risking a private answer on a later call.
			for _, addr := range resolved {
				ip, ok := netip.AddrFromSlice(addr.IP)
				if !ok || !mediaUploadPublicIP(ip) {
					return nil, fmt.Errorf("private or reserved remote address is not allowed")
				}
			}
			dialer := net.Dialer{Timeout: 12 * time.Second, KeepAlive: -1}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0].IP.String(), port))
		},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := parseMediaUploadURL(req.URL.String())
			return err
		},
	}
}

func mediaUploadNameFromURL(u *url.URL, header http.Header) (string, error) {
	if cd := header.Get("Content-Disposition"); cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			if name, err := safeMediaUploadName(params["filename"]); err == nil {
				return name, nil
			}
		}
	}
	if name, err := safeMediaUploadName(path.Base(u.Path)); err == nil {
		return name, nil
	}
	typ, _, _ := mime.ParseMediaType(header.Get("Content-Type"))
	exts := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif",
		"image/webp": ".webp", "image/avif": ".avif", "image/jxl": ".jxl",
		"image/bmp": ".bmp", "image/tiff": ".tif", "video/mp4": ".mp4",
		"video/webm": ".webm", "video/quicktime": ".mov",
		"video/x-matroska": ".mkv", "video/x-msvideo": ".avi",
	}
	ext := exts[typ]
	if ext == "" {
		return "", fmt.Errorf("remote URL has no supported media filename or content type")
	}
	return "download" + ext, nil
}

func mediaUploadURLResponseAllowed(response *http.Response) error {
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("remote server returned HTTP %d", response.StatusCode)
	}
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentType == "text/html" || contentType == "application/json" ||
		contentType == "text/plain" || contentType == "application/xml" {
		return fmt.Errorf("URL returned a web page or API response instead of media")
	}
	return nil
}

type mediaUploadURLRequest struct {
	URL         string `json:"url"`
	Destination string `json:"destination"`
	ArtistIDs   string `json:"artistIDs"`
	GalleryIDs  string `json:"galleryIDs"`
	TagIDs      string `json:"tagIDs"`
}

type mediaUploadStreamEvent struct {
	Type    string              `json:"type"`
	Loaded  int64               `json:"loaded,omitempty"`
	Total   int64               `json:"total,omitempty"`
	Message string              `json:"message,omitempty"`
	Result  *booruLibraryResult `json:"result,omitempty"`
}

func writeMediaUploadEvent(w http.ResponseWriter, e mediaUploadStreamEvent) {
	_ = json.NewEncoder(w).Encode(e)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func copyMediaUploadRemote(w http.ResponseWriter, dest io.Writer, source io.Reader, total int64) (int64, error) {
	buf := make([]byte, 256*1024)
	var loaded int64
	var lastSent time.Time
	for {
		n, err := source.Read(buf)
		if n > 0 {
			written, writeErr := dest.Write(buf[:n])
			loaded += int64(written)
			if writeErr != nil {
				return loaded, writeErr
			}
			if written != n {
				return loaded, io.ErrShortWrite
			}
			if time.Since(lastSent) >= 200*time.Millisecond {
				writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "progress", Loaded: loaded, Total: total})
				lastSent = time.Now()
			}
		}
		if errors.Is(err, io.EOF) {
			writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "progress", Loaded: loaded, Total: total})
			return loaded, nil
		}
		if err != nil {
			return loaded, err
		}
	}
}

func (rs booruBrowserRoutes) ImportMediaURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !mediaUploadSameOrigin(r) {
		writeBooruError(w, http.StatusForbidden, "origin", "URL imports must originate in StashBooru.")
		return
	}
	var input mediaUploadURLRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input); err != nil {
		writeBooruError(w, http.StatusBadRequest, "url", "Invalid URL import request.")
		return
	}
	u, err := parseMediaUploadURL(input.URL)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "url", err.Error())
		return
	}
	destination, err := rs.destination(input.Destination)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "directory", err.Error())
		return
	}
	artistIDs, err := parseMediaUploadIDs(input.ArtistIDs, 32)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
		return
	}
	galleryIDs, err := parseMediaUploadIDs(input.GalleryIDs, 32)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
		return
	}
	tagIDs, err := parseMediaUploadIDs(input.TagIDs, 64)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
		return
	}
	if err := validateMediaUploadMetadata(r.Context(), rs.repository, artistIDs, galleryIDs, tagIDs); err != nil {
		writeBooruError(w, http.StatusBadRequest, "metadata", err.Error())
		return
	}
	if rs.jobs == nil {
		writeBooruError(w, http.StatusServiceUnavailable, "import", "Library import service unavailable.")
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "url", "Invalid remote URL.")
		return
	}
	request.Header.Set("Accept", "image/*, video/*;q=0.9, application/octet-stream;q=0.5")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := newMediaUploadRemoteClient().Do(request)
	if err != nil {
		writeBooruError(w, http.StatusBadGateway, "url", fmt.Sprintf("Could not securely fetch remote media: %v", err))
		return
	}
	defer response.Body.Close()
	if err := mediaUploadURLResponseAllowed(response); err != nil {
		writeBooruError(w, http.StatusBadGateway, "url", err.Error())
		return
	}
	name, err := mediaUploadNameFromURL(response.Request.URL, response.Header)
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "url", err.Error())
		return
	}
	staged, err := os.CreateTemp(destination, ".stash-upload-*.part")
	if err != nil {
		writeBooruError(w, http.StatusBadRequest, "directory", "Cannot write to selected library folder.")
		return
	}
	stage := staged.Name()
	defer os.Remove(stage)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	loaded, err := copyMediaUploadRemote(w, staged, response.Body, response.ContentLength)
	if err == nil {
		err = staged.Sync()
	}
	closeErr := staged.Close()
	if err != nil || closeErr != nil || loaded == 0 {
		writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "error", Message: "Remote download failed or was empty. Nothing was imported."})
		return
	}
	destination, err = rs.destination(destination)
	if err != nil {
		writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "error", Message: "Destination folder changed during download."})
		return
	}
	published, err := publishMediaUpload(stage, destination, name)
	if err != nil {
		writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "error", Message: err.Error()})
		return
	}
	writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "processing", Message: "Download complete; importing into Stash"})
	type outcome struct {
		result booruLibraryResult
		err error
	}
	done := make(chan outcome, 1)
	rs.jobs.Add(r.Context(), "Import downloaded media: "+name, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		result, e := importUploadedMedia(ctx, progress, published, artistIDs, galleryIDs, tagIDs)
		done <- outcome{result: result, err: e}
		return e
	}))
	select {
	case completed := <-done:
		if completed.err != nil {
			writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "error", Message: completed.err.Error()})
			return
		}
		writeMediaUploadEvent(w, mediaUploadStreamEvent{Type: "complete", Result: &completed.result})
	case <-r.Context().Done():
		// The uploaded original is complete on disk and can be scanned again.
		return
	}
}

