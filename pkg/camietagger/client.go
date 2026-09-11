package camietagger

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	Model            = "Camais03/camie-tagger-v2"
	DefaultThreshold = 0.492
	DefaultLimit     = 50
	MaxLimit         = 200
)

type Status struct {
	Installed      bool   `json:"installed"`
	Loaded         bool   `json:"loaded"`
	ModelExists    bool   `json:"modelExists"`
	MetadataExists bool   `json:"metadataExists"`
	ModelPath      string `json:"modelPath"`
	MetadataPath   string `json:"metadataPath"`
	Model          string `json:"model"`
	TagCount       int    `json:"tagCount"`
	Error          string `json:"error,omitempty"`
}

type Tag struct {
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Score        float64 `json:"score"`
	RawName      string  `json:"rawName,omitempty"`
	Source       string  `json:"source,omitempty"`
	TargetPath   string  `json:"targetPath,omitempty"`
	TargetExists bool    `json:"targetExists,omitempty"`
}

type Tagger interface {
	Status(ctx context.Context) (Status, error)
	Tag(ctx context.Context, path string, threshold float64, limit int) ([]Tag, error)
	Close() error
}

type Client struct {
	mu         sync.Mutex
	workerPath string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	nextID     uint64
}

var _ Tagger = (*Client)(nil)

type request struct {
	ID        uint64  `json:"id"`
	Op        string  `json:"op"`
	Path      string  `json:"path,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Limit     int     `json:"limit,omitempty"`
}

type response struct {
	ID             uint64  `json:"id"`
	OK             bool    `json:"ok"`
	Error          string  `json:"error,omitempty"`
	Model          string  `json:"model,omitempty"`
	ModelPath      string  `json:"model_path,omitempty"`
	MetadataPath   string  `json:"metadata_path,omitempty"`
	ModelExists    bool    `json:"model_exists,omitempty"`
	MetadataExists bool    `json:"metadata_exists,omitempty"`
	Installed      bool    `json:"installed,omitempty"`
	Loaded         bool    `json:"loaded,omitempty"`
	TagCount       int     `json:"tag_count,omitempty"`
	Threshold      float64 `json:"threshold,omitempty"`
	Tags           []Tag   `json:"tags,omitempty"`
}

func DefaultWorkerPath() string {
	if configured := strings.TrimSpace(os.Getenv("STASH_CAMIE_TAGGER_WORKER")); configured != "" {
		return configured
	}
	return filepath.FromSlash("scripts/camie_tagger_worker.py")
}

func New(workerPath string) *Client {
	if strings.TrimSpace(workerPath) == "" {
		workerPath = DefaultWorkerPath()
	}
	return &Client{workerPath: workerPath}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	res, err := c.call(ctx, request{Op: "status"})
	if err != nil {
		return Status{}, err
	}
	if err := validateResponse(res); err != nil {
		return Status{}, err
	}
	return statusFromResponse(res), nil
}

func (c *Client) Tag(ctx context.Context, path string, threshold float64, limit int) ([]Tag, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("camie tagger image path is empty")
	}
	if err := validateOptions(threshold, limit); err != nil {
		return nil, err
	}

	res, err := c.call(ctx, request{Op: "tag", Path: path, Threshold: threshold, Limit: limit})
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

func statusFromResponse(res response) Status {
	return Status{
		Installed:      res.Installed,
		Loaded:         res.Loaded,
		ModelExists:    res.ModelExists,
		MetadataExists: res.MetadataExists,
		ModelPath:      res.ModelPath,
		MetadataPath:   res.MetadataPath,
		Model:          res.Model,
		TagCount:       res.TagCount,
		Error:          res.Error,
	}
}

func validateOptions(threshold float64, limit int) error {
	if threshold <= 0 || threshold >= 1 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return fmt.Errorf("camie threshold must be greater than 0 and less than 1")
	}
	if limit < 1 || limit > MaxLimit {
		return fmt.Errorf("camie category limit must be between 1 and %d", MaxLimit)
	}
	return nil
}

func validateResponse(res response) error {
	if res.Model != Model {
		return fmt.Errorf("unexpected Camie model %q", res.Model)
	}
	if res.Installed && res.TagCount <= 0 {
		return fmt.Errorf("camie worker reports an installed model but no metadata tags")
	}
	return nil
}

func validateTags(tags []Tag) error {
	for _, tag := range tags {
		if strings.TrimSpace(tag.Name) == "" {
			return fmt.Errorf("camie worker returned an empty tag name")
		}
		if tag.Score < 0 || tag.Score > 1 || math.IsNaN(tag.Score) || math.IsInf(tag.Score, 0) {
			return fmt.Errorf("camie worker returned invalid score %v for tag %q", tag.Score, tag.Name)
		}
	}
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil {
		return nil
	}

	c.nextID++
	if err := json.NewEncoder(c.stdin).Encode(request{ID: c.nextID, Op: "shutdown"}); err != nil {
		_ = c.stopLocked(true)
		return fmt.Errorf("sending shutdown request to Camie tagger worker: %w", err)
	}
	return c.stopLocked(false)
}

func (c *Client) call(ctx context.Context, req request) (response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return response{}, err
	}
	if err := c.startLocked(); err != nil {
		return response{}, err
	}

	c.nextID++
	req.ID = c.nextID
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		_ = c.stopLocked(true)
		return response{}, fmt.Errorf("sending request to Camie tagger worker: %w", err)
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
			return response{}, fmt.Errorf("reading Camie tagger worker response: %w", read.err)
		}

		var res response
		if err := json.Unmarshal(read.line, &res); err != nil {
			_ = c.stopLocked(true)
			return response{}, fmt.Errorf("decoding Camie tagger worker response: %w", err)
		}
		if res.ID != req.ID {
			_ = c.stopLocked(true)
			return response{}, fmt.Errorf("camie tagger worker response id %d does not match request id %d", res.ID, req.ID)
		}
		if !res.OK {
			if res.Error == "" {
				res.Error = "unknown worker error"
			}
			return response{}, fmt.Errorf("camie tagger worker: %s", res.Error)
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
		return fmt.Errorf("opening Camie tagger worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("opening Camie tagger worker stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("starting Camie tagger worker %q: %w", workerPath, err)
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
