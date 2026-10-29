package api

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func shadowHTTPFixturePNG(t *testing.T) []byte {
	t.Helper()
	pixels := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(20 * x), G: uint8(20 * y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, pixels); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestShadowHTTPOriginalAndVerifiedBytesAgreeOnGETAndRange(t *testing.T) {
	data := shadowHTTPFixturePNG(t)
	original := filepath.Join(t.TempDir(), "sample.png")
	if err := os.WriteFile(original, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct {
		label  string
		method string
		header string
		status int
	}{
		{label: "whole-image", method: http.MethodGet, status: http.StatusOK},
		{label: "partial-bytes", method: http.MethodGet, header: "bytes=0-19", status: http.StatusPartialContent},
		{label: "head", method: http.MethodHead, status: http.StatusOK},
	} {
		t.Run(request.label, func(t *testing.T) {
			normalRequest := httptest.NewRequest(request.method, "/image/1/image", nil)
			shadowRequest := httptest.NewRequest(request.method, "/image/storage-shadow", nil)
			if request.header != "" {
				normalRequest.Header.Set("Range", request.header)
				shadowRequest.Header.Set("Range", request.header)
			}
			normal := httptest.NewRecorder()
			shadow := httptest.NewRecorder()
			http.ServeFile(normal, normalRequest, original)
			serveVerifiedStorageShadowBytes(shadow, shadowRequest, data)
			if normal.Code != request.status || shadow.Code != request.status {
				t.Fatalf("unexpected status original=%d shadow=%d", normal.Code, shadow.Code)
			}
			if !bytes.Equal(normal.Body.Bytes(), shadow.Body.Bytes()) {
				t.Fatal("shadow byte-range response differs from original file response")
			}
			if shadow.Header().Get("Content-Type") != "image/png" ||
				shadow.Header().Get("Cache-Control") != "no-store" ||
				shadow.Header().Get("X-Content-Type-Options") != "nosniff" ||
				shadow.Header().Get("Content-Security-Policy") == "" {
				t.Fatalf("missing safe shadow headers: %+v", shadow.Header())
			}
			if shadow.Header().Get("Content-Disposition") != "" {
				t.Fatal("valid PNG unexpectedly forced download")
			}
			if request.header != "" && shadow.Header().Get("Content-Range") != normal.Header().Get("Content-Range") {
				t.Fatal("incompatible shadow byte-range header")
			}
		})
	}
}

func TestShadowHTTPNonRasterPayloadsAreAttachments(t *testing.T) {
	cases := []string{
		"<html><script>window.alert('x')</script></html>",
		"<svg xmlns='http://www.w3.org/2000'><script/></svg>",
		"not-supported-binary-data",
	}
	for _, value := range cases {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/image/storage-shadow", nil)
		serveVerifiedStorageShadowBytes(recorder, request, []byte(value))
		if recorder.Code != http.StatusOK ||
			recorder.Header().Get("Content-Type") != "application/octet-stream" ||
			recorder.Header().Get("Content-Disposition") != "attachment" ||
			recorder.Header().Get("X-Content-Type-Options") != "nosniff" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			!bytes.Equal(recorder.Body.Bytes(), []byte(value)) {
			t.Fatalf("unsafe non-raster response: %+v", recorder)
		}
	}
}
