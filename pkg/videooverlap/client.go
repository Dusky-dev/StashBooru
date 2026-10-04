package videooverlap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Capabilities struct {
	Algorithm string `json:"algorithm"`
	Signature string `json:"signature"`
	Decoder   string `json:"decoder"`
	Processor string `json:"processor"`
	MaxFrames int    `json:"maxFrames"`
}

type Client struct{ URL, Token, FFMpeg, FFProbe string }

func (c Client) local(ctx context.Context, out any, args ...string) error {
	worker := os.Getenv("STASH_VIDEO_OVERLAP_WORKER")
	if worker == "" {
		worker = filepath.FromSlash("scripts/video_overlap_worker.py")
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
		return fmt.Errorf("video sampler: %w: %s", err, stderr.String())
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

func remoteJSON(res *http.Response, out any) error {
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("video worker HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 16*1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 16*1024*1024 {
		return fmt.Errorf("video worker response exceeds 16 MiB")
	}
	return json.Unmarshal(data, out)
}

func (c Client) Capabilities(ctx context.Context) (Capabilities, error) {
	var ret Capabilities
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var err error
	if c.URL == "" {
		err = c.local(ctx, &ret, "capabilities")
	} else {
		var req *http.Request
		req, err = c.request(ctx, http.MethodGet, "/v1/video-overlap/capabilities", nil)
		if err == nil {
			var res *http.Response
			res, err = http.DefaultClient.Do(req)
			if err == nil {
				err = remoteJSON(res, &ret)
			}
		}
	}
	if err == nil && (ret.Algorithm != Algorithm || !validDigest(ret.Signature) || ret.MaxFrames != MaxFrames) {
		err = fmt.Errorf("worker does not support %s; update video_overlap_worker.py and visual_embedding_server.py and restart it", Algorithm)
	}
	return ret, err
}

func SelectClient(ctx context.Context, mode string, local, remote Client) (Client, Capabilities, string, error) {
	if mode != "auto" && mode != "local" && mode != "remote" {
		return local, Capabilities{}, "", fmt.Errorf("unknown video backend")
	}
	if mode != "local" && remote.URL != "" {
		probe, cancel := context.WithTimeout(ctx, 8*time.Second)
		caps, err := remote.Capabilities(probe)
		cancel()
		if err == nil {
			return remote, caps, "remote", nil
		}
		if mode == "remote" {
			return remote, caps, "remote", err
		}
		caps, localErr := local.Capabilities(ctx)
		return local, caps, "local (remote unavailable: " + err.Error() + ")", localErr
	}
	if mode == "remote" {
		return remote, Capabilities{}, "remote", fmt.Errorf("configure the remote worker URL in System settings first")
	}
	caps, err := local.Capabilities(ctx)
	return local, caps, "local", err
}

type sampled struct {
	Algorithm string  `json:"algorithm"`
	Decoder   string  `json:"decoder"`
	SHA256    string  `json:"sha256"`
	Step      float64 `json:"step"`
	Media     Media   `json:"media"`
	Frames    []struct {
		Time float64 `json:"time"`
		RGB  []byte  `json:"rgb"`
	} `json:"frames"`
}

func validDigest(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == sha256.Size
}

func DigestFile(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		var n int
		n, err = file.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func Measure(source Source) (Source, error) {
	info, err := os.Stat(source.Path)
	if err != nil {
		return source, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return source, fmt.Errorf("select a nonempty regular video file")
	}
	source.Size, source.ModTime = info.Size(), info.ModTime().UnixNano()
	return source, nil
}

func (c Client) Index(ctx context.Context, source Source, config Config, caps Capabilities) (Signature, error) {
	ret := Signature{Source: source, Config: config, Algorithm: Algorithm, IndexedAt: time.Now()}
	if err := config.Validate(); err != nil {
		return ret, err
	}
	ctx, cancel := context.WithTimeout(ctx, 32*time.Minute)
	defer cancel()
	options, err := json.Marshal(config)
	if err != nil {
		return ret, err
	}
	var raw sampled
	if c.URL == "" {
		err = c.local(ctx, &raw, "sample", "--input", source.Path, "--options", string(options))
	} else {
		file, openErr := os.Open(source.Path)
		if openErr != nil {
			return ret, openErr
		}
		defer file.Close()
		var req *http.Request
		req, err = c.request(ctx, http.MethodPost, "/v1/video-overlap/sample", file)
		if err == nil {
			req.ContentLength = source.Size
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Stash-Video-Options", string(options))
			var res *http.Response
			res, err = http.DefaultClient.Do(req)
			if err == nil {
				err = remoteJSON(res, &raw)
			}
		}
	}
	if err != nil {
		return ret, err
	}
	if raw.Algorithm != Algorithm || raw.Decoder != caps.Signature || !validDigest(raw.SHA256) || len(raw.Frames) < 1 || len(raw.Frames) > MaxFrames || raw.Media.Width < 1 || raw.Media.Height < 1 || !finite(raw.Step) || raw.Step < config.SampleSeconds || !finite(raw.Media.Duration) || raw.Media.Duration <= 0 || raw.Media.Duration > 24*3600 {
		return ret, fmt.Errorf("invalid or changed video worker result")
	}
	ret.Decoder, ret.SHA256, ret.Step, ret.Media = raw.Decoder, raw.SHA256, raw.Step, raw.Media
	previous := -1.0
	for _, frame := range raw.Frames {
		if !finite(frame.Time) || frame.Time < 0 || frame.Time <= previous || frame.Time > ret.Media.Duration+1 {
			return ret, fmt.Errorf("invalid sample presentation timestamps")
		}
		f, e := Fingerprint(frame.Time, frame.RGB)
		if e != nil {
			return ret, e
		}
		ret.Frames = append(ret.Frames, f)
		previous = frame.Time
	}
	DownweightRepeats(ret.Frames)
	digest, err := DigestFile(ctx, source.Path)
	if err != nil {
		return ret, err
	}
	measured, err := Measure(source)
	if err != nil {
		return ret, err
	}
	if measured != source || digest != ret.SHA256 {
		return ret, fmt.Errorf("source changed during indexing; retry after scanning")
	}
	return ret, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
