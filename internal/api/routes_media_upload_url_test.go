package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestMediaUploadURLRequiresPublicHTTP(t *testing.T) {
	for _, value := range []string{
		"file:///etc/passwd", "ftp://example.com/a.mp4",
		"http://user:pass@example.com/a.jpg",
		"http://example.com:8080/a.jpg", "https://example.com:8443/video.mp4",
		"http:///broken", "localhost/a.jpg",
	} {
		if _, err := parseMediaUploadURL(value); err == nil {
			t.Errorf("unsafe URL accepted: %s", value)
		}
	}
	for _, value := range []string{
		"https://example.com/media/image.jpg",
		"http://example.org/file.mp4?token=public",
		"https://example.org:443/a.webp",
	} {
		if _, err := parseMediaUploadURL(value); err != nil {
			t.Errorf("public URL rejected: %s: %v", value, err)
		}
	}
}

func TestMediaUploadPublicIPBlocksLocalAndReserved(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1", "10.3.4.5", "192.168.0.1", "169.254.169.254",
		"100.64.22.10", "198.18.0.1", "0.0.0.0", "::1",
		"fd12::1234", "fe80::1", "::ffff:127.0.0.1", "2001:db8::1",
	} {
		if mediaUploadPublicIP(netip.MustParseAddr(ip)) {
			t.Errorf("nonpublic IP accepted: %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !mediaUploadPublicIP(netip.MustParseAddr(ip)) {
			t.Errorf("public IP rejected: %s", ip)
		}
	}
}

func TestMediaUploadNameFromURL(t *testing.T) {
	u, _ := parseMediaUploadURL("https://example.com/media/photo.jpg?download=1")
	name, err := mediaUploadNameFromURL(u, http.Header{})
	if err != nil || name != "photo.jpg" {
		t.Fatalf("URL media filename not honored: %q %v", name, err)
	}
	header := http.Header{}
	header.Set("Content-Disposition", `attachment; filename="video.mkv"`)
	name, err = mediaUploadNameFromURL(u, header)
	if err != nil || name != "video.mkv" {
		t.Fatalf("Content-Disposition filename not honored: %q %v", name, err)
	}
	u, _ = parseMediaUploadURL("https://example.com/download?id=2")
	header = http.Header{}
	header.Set("Content-Type", "image/avif")
	name, err = mediaUploadNameFromURL(u, header)
	if err != nil || name != "download.avif" {
		t.Fatalf("MIME extension not inferred: %q %v", name, err)
	}
	header.Set("Content-Type", "text/html")
	if _, err = mediaUploadNameFromURL(u, header); err == nil {
		t.Fatal("HTML was accepted as a media filename")
	}
}

type mediaUploadBufferResponse struct {
	http.ResponseWriter
	bytes.Buffer
}

func (w *mediaUploadBufferResponse) Header() http.Header { return http.Header{} }
func (w *mediaUploadBufferResponse) WriteHeader(int) {}
func (w *mediaUploadBufferResponse) Write(b []byte) (int, error) { return w.Buffer.Write(b) }

func TestMediaUploadRemoteProgressUsesActualBytes(t *testing.T) {
	src := strings.NewReader(strings.Repeat("x", 1500000))
	var dest bytes.Buffer
	var response mediaUploadBufferResponse
	n, err := copyMediaUploadRemote(&response, &dest, src, int64(src.Len()))
	if err != nil || n != 1500000 || int64(dest.Len()) != n {
		t.Fatalf("remote download bytes mismatch: %d, %v", n, err)
	}
	if !strings.Contains(response.String(), `"loaded":1500000`) ||
		!strings.Contains(response.String(), `"total":1500000`) {
		t.Fatalf("progress did not report verified byte counts: %s", response.String())
	}
	_, err = copyMediaUploadRemote(&response, io.Discard, errorMediaReader{}, 0)
	if err == nil {
		t.Fatal("network read error was ignored")
	}
}

type errorMediaReader struct{}
func (errorMediaReader) Read([]byte) (int, error) { return 0, errors.New("disconnected") }
