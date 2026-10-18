package booru

import (
	"context"
	"crypto/md5" //nolint:gosec // Provider content checksum, not authentication.
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MediaRequest can only use a provider URL validated against the source's CDN
// allowlist. Callers must obtain it from a server-owned search snapshot.
type MediaRequest struct {
	URL           string
	Limit         int64
	ExpectedBytes int64
	ExpectedMD5   string
	ImagesOnly    bool
}

type MediaInfo struct {
	ContentType, Extension, MD5 string
	Bytes                       int64
}

func (c *Client) FetchMedia(ctx context.Context, source Source, input MediaRequest, output io.Writer) (MediaInfo, error) {
	var ret MediaInfo
	if err := ctx.Err(); err != nil {
		return ret, err
	}
	if source.Validate() != nil || input.URL == "" || source.mediaURL(input.URL) != input.URL || input.Limit < 1 || input.Limit > 128<<20 || input.ExpectedBytes > input.Limit {
		return ret, fmt.Errorf("media is unavailable, unsafe or exceeds the download limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case c.media <- struct{}{}:
		defer func() { <-c.media }()
	case <-ctx.Done():
		return ret, ctx.Err()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
	if err != nil {
		return ret, fmt.Errorf("invalid media request")
	}
	headers := func(req *http.Request) {
		req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
		req.Header.Set("Referer", strings.TrimRight(source.URL, "/")+"/")
		req.Header.Del("Authorization")
		base, _ := url.Parse(source.URL)
		if strings.EqualFold(req.URL.Hostname(), base.Hostname()) && (source.Engine == "danbooru" || source.Engine == "e621") && source.APIKey != "" {
			req.SetBasicAuth(source.Login, source.APIKey)
		}
	}
	headers(req)
	client := *c.http
	client.Timeout = 2 * time.Minute
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 || source.mediaURL(next.URL.String()) != next.URL.String() {
			return fmt.Errorf("unsafe media redirect")
		}
		headers(next)
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return ret, fmt.Errorf("media could not be reached; check source CDN hosts and server connectivity")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ret, fmt.Errorf("media provider returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > input.Limit {
		return ret, fmt.Errorf("media exceeds the size limit")
	}
	prefix := make([]byte, 512)
	n, err := io.ReadFull(res.Body, prefix)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return ret, fmt.Errorf("cannot read media")
	}
	if int64(n) > input.Limit {
		return ret, fmt.Errorf("media exceeds size limit")
	}
	prefix = prefix[:n]
	ret.ContentType = http.DetectContentType(prefix)
	switch ret.ContentType {
	case "image/jpeg":
		ret.Extension = "jpg"
	case "image/png":
		ret.Extension = "png"
	case "image/gif":
		ret.Extension = "gif"
	case "image/webp":
		ret.Extension = "webp"
	case "image/bmp":
		ret.Extension = "bmp"
	case "image/tiff":
		ret.Extension = "tiff"
	case "video/mp4":
		ret.Extension = "mp4"
	case "video/webm":
		ret.Extension = "webm"
	default:
		switch {
		case len(prefix) >= 2 && prefix[0] == 0xff && prefix[1] == 0x0a:
			ret.ContentType, ret.Extension = "image/jxl", "jxl"
		case len(prefix) >= 12 && string(prefix[4:12]) == "JXL \r\n\x87\n":
			ret.ContentType, ret.Extension = "image/jxl", "jxl"
		case len(prefix) >= 12 && string(prefix[4:8]) == "ftyp" && (string(prefix[8:12]) == "avif" || string(prefix[8:12]) == "avis"):
			ret.ContentType, ret.Extension = "image/avif", "avif"
		default:
			return ret, fmt.Errorf("provider did not return a supported image or video; HTML and SVG are refused")
		}
	}
	if input.ImagesOnly && !strings.HasPrefix(ret.ContentType, "image/") {
		return ret, fmt.Errorf("preview is not an image")
	}
	h := md5.New() //nolint:gosec // Provider content identity.
	dest := io.MultiWriter(output, h)
	if _, err = dest.Write(prefix); err != nil {
		return ret, err
	}
	count, err := io.Copy(dest, io.LimitReader(res.Body, input.Limit-int64(n)+1))
	ret.Bytes = int64(n) + count
	if err != nil || ret.Bytes > input.Limit || (res.ContentLength >= 0 && res.ContentLength != ret.Bytes) || (input.ExpectedBytes > 0 && input.ExpectedBytes != ret.Bytes) {
		return ret, fmt.Errorf("media was truncated or exceeded the size limit")
	}
	ret.MD5 = fmt.Sprintf("%x", h.Sum(nil))
	if input.ExpectedMD5 != "" && ret.MD5 != input.ExpectedMD5 {
		return ret, fmt.Errorf("original checksum does not match the reviewed post; refresh the search")
	}
	return ret, nil
}
