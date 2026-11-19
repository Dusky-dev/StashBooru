package booru

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

var icoBytes = append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{1}, 64)...)

func TestFetchIconAcceptsSniffedImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/favicon.ico" {
			http.NotFound(w, r)
			return
		}
		// A wrong declared type must not matter; the bytes decide.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(icoBytes)
	}))
	defer server.Close()
	data, contentType, err := fetchIcon(context.Background(), server.Client(), server.URL+"/some/path")
	if err != nil || contentType != "image/x-icon" || !bytes.Equal(data, icoBytes) {
		t.Fatalf("got %q %v", contentType, err)
	}
}

func TestFetchIconRefusesUnsafeOrUnusable(t *testing.T) {
	offsite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(icoBytes)
	}))
	defer offsite.Close()
	cases := map[string]http.HandlerFunc{
		"missing": func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"oversize": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(append(icoBytes, make([]byte, MaxIconBytes)...))
		},
		"html": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<html><body>not an icon</body></html>"))
		},
		"svg": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
		},
		"offsite redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.org:1/favicon.ico", http.StatusFound)
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			_, _, err := fetchIcon(context.Background(), server.Client(), server.URL)
			if !errors.Is(err, ErrNoIcon) {
				t.Fatalf("expected ErrNoIcon, got %v", err)
			}
		})
	}
	if _, _, err := fetchIcon(context.Background(), offsite.Client(), "ftp://example.org"); err == nil {
		t.Fatal("non-HTTP source URL accepted")
	}
}
