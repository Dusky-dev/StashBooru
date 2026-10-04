package imagerestore

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func ReadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return nil, fmt.Errorf("PNG exceeds restoration dimensions")
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	return png.Decode(f)
}

// Feather uses two finite integer box passes, with zero-padding at canvas edges.
// The same algorithm paints the browser's exact effective-mask overlay.
func Feather(input image.Image, radius int) (*image.Gray, error) {
	if radius < 0 || radius > 16 {
		return nil, fmt.Errorf("feather radius must be 0–16 source pixels")
	}
	b := input.Bounds()
	ret := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(input.At(x, y)).(color.NRGBA)
			if c.A != 255 || c.R != c.G || c.R != c.B {
				return nil, fmt.Errorf("paint mask as opaque grayscale PNG")
			}
			ret.SetGray(x, y, color.Gray{Y: c.R})
		}
	}
	if radius == 0 {
		return ret, nil
	}
	tmp := image.NewGray(b)
	width := 2*radius + 1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		sum := 0
		for x := b.Min.X; x <= min(b.Min.X+radius, b.Max.X-1); x++ {
			sum += int(ret.GrayAt(x, y).Y)
		}
		for x := b.Min.X; x < b.Max.X; x++ {
			tmp.SetGray(x, y, color.Gray{Y: uint8((sum + radius) / width)})
			if x-radius >= b.Min.X {
				sum -= int(ret.GrayAt(x-radius, y).Y)
			}
			if x+radius+1 < b.Max.X {
				sum += int(ret.GrayAt(x+radius+1, y).Y)
			}
		}
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		sum := 0
		for y := b.Min.Y; y <= min(b.Min.Y+radius, b.Max.Y-1); y++ {
			sum += int(tmp.GrayAt(x, y).Y)
		}
		for y := b.Min.Y; y < b.Max.Y; y++ {
			ret.SetGray(x, y, color.Gray{Y: uint8((sum + radius) / width)})
			if y-radius >= b.Min.Y {
				sum -= int(tmp.GrayAt(x, y-radius).Y)
			}
			if y+radius+1 < b.Max.Y {
				sum += int(tmp.GrayAt(x, y+radius+1).Y)
			}
		}
	}
	return ret, nil
}

func WritePNG(path string, input image.Image) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, input); err != nil {
		return err
	}
	return f.Sync()
}

// VerifyPixels distrusts worker metadata. Unmasked RGBA and all alpha must be exact.
func VerifyPixels(source, mask, output string) error {
	a, err := ReadPNG(source)
	if err != nil {
		return err
	}
	m, err := ReadPNG(mask)
	if err != nil {
		return err
	}
	b, err := ReadPNG(output)
	if err != nil {
		return err
	}
	if a.Bounds() != m.Bounds() || a.Bounds() != b.Bounds() {
		return fmt.Errorf("restoration changed canvas dimensions")
	}
	painted := false
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ac := color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)
			bc := color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)
			mc := color.GrayModel.Convert(m.At(x, y)).(color.Gray)
			painted = painted || mc.Y > 0
			if ac.A != bc.A || (mc.Y == 0 && ac != bc) {
				return fmt.Errorf("worker changed unmasked pixels or alpha at %d,%d", x, y)
			}
		}
	}
	if !painted {
		return fmt.Errorf("paint a non-empty mask")
	}
	return nil
}
