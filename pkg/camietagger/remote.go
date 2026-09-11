package camietagger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type RemoteClient struct {
	baseURL string
	token   string
	client  *http.Client
}

var _ Tagger = (*RemoteClient)(nil)

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
		return "", fmt.Errorf("remote Camie worker URL is empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing remote Camie worker URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("remote Camie worker URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("remote Camie worker URL has no host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("remote Camie worker URL must not contain a query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *RemoteClient) Close() error {
	return nil
}

func (c *RemoteClient) Status(ctx context.Context) (Status, error) {
	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := c.do(statusCtx, http.MethodGet, "/v1/camie/status", nil, -1)
	if err != nil {
		return Status{}, err
	}
	if err := validateResponse(res); err != nil {
		return Status{}, err
	}
	return statusFromResponse(res), nil
}

func (c *RemoteClient) Tag(ctx context.Context, path string, threshold float64, limit int) ([]Tag, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("Camie tagger image path is empty")
	}
	if err := validateOptions(threshold, limit); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening image for remote Camie tagging: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image for remote Camie tagging: %w", err)
	}

	query := url.Values{}
	query.Set("threshold", strconv.FormatFloat(threshold, 'f', -1, 64))
	query.Set("limit", strconv.Itoa(limit))
	res, err := c.do(ctx, http.MethodPost, "/v1/camie/tag?"+query.Encode(), file, stat.Size())
	if err != nil {
		return nil, err
	}
	if err := validateResponse(res); err != nil {
		return nil, err
	}
	if err := validateTags(res.Tags); err != nil {
		return nil, err
	}
	return res.Tags, nil
}

func (c *RemoteClient) do(ctx context.Context, method, path string, body io.Reader, contentLength int64) (response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return response{}, fmt.Errorf("creating remote Camie request: %w", err)
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
		return response{}, fmt.Errorf("calling remote Camie worker: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 16*1024))
		text := strings.TrimSpace(string(message))
		if text == "" {
			text = httpResponse.Status
		}
		return response{}, fmt.Errorf("remote Camie worker returned HTTP %d: %s", httpResponse.StatusCode, text)
	}

	var res response
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 16*1024*1024))
	if err := decoder.Decode(&res); err != nil {
		return response{}, fmt.Errorf("decoding remote Camie worker response: %w", err)
	}
	if !res.OK {
		if res.Error == "" {
			res.Error = "unknown worker error"
		}
		return response{}, fmt.Errorf("remote Camie worker: %s", res.Error)
	}
	return res, nil
}
