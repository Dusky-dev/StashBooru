package booru

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxIconBytes bounds a site icon download.
const MaxIconBytes = 256 << 10

var iconTypes = map[string]bool{
	"image/x-icon": true, "image/vnd.microsoft.icon": true, "image/png": true,
	"image/gif": true, "image/jpeg": true, "image/webp": true,
}

// ErrNoIcon reports that a site has no usable /favicon.ico.
var ErrNoIcon = errors.New("site has no usable icon")

var iconClient = &http.Client{
	Transport: &http.Transport{
		// Same public-address boundary as provider requests.
		DialContext:           publicDialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	},
	Timeout: 10 * time.Second,
}

// FetchIcon downloads the configured site's /favicon.ico. Only the source's
// own URL is used; the browser never supplies a host.
func FetchIcon(ctx context.Context, source Source) ([]byte, string, error) {
	return fetchIcon(ctx, iconClient, source.URL)
}

func sameSite(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return true
	}
	site := func(host string) string {
		parts := strings.Split(host, ".")
		if len(parts) <= 2 {
			return host
		}
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return site(a) == site(b)
}

func fetchIcon(ctx context.Context, client *http.Client, base string) ([]byte, string, error) {
	root, err := url.Parse(base)
	if err != nil || (root.Scheme != "https" && root.Scheme != "http") || root.Host == "" {
		return nil, "", fmt.Errorf("invalid source URL")
	}
	target := &url.URL{Scheme: root.Scheme, Host: root.Host, Path: "/favicon.ico"}
	limited := *client
	limited.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many icon redirects")
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return errors.New("icon redirect scheme refused")
		}
		host, _, err := net.SplitHostPort(req.URL.Host)
		if err != nil {
			host = req.URL.Host
		}
		rootHost, _, err := net.SplitHostPort(root.Host)
		if err != nil {
			rootHost = root.Host
		}
		if !sameSite(host, rootHost) {
			return errors.New("icon redirect left the site")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := limited.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNoIcon, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: status %d", ErrNoIcon, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxIconBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNoIcon, err)
	}
	if len(data) == 0 || len(data) > MaxIconBytes {
		return nil, "", fmt.Errorf("%w: size %d", ErrNoIcon, len(data))
	}
	// Trust the bytes, not the server's Content-Type; SVG is never served.
	contentType := http.DetectContentType(data)
	if !iconTypes[contentType] {
		return nil, "", fmt.Errorf("%w: type %s", ErrNoIcon, contentType)
	}
	return data, contentType, nil
}
