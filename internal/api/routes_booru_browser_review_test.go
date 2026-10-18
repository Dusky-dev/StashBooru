package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

func TestBooruReviewHTTPValidationAndRedaction(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	instance := booru.Defaults()[0].Instance()
	for _, test := range []struct {
		update bool
		body   string
		want   int
	}{
		{false, `{"source":"danbooru","instance":"old","posts":[{"id":"1"}]}`, 409},
		{false, `{"source":"unknown","posts":[]}`, 400},
		{false, `{"posts":[{"id":"0"}]}`, 400},
		{false, `{"posts":[{"id":"18446744073709551616"}]}`, 400},
		{false, `{"posts":[{"id":"1","md5":"bad"}]}`, 400},
		{false, `{"posts":[{"id":"1"},{"id":"1"}]}`, 400},
		{false, `{"apiKey":"private-secret"}`, 400},
		{true, `{"changes":[]}`, 400},
		{true, `{"changes":[{"id":"1","version":-1,"ignored":true}]}`, 400},
		{true, `{"changes":[{"id":"1"}]}`, 400},
		{true, `{"changes":[{"id":"1","ignored":true},{"id":"1","downloaded":true}]}`, 400},
		{false, fmt.Sprintf(`{"source":"danbooru","instance":%q,"posts":[]}`, instance), 503},
	} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		if test.update {
			rs.UpdateReview(w, request)
		} else {
			rs.ReadReview(w, request)
		}
		if w.Code != test.want || strings.Contains(w.Body.String(), "private-secret") {
			t.Fatalf("review validation: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("review state must not be cached")
		}
	}
	for _, headers := range []map[string]string{{"Content-Type": "text/plain"}, {"Content-Type": "application/json", "Origin": "https://evil.example.org"}, {"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}} {
		request := httptest.NewRequest("POST", "https://stash.example.org/", strings.NewReader(`{}`))
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		rs.UpdateReview(w, request)
		if w.Code != 403 && w.Code != 415 {
			t.Fatal("unsafe review write accepted")
		}
	}
}

func TestBooruLocalVerificationBoundsAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("known bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	base := &models.BaseFile{Path: path}
	budget := booruLocalPageLimit
	verified := verifyBooruFile(context.Background(), base, &budget)
	if verified == nil || verified.hash != "4958622f28750119e70868b23024d327" {
		t.Fatal("current bytes were not hashed accurately")
	}
	budget = 1
	if verifyBooruFile(context.Background(), base, &budget) != nil {
		t.Fatal("page budget ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget = booruLocalPageLimit
	if verifyBooruFile(ctx, base, &budget) != nil {
		t.Fatal("cancellation ignored")
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(booruLocalFileLimit + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if verifyBooruFile(context.Background(), base, &budget) != nil {
		t.Fatal("oversized file was read")
	}
	zip := models.FileID(1)
	base.ZipFileID = &zip
	if verifyBooruFile(context.Background(), base, &budget) != nil {
		t.Fatal("ZIP member was misrepresented as verified")
	}
}
