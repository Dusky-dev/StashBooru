package image

import (
	"context"
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

func TestGIFScanAverageFrameRateIncludesFinalHold(t *testing.T) {
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe required")
	}
	path := filepath.Join(t.TempDir(), "final-hold.gif")
	animation := &gif.GIF{LoopCount: 0}
	for i := range 120 {
		frame := image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.Black, color.White})
		frame.SetColorIndex(i%16, i/16, 1)
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 4)
	}
	animation.Delay[119] = 26
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = gif.EncodeAll(f, animation)
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	d := Decorator{FFProbe: ffmpeg.NewFFProbe(probe)}
	scanned, err := d.Decorate(context.Background(), &file.OsFS{}, &models.BaseFile{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	vf, ok := scanned.(*models.VideoFile)
	if !ok {
		t.Fatalf("unexpected type %T", scanned)
	}
	if vf.FrameCount != 120 || math.Abs(vf.FrameRate-120/5.02) > 0.000001 || math.Abs(vf.Duration-5.02) > 0.000001 {
		t.Fatalf("frames=%d rate=%v duration=%v", vf.FrameCount, vf.FrameRate, vf.Duration)
	}
}
