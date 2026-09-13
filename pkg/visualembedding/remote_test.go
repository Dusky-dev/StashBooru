package visualembedding

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

const testModelID = "deepghs/wd14_tagger_with_embeddings:SmilingWolf/wd-eva02-large-tagger-v3"
const testModelRevision = "02fcdebd8afb52d5697a91efa4ca1c522b632581"

func TestRemoteClientStatusEmbedAndTag(t *testing.T) {
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
		case "/v1/status":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected status method %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_ = json.NewEncoder(w).Encode(response{
				OK:            true,
				Model:         testModelID,
				ModelRevision: testModelRevision,
				Dimensions:    Dimensions,
				Installed:     true,
				Loaded:        false,
				ModelPath:     "/models/model.onnx",
			})
		case "/v1/embed":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected embed method %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
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
			embedding := make([]float32, Dimensions)
			embedding[0] = 1
			_ = json.NewEncoder(w).Encode(response{
				OK:            true,
				Model:         testModelID,
				ModelRevision: testModelRevision,
				Dimensions:    Dimensions,
				Installed:     true,
				Loaded:        true,
				ModelPath:     "/models/model.onnx",
				Embedding:     embedding,
			})
		case "/v1/tag":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected tag method %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			if got := r.URL.Query().Get("threshold"); got != "0.42" {
				t.Errorf("unexpected tag threshold %q", got)
			}
			if got := r.URL.Query().Get("limit"); got != "17" {
				t.Errorf("unexpected tag limit %q", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading tag request body: %v", err)
				http.Error(w, "body error", http.StatusBadRequest)
				return
			}
			if string(body) != string(imageBytes) {
				t.Errorf("unexpected tag image body %q", string(body))
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(response{
				OK:            true,
				Model:         testModelID,
				ModelRevision: testModelRevision,
				Dimensions:    Dimensions,
				Installed:     true,
				Loaded:        true,
				ModelPath:     "/models/model.onnx",
				Tags: []TagPrediction{
					{Name: "1girl", Category: "general", Score: 0.93},
					{Name: "example_character", Category: "character", Score: 0.88},
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
	if !status.Installed || status.Loaded || status.ModelPath != "/models/model.onnx" {
		t.Fatalf("unexpected remote status: %+v", status)
	}

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "image.bin")
	if err := os.WriteFile(imagePath, imageBytes, 0600); err != nil {
		t.Fatal(err)
	}

	embedding, err := client.Embed(context.Background(), imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(embedding) != Dimensions || embedding[0] != 1 {
		t.Fatalf("unexpected embedding: len=%d first=%f", len(embedding), embedding[0])
	}

	tags, err := client.Tag(context.Background(), imagePath, 0.42, 17)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Name != "1girl" || tags[1].Category != "character" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
}

func TestNewRemoteRejectsInvalidURL(t *testing.T) {
	for _, raw := range []string{"", "gpu-pc:8000", "ftp://gpu-pc/model"} {
		if _, err := NewRemote(raw, ""); err == nil {
			t.Fatalf("expected URL %q to be rejected", raw)
		}
	}
}
