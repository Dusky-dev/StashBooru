// Package imagerestore runs bounded still-image restoration independently of conversion.
package imagerestore

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
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

const MaxBytes int64 = 96 * 1024 * 1024
const MaxPixels = 16 * 1024 * 1024

type Capabilities struct {
	Protocol  int    `json:"protocol"`
	Model     string `json:"model"`
	Revision  string `json:"revision"`
	Signature string `json:"signature"`
	Available bool   `json:"available"`
	CPU       bool   `json:"cpu"`
	GPU       bool   `json:"gpu"`
	MaxPixels int    `json:"maxPixels"`
	WorkSize  int    `json:"workSize"`
	Reference string `json:"reference"`
	Notice    string `json:"notice"`
}

type Options struct {
	Operation      string  `json:"operation"`
	Hardware       string  `json:"hardware"`
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negativePrompt"`
	Seed           int     `json:"seed"`
	Steps          int     `json:"steps"`
	Guidance       float64 `json:"guidance"`
	Strength       float64 `json:"strength"`
	Signature      string  `json:"signature"`
}

func (o Options) Validate() error {
	if o.Hardware != "auto" && o.Hardware != "gpu" && o.Hardware != "cpu" {
		return fmt.Errorf("choose auto, gpu or cpu hardware")
	}
	if len(o.Prompt) > 2000 || len(o.NegativePrompt) > 2000 || o.Seed < 0 || o.Seed > 2147483647 || o.Steps < 5 || o.Steps > 50 || !(o.Guidance >= 1 && o.Guidance <= 15) || !(o.Strength >= .05 && o.Strength <= 1) || int(float64(o.Steps)*o.Strength) < 1 {
		return fmt.Errorf("invalid prompt/seed/steps/guidance/strength; use steps 5–50, guidance 1–15 and strength .05–1")
	}
	return nil
}

type Receipt struct {
	Protocol      int     `json:"protocol"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Model         string  `json:"model,omitempty"`
	Revision      string  `json:"revision,omitempty"`
	Signature     string  `json:"signature,omitempty"`
	Hardware      string  `json:"hardware,omitempty"`
	Crop          []int   `json:"crop,omitempty"`
	WorkSize      []int   `json:"workSize,omitempty"`
	Seconds       float64 `json:"seconds,omitempty"`
	OutputSHA256  string  `json:"outputSHA256"`
	Normalization string  `json:"normalization"`
}

type Client struct{ URL, Token string }

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 128*1024 {
		return 0, fmt.Errorf("worker log exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (c Client) local(ctx context.Context, out interface{}, args ...string) error {
	worker := os.Getenv("STASH_IMAGE_RESTORATION_WORKER")
	if worker == "" {
		worker = filepath.FromSlash("scripts/image_restoration_worker.py")
	}
	python := os.Getenv("STASH_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, append([]string{worker}, args...)...)
	configureProcess(cmd)
	var stdout, stderr boundedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restoration worker: %w: %s", err, stderr.String())
	}
	return json.Unmarshal(stdout.Bytes(), out)
}

func (c Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, body)
	if err == nil && c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, err
}
func remoteError(res *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
	return fmt.Errorf("remote restoration HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
}

func (c Client) Capabilities(ctx context.Context) (Capabilities, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var ret Capabilities
	if c.URL == "" {
		if err := c.local(ctx, &ret, "capabilities"); err != nil {
			return ret, err
		}
	} else {
		req, err := c.request(ctx, http.MethodGet, "/v1/restoration/capabilities", nil)
		if err != nil {
			return ret, err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return ret, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return ret, remoteError(res)
		}
		if err := json.NewDecoder(io.LimitReader(res.Body, 128*1024)).Decode(&ret); err != nil {
			return ret, err
		}
	}
	if ret.Protocol != 1 || ret.MaxPixels <= 0 || ret.MaxPixels > MaxPixels || ret.WorkSize != 512 {
		return ret, fmt.Errorf("unsupported restoration worker; install the P11 adapter and restart")
	}
	return ret, nil
}

// Explicit remote never invokes local processing. Auto falls back only at negotiation.
func SelectWorker(ctx context.Context, backend string, local, remote Client) (Client, Capabilities, string, error) {
	if backend != "auto" && backend != "remote" && backend != "local" {
		return local, Capabilities{}, "", fmt.Errorf("choose auto, remote or local backend")
	}
	notice := ""
	if backend != "local" {
		if remote.URL == "" {
			if backend == "remote" {
				return remote, Capabilities{}, "", fmt.Errorf("configure the System remote worker URL/token first")
			}
		} else {
			caps, err := remote.Capabilities(ctx)
			if err == nil && (caps.Available || backend == "remote") {
				return remote, caps, "", nil
			}
			if backend == "remote" {
				return remote, caps, "", err
			}
			if err != nil {
				notice = "Remote restoration unavailable; local selected: " + err.Error()
			} else {
				notice = "Remote model unavailable; local selected: " + caps.Notice
			}
		}
	}
	caps, err := local.Capabilities(ctx)
	return local, caps, notice, err
}

func Bundle(path string, entries map[string]string, options Options) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	var total int64
	for name, path := range entries {
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		writer, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			input.Close()
			return err
		}
		n, err := io.Copy(writer, io.LimitReader(input, MaxBytes-total+1))
		input.Close()
		if err != nil {
			return err
		}
		total += n
		if total > MaxBytes-8192 {
			return fmt.Errorf("restoration input exceeds 96 MiB")
		}
	}
	writer, err := z.Create("options.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(writer).Encode(options); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (c Client) Process(ctx context.Context, input, output string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if c.URL == "" {
		var receipt Receipt
		err := c.local(ctx, &receipt, "process", "--input", input, "--output", output)
		if err != nil {
			_ = os.Remove(output)
		}
		return err
	}
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > MaxBytes {
		return fmt.Errorf("restoration input exceeds 96 MiB")
	}
	req, err := c.request(ctx, http.MethodPost, "/v1/restoration", f)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return remoteError(res)
	}
	if res.ContentLength <= 0 || res.ContentLength > MaxBytes {
		return fmt.Errorf("invalid restoration response length")
	}
	out, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			_ = os.Remove(output)
		}
	}()
	digest := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, digest), io.LimitReader(res.Body, res.ContentLength+1))
	if err != nil {
		return err
	}
	if n != res.ContentLength || fmt.Sprintf("%x", digest.Sum(nil)) != res.Header.Get("X-Stash-Content-SHA256") {
		return fmt.Errorf("restoration response incomplete or corrupted")
	}
	if err := out.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func Unbundle(path, directory, role string) (Receipt, error) {
	var ret Receipt
	z, err := zip.OpenReader(path)
	if err != nil {
		return ret, err
	}
	defer z.Close()
	if len(z.File) != 2 {
		return ret, fmt.Errorf("invalid restoration result bundle")
	}
	seen := map[string]bool{}
	for _, entry := range z.File {
		if seen[entry.Name] || (entry.Name != role+".png" && entry.Name != "receipt.json") || entry.UncompressedSize64 > uint64(MaxBytes) {
			return ret, fmt.Errorf("invalid restoration result entry")
		}
		seen[entry.Name] = true
		r, err := entry.Open()
		if err != nil {
			return ret, err
		}
		if entry.Name == "receipt.json" {
			err = json.NewDecoder(io.LimitReader(r, 8192)).Decode(&ret)
		} else {
			var out *os.File
			out, err = os.OpenFile(filepath.Join(directory, entry.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err == nil {
				_, err = io.Copy(out, io.LimitReader(r, MaxBytes+1))
				if err == nil {
					err = out.Sync()
				}
				out.Close()
			}
		}
		r.Close()
		if err != nil {
			return ret, err
		}
	}
	if ret.Protocol != 1 || ret.Width <= 0 || ret.Height <= 0 || int64(ret.Width)*int64(ret.Height) > MaxPixels {
		return ret, fmt.Errorf("invalid restoration output dimensions")
	}
	digest, err := SHA256(filepath.Join(directory, role+".png"))
	if err != nil {
		return ret, err
	}
	if digest != ret.OutputSHA256 {
		return ret, fmt.Errorf("restoration output checksum mismatch")
	}
	return ret, nil
}

func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", digest.Sum(nil)), nil
}
