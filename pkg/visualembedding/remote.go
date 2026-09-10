package visualembedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Embedder is the inference surface used by visual similarity indexing. Both
// the local stdin/stdout worker and the remote HTTP worker implement it.
type Embedder interface {
	Status(ctx context.Context) (ModelStatus, error)
	Embed(ctx context.Context, path string) ([]float32, error)
	Close() error
}

var _ Embedder = (*Client)(nil)

type RemoteClient struct {
	baseURL string
	token   string
	client  *http.Client
}

var _ Embedder = (*RemoteClient)(nil)

func NewRemote(baseURL, token string) (*RemoteClient, error) {
	normalized, err := normalizeRemoteURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &RemoteClient{
		baseURL: normalized,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}, nil
}

func normalizeRemoteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("remote visual embedding worker URL is empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing remote visual embedding worker URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("remote visual embedding worker URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("remote visual embedding worker URL has no host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("remote visual embedding worker URL must not contain a query or fragment")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *RemoteClient) Close() error {
	return nil
}

func (c *RemoteClient) Status(ctx context.Context) (ModelStatus, error) {
	res, err := c.do(ctx, http.MethodGet, "/v1/status", nil, -1)
	if err != nil {
		return ModelStatus{}, err
	}
	if err := validateWorker(res); err != nil {
		return ModelStatus{}, err
	}
	return statusFromResponse(res), nil
}

func (c *RemoteClient) Embed(ctx context.Context, path string) ([]float32, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("visual embedding path is empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening image for remote visual embedding: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image for remote visual embedding: %w", err)
	}

	res, err := c.do(ctx, http.MethodPost, "/v1/embed", file, stat.Size())
	if err != nil {
		return nil, err
	}
	if err := validateWorker(res); err != nil {
		return nil, err
	}
	if len(res.Embedding) != Dimensions {
		return nil, fmt.Errorf("remote visual embedding worker returned %d dimensions, expected %d", len(res.Embedding), Dimensions)
	}
	return res.Embedding, nil
}

func (c *RemoteClient) do(ctx context.Context, method, path string, body io.Reader, contentLength int64) (response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return response{}, fmt.Errorf("creating remote visual embedding request: %w", err)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResponse, err := c.client.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("calling remote visual embedding worker: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 16*1024))
		text := strings.TrimSpace(string(message))
		if text == "" {
			text = httpResponse.Status
		}
		return response{}, fmt.Errorf("remote visual embedding worker returned HTTP %d: %s", httpResponse.StatusCode, text)
	}

	var res response
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 16*1024*1024))
	if err := decoder.Decode(&res); err != nil {
		return response{}, fmt.Errorf("decoding remote visual embedding worker response: %w", err)
	}
	if !res.OK {
		if res.Error == "" {
			res.Error = "unknown worker error"
		}
		return response{}, fmt.Errorf("remote visual embedding worker: %s", res.Error)
	}
	return res, nil
}
