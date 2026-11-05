package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestSharedLinkEndpointKeepsAccessKeysOutOfResponse(t *testing.T) {
	rs := booruBrowserRoutes{}
	raw := "https://mega.nz/file/abcDEF123#abcdef1234567890"
	body, _ := json.Marshal(map[string]string{"url": raw})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/booru-browser/shared-link", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rs.SharedLink(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("link inspect: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "abcdef1234567890") ||
		strings.Contains(w.Body.String(), "mega.nz/file/abcDEF123#") {
		t.Fatal("decryption key leaked into shared-link JSON")
	}
	var response struct {
		Source booru.PublicSource `json:"source"`
		Post   booru.Post         `json:"post"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Source.Instance != booru.SharedLinkSource().Instance() || response.Post.MediaKey == "" {
		t.Fatalf("invalid shared link response: %+v", response)
	}
	source, post, ok := rs.mediaSnapshot(response.Post.MediaKey)
	if !ok || source.ID != booru.SharedLinkSource().ID || post.OriginalURL != raw {
		t.Fatal("temporary server-side media key did not retain the original share link")
	}
}
