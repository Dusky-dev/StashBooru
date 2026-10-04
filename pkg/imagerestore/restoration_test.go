package imagerestore

import (
	"archive/zip"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaskFeatherAndUnmaskedPixelContract(t *testing.T) {
	dir := t.TempDir()
	source := image.NewNRGBA(image.Rect(0, 0, 7, 7))
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 30), B: 12, A: uint8(50 + x*20)})
		}
	}
	mask := image.NewGray(source.Bounds())
	mask.SetGray(3, 3, color.Gray{Y: 255})
	effective, err := Feather(mask, 1)
	require.NoError(t, err)
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			expected := uint8(0)
			if x >= 2 && x <= 4 && y >= 2 && y <= 4 {
				expected = 28
			}
			require.Equal(t, expected, effective.GrayAt(x, y).Y)
		}
	}
	output := image.NewNRGBA(source.Bounds())
	copy(output.Pix, source.Pix)
	pixel := output.NRGBAAt(3, 3)
	pixel.R = 255
	output.SetNRGBA(3, 3, pixel)
	for name, input := range map[string]image.Image{"source.png": source, "mask.png": effective, "output.png": output} {
		require.NoError(t, WritePNG(filepath.Join(dir, name), input))
	}
	require.NoError(t, VerifyPixels(filepath.Join(dir, "source.png"), filepath.Join(dir, "mask.png"), filepath.Join(dir, "output.png")))
	pixel = output.NRGBAAt(0, 0)
	pixel.R++
	output.SetNRGBA(0, 0, pixel)
	require.NoError(t, os.Remove(filepath.Join(dir, "output.png")))
	require.NoError(t, WritePNG(filepath.Join(dir, "output.png"), output))
	require.ErrorContains(t, VerifyPixels(filepath.Join(dir, "source.png"), filepath.Join(dir, "mask.png"), filepath.Join(dir, "output.png")), "unmasked")
	mask.SetGray(0, 0, color.Gray{Y: 255})
	pixel.A++
	output.SetNRGBA(0, 0, pixel)
	require.NoError(t, os.Remove(filepath.Join(dir, "output.png")))
	require.NoError(t, WritePNG(filepath.Join(dir, "output.png"), output))
	require.ErrorContains(t, VerifyPixels(filepath.Join(dir, "source.png"), filepath.Join(dir, "mask.png"), filepath.Join(dir, "output.png")), "alpha")
	_, err = Feather(mask, 17)
	require.Error(t, err)
}

func TestExplicitRemoteAndCorruptTransport(t *testing.T) {
	t.Setenv("STASH_IMAGE_RESTORATION_WORKER", "must-not-run")
	emptyClient, emptyCaps, notice, err := SelectWorker(context.Background(), "remote", Client{}, Client{})
	require.Empty(t, emptyClient.URL)
	require.False(t, emptyCaps.Available)
	require.Empty(t, notice)
	require.ErrorContains(t, err, "remote worker URL")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(Capabilities{Protocol: 1, MaxPixels: MaxPixels, WorkSize: 512})
			return
		}
		w.Header().Set("Content-Length", "3")
		w.Header().Set("X-Stash-Content-SHA256", "wrong")
		_, _ = w.Write([]byte("bad"))
	}))
	defer server.Close()
	client := Client{URL: server.URL, Token: "token"}
	selected, caps, _, err := SelectWorker(context.Background(), "remote", Client{}, client)
	require.NoError(t, err)
	require.Equal(t, client, selected)
	require.False(t, caps.Available)
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input"), filepath.Join(dir, "output")
	require.NoError(t, os.WriteFile(input, []byte("fixture"), 0600))
	require.ErrorContains(t, client.Process(context.Background(), input, output), "corrupted")
	_, err = os.Stat(output)
	require.True(t, os.IsNotExist(err))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, client.Process(ctx, input, output))
	_, err = os.Stat(output)
	require.True(t, os.IsNotExist(err))
}

func TestPreviewJournalPruningAndBoundedArchive(t *testing.T) {
	s := Store{Root: t.TempDir()}
	id := "0123456789abcdef0123456789abcdef"
	record := &Record{ID: id, Status: "preview", ExpiresAt: time.Now().Add(-time.Hour)}
	require.NoError(t, s.Save(record))
	loaded, err := s.Load(id)
	require.NoError(t, err)
	require.Equal(t, record.ID, loaded.ID)
	_, _, err = s.Prune(time.Now())
	require.NoError(t, err)
	_, err = s.Load(id)
	require.Error(t, err)
	record.Status = "saved"
	require.NoError(t, s.Save(record))
	_, _, err = s.Prune(time.Now())
	require.NoError(t, err)
	_, err = s.Load(id)
	require.NoError(t, err)
	_, err = s.Directory("../../escape")
	require.Error(t, err)
	archive := filepath.Join(t.TempDir(), "malicious.zip")
	f, err := os.Create(archive)
	require.NoError(t, err)
	z := zip.NewWriter(f)
	w, err := z.Create("../escape.png")
	require.NoError(t, err)
	_, err = w.Write([]byte("bad"))
	require.NoError(t, err)
	w, err = z.Create("receipt.json")
	require.NoError(t, err)
	_, _ = w.Write([]byte("{}"))
	require.NoError(t, z.Close())
	require.NoError(t, f.Close())
	_, err = Unbundle(archive, t.TempDir(), "output")
	require.ErrorContains(t, err, "invalid restoration result entry")
	options := Options{Hardware: "auto", Seed: 42, Steps: 20, Guidance: 7.5, Strength: 1}
	require.NoError(t, options.Validate())
	options.Guidance = math.NaN()
	require.Error(t, options.Validate())
}

func TestLocalCancellationRemovesPartialResult(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 is needed for process cancellation fixture")
	}
	dir := t.TempDir()
	worker := filepath.Join(dir, "worker.py")
	require.NoError(t, os.WriteFile(worker, []byte("import sys,time\nfrom pathlib import Path\nPath(sys.argv[-1]).write_bytes(b'partial result')\ntime.sleep(60)\n"), 0600))
	t.Setenv("STASH_PYTHON", python)
	t.Setenv("STASH_IMAGE_RESTORATION_WORKER", worker)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output := filepath.Join(dir, "result.zip")
	require.Error(t, (Client{}).Process(ctx, filepath.Join(dir, "input.zip"), output))
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	_, err = os.Stat(output)
	require.True(t, os.IsNotExist(err))
}
