// Package mediaconvert runs the shared media encoder locally or on the tagging worker.
package mediaconvert

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Content identity, not authentication.
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Upscaler       string  `json:"upscaler,omitempty"`
	UpscaleScale   int     `json:"upscaleScale,omitempty"`
	Format         string  `json:"format"`
	Hardware       string  `json:"hardware"`
	Quality        float64 `json:"quality"`
	Effort         int     `json:"effort"`
	Distance       float64 `json:"distance"`
	FasterDecoding *int    `json:"fasterDecoding,omitempty"`
	Lossless       bool    `json:"lossless"`
	AllowLarger    bool    `json:"allowLarger"`
	DropAudio      bool    `json:"dropAudio"`
	AllowAlphaLoss bool    `json:"allowAlphaLoss"`
}

type Format struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Extension string   `json:"extension"`
	Family    string   `json:"family"`
	CPU       []string `json:"cpu"`
	GPU       []string `json:"gpu"`
	Controls  []string `json:"controls"`
	Available bool     `json:"available"`
}

type Capabilities struct {
	Formats   []Format   `json:"formats"`
	Upscalers []Upscaler `json:"upscalers"`
}

type Upscaler struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	CPU       bool   `json:"cpu"`
	Notice    string `json:"notice"`
}

type Result struct {
	Upscaler   string  `json:"upscaler,omitempty"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Frames     int64   `json:"frames"`
	Duration   float64 `json:"duration"`
	FrameRate  float64 `json:"frameRate"`
	VideoCodec string  `json:"videoCodec"`
	AudioCodec string  `json:"audioCodec"`
	BitRate    int64   `json:"bitRate"`
	Format     string  `json:"format"`
	Encoder    string  `json:"encoder"`
	Seconds    float64 `json:"seconds"`
	Size       int64   `json:"size"`
}

type Client struct {
	URL, Token      string
	FFMpeg, FFProbe string
}

func (c Client) local(ctx context.Context, out interface{}, args ...string) error {
	worker := os.Getenv("STASH_MEDIA_CONVERSION_WORKER")
	if worker == "" {
		worker = filepath.FromSlash("scripts/media_conversion_worker.py")
	}
	python := os.Getenv("STASH_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, append([]string{worker}, args...)...)
	configureProcess(cmd)
	cmd.Env = os.Environ()
	if c.FFMpeg != "" {
		cmd.Env = append(cmd.Env, "STASH_CONVERTER_FFMPEG="+c.FFMpeg)
	}
	if c.FFProbe != "" {
		cmd.Env = append(cmd.Env, "STASH_CONVERTER_FFPROBE="+c.FFProbe)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("converter: %w: %s", err, stderr.String())
	}
	return json.Unmarshal(stdout.Bytes(), out)
}

func (c Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

func responseError(res *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
	return fmt.Errorf("remote converter HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
}

func (c Client) Capabilities(ctx context.Context) (Capabilities, error) {
	var ret Capabilities
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if c.URL == "" {
		return ret, c.local(ctx, &ret, "capabilities")
	}
	req, err := c.request(ctx, http.MethodGet, "/v1/convert/capabilities", nil)
	if err != nil {
		return ret, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ret, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ret, responseError(res)
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 128*1024)).Decode(&ret)
	return ret, err
}

func (c Client) Convert(ctx context.Context, source, output string, options Options) (Result, error) {
	var ret Result
	data, err := json.Marshal(options)
	if err != nil {
		return ret, err
	}
	if c.URL == "" {
		return ret, c.local(ctx, &ret, "convert", "--input", source, "--output", output, "--options", string(data))
	}
	f, err := os.Open(source)
	if err != nil {
		return ret, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return ret, err
	}
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()
	req, err := c.request(ctx, http.MethodPost, "/v1/convert", f)
	if err != nil {
		return ret, err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Stash-Conversion-Options", string(data))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ret, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ret, responseError(res)
	}
	metadata, err := base64.StdEncoding.DecodeString(res.Header.Get("X-Stash-Conversion"))
	if err != nil {
		return ret, err
	}
	if err = json.Unmarshal(metadata, &ret); err != nil {
		return ret, err
	}
	if ret.Size <= 0 || res.ContentLength != ret.Size || ret.Width <= 0 || ret.Height <= 0 || ret.Frames <= 0 {
		return ret, fmt.Errorf("invalid remote conversion metadata")
	}
	out, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ret, err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(output)
		}
	}()
	hash := md5.New() //nolint:gosec // Transport content identity.
	n, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(res.Body, ret.Size+1))
	if err != nil {
		return ret, err
	}
	if n != ret.Size || fmt.Sprintf("%x", hash.Sum(nil)) != res.Header.Get("X-Stash-Content-MD5") {
		return ret, fmt.Errorf("remote conversion transfer was incomplete or corrupted")
	}
	if err := out.Sync(); err != nil {
		return ret, err
	}
	ok = true
	return ret, nil
}
