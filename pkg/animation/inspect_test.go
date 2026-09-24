package animation

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/file"
)

func TestImageFrameInspection(t *testing.T) {
	root := t.TempDir()
	frames := []*image.Paletted{}
	for i := range 3 {
		im := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White})
		im.SetColorIndex(i, i, 1)
		frames = append(frames, im)
	}
	var animated, still, plainPNG bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: frames, Delay: []int{2, 3, 4}}); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&still, frames[0], nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&plainPNG, frames[0]); err != nil {
		t.Fatal(err)
	}
	pngBytes := plainPNG.Bytes()
	control := make([]byte, 20)
	binary.BigEndian.PutUint32(control[:4], 8)
	copy(control[4:8], "acTL")
	binary.BigEndian.PutUint32(control[8:12], 3)
	apng := append(append(append([]byte{}, pngBytes[:33]...), control...), pngBytes[33:]...)
	webp := func(count int) []byte {
		var b bytes.Buffer
		b.WriteString("RIFF")
		_ = binary.Write(&b, binary.LittleEndian, uint32(4+count*24))
		b.WriteString("WEBP")
		for range count {
			b.WriteString("ANMF")
			_ = binary.Write(&b, binary.LittleEndian, uint32(16))
			b.Write(make([]byte, 16))
		}
		return b.Bytes()
	}
	for _, tc := range []struct {
		name  string
		data  []byte
		count int
	}{
		{"animated.gif", animated.Bytes(), 3}, {"still.gif", still.Bytes(), 1},
		{"still.png", pngBytes, 1}, {"animated.png", apng, 3},
		{"animated.webp", webp(3), 3}, {"one-frame.webp", webp(1), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name)
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			count, err := Count(context.Background(), &file.OsFS{}, path, "")
			if err != nil || count != tc.count {
				t.Fatalf("count=%d err=%v; want %d", count, err, tc.count)
			}
			if tc.name == "animated.gif" {
				rate, err := FrameRate(context.Background(), &file.OsFS{}, path, count)
				if err != nil {
					t.Fatal(err)
				}
				if want := 3.0 / 0.09; rate < want-0.001 || rate > want+0.001 {
					t.Fatalf("frame rate=%f; want %f", rate, want)
				}
			}
		})
	}
	for _, name := range []string{"broken.gif", "broken.png", "broken.webp"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
			t.Fatal(err)
		}
		if count, err := Count(context.Background(), &file.OsFS{}, path, ""); err == nil || count != 0 {
			t.Fatalf("malformed %s accepted", name)
		}
	}
	if _, err := exec.LookPath("cjxl"); err != nil {
		return
	}
	if _, err := exec.LookPath("jxlinfo"); err != nil {
		return
	}
	for _, name := range []string{"animated", "still"} {
		out := filepath.Join(root, name+".jxl")
		if log, err := exec.Command("cjxl", filepath.Join(root, name+".gif"), out, "--effort=1").CombinedOutput(); err != nil {
			t.Fatalf("cjxl: %v: %s", err, log)
		}
		count, err := Count(context.Background(), &file.OsFS{}, out, "")
		want := 1
		if name == "animated" {
			want = 3
		}
		if err != nil || count != want {
			t.Fatalf("%s JXL frames=%d err=%v", name, count, err)
		}
		if name == "animated" {
			rate, err := FrameRate(context.Background(), &file.OsFS{}, out, count)
			if err != nil || rate <= 0 {
				t.Fatalf("animated JXL frame rate=%f err=%v", rate, err)
			}
		}
	}
}
