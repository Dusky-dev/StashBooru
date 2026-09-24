// Package animation inspects image containers without decoding their pixels.
package animation

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

// Count returns zero when a required optional inspector is unavailable. It never
// guesses that a JPEG XL is still merely because FFmpeg decoded its first frame.
func Count(ctx context.Context, fs models.FS, path, ffprobe string) (int, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gif", ".png", ".apng", ".webp", ".jxl", ".avif", ".avifs", ".heic", ".heif":
	default:
		return 1, nil
	}
	f, err := fs.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	switch ext {
	case ".gif":
		return gifFrames(r)
	case ".png", ".apng":
		return pngFrames(r)
	case ".webp":
		return webpFrames(r)
	}
	// External inspectors need a real path. Zip members are copied one at a time.
	if _, ok := f.(*os.File); !ok {
		tmp, err := os.CreateTemp("", "stash-animation-*"+ext)
		if err != nil {
			return 0, err
		}
		defer os.Remove(tmp.Name())
		_, err = io.Copy(tmp, r)
		closeErr := tmp.Close()
		if err != nil {
			return 0, err
		}
		if closeErr != nil {
			return 0, closeErr
		}
		path = tmp.Name()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if ext == ".jxl" {
		tool, err := exec.LookPath("jxlinfo")
		if err != nil {
			return 0, nil
		}
		cmd := exec.CommandContext(ctx, tool, "-v", path)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return 0, err
		}
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		frames, still := 0, false
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if strings.HasPrefix(line, "frame:") {
				frames++
			}
			still = still || line == "have_animation: 0"
		}
		if scan.Err() != nil {
			cancel()
		}
		if err := cmd.Wait(); err != nil {
			return 0, fmt.Errorf("inspecting JPEG XL frames: %w", err)
		}
		if err := scan.Err(); err != nil {
			return 0, err
		}
		if still {
			return 1, nil
		}
		return frames, nil
	}
	if ffprobe == "" {
		return 0, nil
	}
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-protocol_whitelist", "file,pipe", "-select_streams", "v:0", "-count_frames", "-show_entries", "stream=nb_read_frames", "-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || count < 1 {
		return 0, fmt.Errorf("could not determine image frame count")
	}
	return count, nil
}

// FrameRate returns the average displayed frame rate for animated image
// containers. Zero means timing is unavailable or the image is still.
func FrameRate(ctx context.Context, fs models.FS, path string, frames int) (float64, error) {
	if frames <= 1 {
		return 0, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	f, err := fs.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var duration float64
	switch ext {
	case ".gif":
		duration, err = gifDuration(bufio.NewReader(f))
	case ".png", ".apng":
		duration, err = pngDuration(f)
	case ".webp":
		duration, err = webpDuration(f)
	case ".jxl":
		return jxlFrameRate(ctx, f, path, frames)
	default:
		return 0, nil
	}
	if err != nil || duration <= 0 {
		return 0, err
	}
	return float64(frames) / duration, nil
}

func jxlFrameRate(ctx context.Context, f io.Reader, path string, frames int) (float64, error) {
	if _, ok := f.(*os.File); !ok {
		tmp, err := os.CreateTemp("", "stash-animation-*.jxl")
		if err != nil {
			return 0, err
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, f); err != nil {
			tmp.Close()
			return 0, err
		}
		if err := tmp.Close(); err != nil {
			return 0, err
		}
		path = tmp.Name()
	}

	tool, err := exec.LookPath("jxlinfo")
	if err != nil {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, tool, "-v", path).Output()
	if err != nil {
		return 0, fmt.Errorf("inspecting JPEG XL timing: %w", err)
	}
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "Animation length:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		duration, parseErr := strconv.ParseFloat(fields[2], 64)
		if parseErr != nil || duration <= 0 {
			break
		}
		return float64(frames) / duration, nil
	}
	return 0, nil
}

func gifDuration(r *bufio.Reader) (float64, error) {
	var header [13]byte
	if _, err := io.ReadFull(r, header[:]); err != nil || (string(header[:6]) != "GIF89a" && string(header[:6]) != "GIF87a") {
		return 0, fmt.Errorf("invalid GIF header")
	}
	if header[10]&0x80 != 0 {
		if err := skip(r, 3*int64(1<<(uint(header[10]&7)+1))); err != nil {
			return 0, err
		}
	}
	var milliseconds uint64
	for {
		block, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		switch block {
		case 0x3b:
			return float64(milliseconds) / 1000, nil
		case 0x21:
			label, err := r.ReadByte()
			if err != nil {
				return 0, err
			}
			if label == 0xf9 {
				n, err := r.ReadByte()
				if err != nil || n != 4 {
					return 0, fmt.Errorf("invalid GIF graphics control extension")
				}
				var data [4]byte
				if _, err := io.ReadFull(r, data[:]); err != nil {
					return 0, err
				}
				milliseconds += uint64(binary.LittleEndian.Uint16(data[1:3])) * 10
				if end, err := r.ReadByte(); err != nil || end != 0 {
					return 0, fmt.Errorf("invalid GIF graphics control terminator")
				}
				continue
			}
		case 0x2c:
			var descriptor [9]byte
			if _, err := io.ReadFull(r, descriptor[:]); err != nil {
				return 0, err
			}
			if descriptor[8]&0x80 != 0 {
				if err := skip(r, 3*int64(1<<(uint(descriptor[8]&7)+1))); err != nil {
					return 0, err
				}
			}
			if _, err := r.ReadByte(); err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("invalid GIF block")
		}
		for {
			n, err := r.ReadByte()
			if err != nil {
				return 0, err
			}
			if n == 0 {
				break
			}
			if err := skip(r, int64(n)); err != nil {
				return 0, err
			}
		}
	}
}

func pngDuration(r io.Reader) (float64, error) {
	var h [8]byte
	if _, err := io.ReadFull(r, h[:]); err != nil || string(h[:]) != "\x89PNG\r\n\x1a\n" {
		return 0, fmt.Errorf("invalid PNG header")
	}
	var duration float64
	for {
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return 0, err
		}
		n := int64(binary.BigEndian.Uint32(h[:4]))
		kind := string(h[4:])
		if kind == "fcTL" {
			if n != 26 {
				return 0, fmt.Errorf("invalid APNG frame control")
			}
			payload := make([]byte, 26)
			if _, err := io.ReadFull(r, payload); err != nil {
				return 0, err
			}
			numerator := binary.BigEndian.Uint16(payload[20:22])
			denominator := binary.BigEndian.Uint16(payload[22:24])
			if denominator == 0 {
				denominator = 100
			}
			duration += float64(numerator) / float64(denominator)
			if err := skip(r, 4); err != nil {
				return 0, err
			}
			continue
		}
		if err := skip(r, n+4); err != nil {
			return 0, err
		}
		if kind == "IEND" {
			return duration, nil
		}
	}
}

func webpDuration(r io.Reader) (float64, error) {
	var h [12]byte
	if _, err := io.ReadFull(r, h[:]); err != nil || string(h[:4]) != "RIFF" || string(h[8:]) != "WEBP" {
		return 0, fmt.Errorf("invalid WebP header")
	}
	remaining := int64(binary.LittleEndian.Uint32(h[4:8])) - 4
	var milliseconds uint64
	for remaining > 0 {
		if remaining < 8 {
			return 0, fmt.Errorf("truncated WebP container")
		}
		if _, err := io.ReadFull(r, h[:8]); err != nil {
			return 0, err
		}
		n := int64(binary.LittleEndian.Uint32(h[4:8]))
		padded := n + n%2
		if padded > remaining-8 {
			return 0, fmt.Errorf("truncated WebP chunk")
		}
		if string(h[:4]) == "ANMF" {
			if n < 16 {
				return 0, fmt.Errorf("invalid WebP animation frame")
			}
			var data [16]byte
			if _, err := io.ReadFull(r, data[:]); err != nil {
				return 0, err
			}
			milliseconds += uint64(data[12]) | uint64(data[13])<<8 | uint64(data[14])<<16
			if err := skip(r, padded-16); err != nil {
				return 0, err
			}
		} else if err := skip(r, padded); err != nil {
			return 0, err
		}
		remaining -= 8 + padded
	}
	return float64(milliseconds) / 1000, nil
}

func skip(r io.Reader, n int64) error {
	_, err := io.CopyN(io.Discard, r, n)
	return err
}

func pngFrames(r io.Reader) (int, error) {
	var h [8]byte
	if _, err := io.ReadFull(r, h[:]); err != nil || string(h[:]) != "\x89PNG\r\n\x1a\n" {
		return 0, fmt.Errorf("invalid PNG header")
	}
	for {
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return 0, err
		}
		n := int64(binary.BigEndian.Uint32(h[:4]))
		switch string(h[4:]) {
		case "acTL":
			if n != 8 {
				return 0, fmt.Errorf("invalid APNG animation header")
			}
			if _, err := io.ReadFull(r, h[:]); err != nil {
				return 0, err
			}
			return int(binary.BigEndian.Uint32(h[:4])), nil
		case "IDAT", "IEND":
			return 1, nil
		}
		if err := skip(r, n+4); err != nil {
			return 0, err
		}
	}
}

func webpFrames(r io.Reader) (int, error) {
	var h [12]byte
	if _, err := io.ReadFull(r, h[:]); err != nil || string(h[:4]) != "RIFF" || string(h[8:]) != "WEBP" {
		return 0, fmt.Errorf("invalid WebP header")
	}
	remaining := int64(binary.LittleEndian.Uint32(h[4:8])) - 4
	frames := 0
	for remaining > 0 {
		if remaining < 8 {
			return 0, fmt.Errorf("truncated WebP container")
		}
		if _, err := io.ReadFull(r, h[:8]); err != nil {
			return 0, err
		}
		n := int64(binary.LittleEndian.Uint32(h[4:8]))
		n += n % 2
		if n > remaining-8 {
			return 0, fmt.Errorf("truncated WebP chunk")
		}
		if string(h[:4]) == "ANMF" {
			frames++
		}
		if err := skip(r, n); err != nil {
			return 0, err
		}
		remaining -= 8 + n
	}
	return max(1, frames), nil
}

func gifFrames(r *bufio.Reader) (int, error) {
	var header [13]byte
	if _, err := io.ReadFull(r, header[:]); err != nil || (string(header[:6]) != "GIF89a" && string(header[:6]) != "GIF87a") {
		return 0, fmt.Errorf("invalid GIF header")
	}
	if header[10]&0x80 != 0 {
		if err := skip(r, 3*int64(1<<(uint(header[10]&7)+1))); err != nil {
			return 0, err
		}
	}
	frames := 0
	for {
		block, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		switch block {
		case 0x3b:
			return frames, nil
		case 0x21:
			if _, err := r.ReadByte(); err != nil {
				return 0, err
			}
		case 0x2c:
			var descriptor [9]byte
			if _, err := io.ReadFull(r, descriptor[:]); err != nil {
				return 0, err
			}
			if descriptor[8]&0x80 != 0 {
				if err := skip(r, 3*int64(1<<(uint(descriptor[8]&7)+1))); err != nil {
					return 0, err
				}
			}
			if _, err := r.ReadByte(); err != nil { // LZW code size
				return 0, err
			}
			frames++
		default:
			return 0, fmt.Errorf("invalid GIF block")
		}
		for {
			n, err := r.ReadByte()
			if err != nil {
				return 0, err
			}
			if n == 0 {
				break
			}
			if err := skip(r, int64(n)); err != nil {
				return 0, err
			}
		}
	}
}
