package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShadowDownloadPreservesVerifiedByteRangesAndHEAD(t *testing.T) {
	original := shadowHTTPFixturePNG(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, rangeHeader := range []string{"", "bytes=0-31", "bytes=8-"} {
			t.Run(method+"_"+rangeHeader, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(method, "/image/storage-shadow-download", nil)
				if rangeHeader != "" {
					request.Header.Set("Range", rangeHeader)
				}
				serveVerifiedStorageShadowDownload(recorder, request, original, 44, "/media/private/Original.PNG")
				wantStatus := http.StatusOK
				if rangeHeader != "" {
					wantStatus = http.StatusPartialContent
				}
				if recorder.Code != wantStatus {
					t.Fatalf("unexpected download status %d, want %d", recorder.Code, wantStatus)
				}
				if recorder.Header().Get("Content-Type") != "application/octet-stream" ||
					recorder.Header().Get("Content-Disposition") != `attachment; filename="image-44.png"` ||
					recorder.Header().Get("Cache-Control") != "no-store" ||
					recorder.Header().Get("X-Content-Type-Options") != "nosniff" ||
					recorder.Header().Get("Content-Security-Policy") == "" {
					t.Fatalf("unsafe download headers: %+v", recorder.Header())
				}
				if method == http.MethodHead {
					if recorder.Body.Len() != 0 {
						t.Fatal("HEAD must never send source bytes")
					}
					continue
				}
				want := original
				if rangeHeader == "bytes=0-31" {
					want = original[:32]
				} else if rangeHeader == "bytes=8-" {
					want = original[8:]
				}
				if !bytes.Equal(recorder.Body.Bytes(), want) {
					t.Fatal("download differs from verified original image bytes")
				}
			})
		}
	}
}

func TestShadowDownloadNamesNeverIncludeSourceDirectoriesOrUnsafeExtensions(t *testing.T) {
	cases := map[string]string{
		"/some/untrusted secret/Name.JXL": ".jxl",
		"/some/private/picture.avif":      ".avif",
		"/some/private/script.html":       ".bin",
		"/some/private/file.unknown":      ".bin",
		"/some/private/contains.svg":      ".bin",
		"/some/private/photo.JpEg":        ".jpeg",
		"/some/private/image":             ".bin",
	}
	for name, want := range cases {
		if got := storageShadowDownloadExtension(name); got != want {
			t.Fatalf("source %q: extension %q != %q", name, got, want)
		}
		recorder := httptest.NewRecorder()
		serveVerifiedStorageShadowDownload(recorder,
			httptest.NewRequest(http.MethodGet, "/image/storage-shadow-download", nil),
			[]byte("original bytes"), 5, name)
		if got, wantName := recorder.Header().Get("Content-Disposition"), fmt.Sprintf(`attachment; filename="image-5%s"`, want); got != wantName {
			t.Fatalf("unsafe source filename leaked or extension changed: %q != %q", got, wantName)
		}
	}
}

func TestShadowDownloadRejectsAllUntrustedParametersBeforeFilesystemAccess(t *testing.T) {
	tests := []string{
		"",
		"stackID=-1&imageID=1&candidateID=stack-1-delta-0123456789abcdef01234567",
		"stackID=1&imageID=1&candidateID=../../private",
		"stackID=1&imageID=1&candidateID=stack-1-delta-0123456789abcdef01234567&filename=../../bad",
		"stackID=1&imageID=1&candidateID=stack-1-delta-0123456789abcdef01234567&output=/mnt/data",
	}
	for _, query := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/image/storage-shadow-download?"+query, nil)
		(imageRoutes{}).StorageShadowDownload(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("accepted untrusted path or identifier in %q: status %d", query, recorder.Code)
		}
	}
}
