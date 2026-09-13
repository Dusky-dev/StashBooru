package visualembedding

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	Dimensions          = 1024
	TagCount            = 10861
	DefaultTagThreshold = 0.35
	DefaultTagLimit     = 50
	MaxTagLimit         = 200
	Model               = "deepghs/wd14_tagger_with_embeddings@02fcdebd8afb52d5697a91efa4ca1c522b632581:SmilingWolf/wd-eva02-large-tagger-v3"
)

type ModelStatus struct {
	Installed  bool   `json:"installed"`
	Loaded     bool   `json:"loaded"`
	ModelPath  string `json:"modelPath"`
	Model      string `json:"model"`
	Revision   string `json:"revision"`
	Dimensions int    `json:"dimensions"`
}

type TagPrediction struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
}

type Client struct {
	mu         sync.Mutex
	workerPath string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	nextID     uint64
}

type request struct {
	ID        uint64  `json:"id"`
	Op        string  `json:"op"`
	Path      string  `json:"path,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Limit     int     `json:"limit,omitempty"`
}

type response struct {
	ID            uint64          `json:"id"`
	OK            bool            `json:"ok"`
	Error         string          `json:"error,omitempty"`
	Model         string          `json:"model,omitempty"`
	ModelRevision string          `json:"model_revision,omitempty"`
	Dimensions    int             `json:"dimensions,omitempty"`
	Embedding     []float32       `json:"embedding,omitempty"`
	Tags          []TagPrediction `json:"tags,omitempty"`
	Threshold     float64         `json:"threshold,omitempty"`
	Limit         int             `json:"limit,omitempty"`
	Installed     bool            `json:"installed,omitempty"`
	Loaded        bool            `json:"loaded,omitempty"`
	ModelPath     string          `json:"model_path,omitempty"`
}

func DefaultWorkerPath() string {
	if configured := strings.TrimSpace(os.Getenv("STASH_VISUAL_EMBEDDING_WORKER")); configured != "" {
		return configured
	}
	return filepath.FromSlash("scripts/visual_embedding_worker.py")
}

func New(workerPath string) *Client {
	if strings.TrimSpace(workerPath) == "" {
		workerPath = DefaultWorkerPath()
	}
	return &Client{workerPath: workerPath}
}

func (c *Client) Ping(ctx context.Context) error {
	res, err := c.call(ctx, "ping", "")
	if err != nil {
		return err
	}
	return validateWorker(res)
}

func (c *Client) Status(ctx context.Context) (ModelStatus, error) {
	res, err := c.call(ctx, "status", "")
	if err != nil {
		return ModelStatus{}, err
	}
	if err := validateWorker(res); err != nil {
		return ModelStatus{}, err
	}
	return statusFromResponse(res), nil
}

// Download explicitly downloads the pinned default embedding model. Nothing in
// the worker or client calls this implicitly; callers must opt in deliberately.
func (c *Client) Download(ctx context.Context) (ModelStatus, error) {
	res, err := c.call(ctx, "download", "")
	if err != nil {
		return ModelStatus{}, err
	}
	if err := validateWorker(res); err != nil {
		return ModelStatus{}, err
	}
	return statusFromResponse(res), nil
}

func statusFromResponse(res response) ModelStatus {
	return ModelStatus{
		Installed:  res.Installed,
		Loaded:     res.Loaded,
		ModelPath:  res.ModelPath,
		Model:      res.Model,
		Revision:   res.ModelRevision,
		Dimensions: res.Dimensions,
	}
}

func (c *Client) Embed(ctx context.Context, path string) ([]float32, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("visual embedding path is empty")
	}

	res, err := c.call(ctx, "embed", path)
	if err != nil {
		return nil, err
	}
	if err := validateWorker(res); err != nil {
		return nil, err
	}
	if len(res.Embedding) != Dimensions {
		return nil, fmt.Errorf("visual embedding worker returned %d dimensions, expected %d", len(res.Embedding), Dimensions)
	}
	return res.Embedding, nil
}

func (c *Client) Tag(ctx context.Context, path string, threshold float64, limit int) ([]TagPrediction, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("EVA02 tagger path is empty")
	}
	if err := validateTagOptions(threshold, limit); err != nil {
		return nil, err
	}

	res, err := c.callRequest(ctx, request{
		Op:        "tag",
		Path:      path,
		Threshold: threshold,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWorker(res); err != nil {
		return nil, err
	}
	return res.Tags, nil
}

func validateTagOptions(threshold float64, limit int) error {
	if threshold <= 0 || threshold >= 1 {
		return fmt.Errorf("EVA02 threshold must be greater than 0 and less than 1")
	}
	if limit < 1 || limit > MaxTagLimit {
		return fmt.Errorf("EVA02 per-category limit must be between 1 and %d", MaxTagLimit)
	}
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil {
		return nil
	}

	// Best effort graceful shutdown. If the protocol is already broken, killing
	// the worker below is still safe because it is a disposable helper process.
	c.nextID++
	if err := json.NewEncoder(c.stdin).Encode(request{ID: c.nextID, Op: "shutdown"}); err != nil {
		_ = c.stopLocked(true)
		return fmt.Errorf("sending shutdown request to visual embedding worker: %w", err)
	}
	return c.stopLocked(false)
}

func validateWorker(res response) error {
	if res.Dimensions != Dimensions {
		return fmt.Errorf("visual embedding worker reports %d dimensions, expected %d", res.Dimensions, Dimensions)
	}

	expectedPrefix := "deepghs/wd14_tagger_with_embeddings:SmilingWolf/wd-eva02-large-tagger-v3"
	if res.Model != expectedPrefix {
		return fmt.Errorf("unexpected visual embedding model %q", res.Model)
	}
	if res.ModelRevision != "02fcdebd8afb52d5697a91efa4ca1c522b632581" {
		return fmt.Errorf("unexpected visual embedding model revision %q", res.ModelRevision)
	}
	return nil
}

func (c *Client) call(ctx context.Context, op, path string) (response, error) {
	return c.callRequest(ctx, request{Op: op, Path: path})
}

func (c *Client) callRequest(ctx context.Context, req request) (response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return response{}, err
	}
	if err := c.startLocked(); err != nil {
		return response{}, err
	}

	c.nextID++
	id := c.nextID
	req.ID = id
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		_ = c.stopLocked(true)
		return response{}, fmt.Errorf("sending request to visual embedding worker: %w", err)
	}

	type readResult struct {
		line []byte
		err  error
	}
	result := make(chan readResult, 1)
	go func() {
		line, err := c.stdout.ReadBytes('\n')
		result <- readResult{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = c.stopLocked(true)
		return response{}, ctx.Err()
	case read := <-result:
		if read.err != nil {
			_ = c.stopLocked(true)
			return response{}, fmt.Errorf("reading visual embedding worker response: %w", read.err)
		}

		var res response
		if err := json.Unmarshal(read.line, &res); err != nil {
			_ = c.stopLocked(true)
			return response{}, fmt.Errorf("decoding visual embedding worker response: %w", err)
		}
		if res.ID != id {
			_ = c.stopLocked(true)
			return response{}, fmt.Errorf("visual embedding worker response id %d does not match request id %d", res.ID, id)
		}
		if !res.OK {
			if res.Error == "" {
				res.Error = "unknown worker error"
			}
			return response{}, fmt.Errorf("visual embedding worker: %s", res.Error)
		}
		return res, nil
	}
}

func (c *Client) startLocked() error {
	if c.cmd != nil {
		return nil
	}

	workerPath := c.workerPath
	var cmd *exec.Cmd
	if strings.EqualFold(filepath.Ext(workerPath), ".py") {
		cmd = exec.Command("python3", workerPath)
	} else {
		cmd = exec.Command(workerPath)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("opening visual embedding worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("opening visual embedding worker stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("starting visual embedding worker %q: %w", workerPath, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	return nil
}

func (c *Client) stopLocked(kill bool) error {
	if c.cmd == nil {
		return nil
	}

	cmd := c.cmd
	stdin := c.stdin
	c.cmd = nil
	c.stdin = nil
	c.stdout = nil

	if stdin != nil {
		_ = stdin.Close()
	}
	if kill && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	err := cmd.Wait()
	if err != nil && !kill {
		return err
	}
	return nil
}
