package camietagger

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteClientStatusAndTag(t *testing.T) {
	const token = "secret-token"
	imageBytes := []byte("fake-image-bytes")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("unexpected authorization header %q", got)
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/camie/status":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected status method %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_ = json.NewEncoder(w).Encode(response{
				OK:             true,
				Model:          Model,
				ModelPath:      "/models/camie-tagger-v2.onnx",
				MetadataPath:   "/models/camie-tagger-v2-metadata.json",
				ModelExists:    true,
				MetadataExists: true,
				Installed:      true,
				TagCount:       70527,
			})
		case "/v1/camie/tag":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected tag method %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			if got := r.URL.Query().Get("threshold"); got != "0.492" {
				t.Errorf("unexpected threshold %q", got)
			}
			if got := r.URL.Query().Get("limit"); got != "25" {
				t.Errorf("unexpected limit %q", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading request body: %v", err)
				http.Error(w, "body error", http.StatusBadRequest)
				return
			}
			if string(body) != string(imageBytes) {
				t.Errorf("unexpected image body %q", string(body))
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(response{
				OK:             true,
				Model:          Model,
				ModelPath:      "/models/camie-tagger-v2.onnx",
				MetadataPath:   "/models/camie-tagger-v2-metadata.json",
				ModelExists:    true,
				MetadataExists: true,
				Installed:      true,
				TagCount:       70527,
				Threshold:      DefaultThreshold,
				Tags: []Tag{
					{Name: "rem_(re:zero)", Category: "character", Score: 0.97},
					{Name: "re:zero_kara_hajimeru_isekai_seikatsu", Category: "copyright", Score: 0.95},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewRemote(server.URL+"/", token)
	if err != nil {
		t.Fatal(err)
	}

	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.TagCount != 70527 || status.ModelPath != "/models/camie-tagger-v2.onnx" {
		t.Fatalf("unexpected remote Camie status: %+v", status)
	}

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "image.bin")
	if err := os.WriteFile(imagePath, imageBytes, 0600); err != nil {
		t.Fatal(err)
	}

	tags, err := client.Tag(context.Background(), imagePath, DefaultThreshold, 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Name != "rem_(re:zero)" || tags[0].Category != "character" {
		t.Fatalf("unexpected Camie tags: %+v", tags)
	}
}

func TestNewRemoteRejectsInvalidURL(t *testing.T) {
	for _, raw := range []string{"", "gpu-pc:8000", "ftp://gpu-pc/model"} {
		if _, err := NewRemote(raw, ""); err == nil {
			t.Fatalf("expected URL %q to be rejected", raw)
		}
	}
}
