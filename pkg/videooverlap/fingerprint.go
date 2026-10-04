package videooverlap

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/corona10/goimagehash"
	"github.com/disintegration/imaging"
)

// Fingerprint consumes a 32x32 RGB preview. Uniform dark padding is removed
// conservatively; aspect/rotation and original dimensions remain in Media.
func Fingerprint(t float64, rgb []byte) (Frame, error) {
	ret := Frame{Time: t}
	if len(rgb) != 32*32*3 {
		return ret, fmt.Errorf("invalid RGB preview size")
	}
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < 32*32; i++ {
		im.SetNRGBA(i%32, i/32, color.NRGBA{R: rgb[3*i], G: rgb[3*i+1], B: rgb[3*i+2], A: 255})
	}
	bounds := im.Bounds()
	darkRow := func(y int) bool {
		for x := 0; x < 32; x++ {
			p := im.NRGBAAt(x, y)
			if p.R > 12 || p.G > 12 || p.B > 12 {
				return false
			}
		}
		return true
	}
	darkCol := func(x int) bool {
		for y := 0; y < 32; y++ {
			p := im.NRGBAAt(x, y)
			if p.R > 12 || p.G > 12 || p.B > 12 {
				return false
			}
		}
		return true
	}
	for bounds.Min.Y < 12 && darkRow(bounds.Min.Y) && darkRow(bounds.Max.Y-1) {
		bounds.Min.Y++
		bounds.Max.Y--
	}
	for bounds.Min.X < 12 && darkCol(bounds.Min.X) && darkCol(bounds.Max.X-1) {
		bounds.Min.X++
		bounds.Max.X--
	}
	cropped := im.SubImage(bounds)
	p, err := goimagehash.PerceptionHash(cropped)
	if err != nil {
		return ret, err
	}
	d, err := goimagehash.DifferenceHash(cropped)
	if err != nil {
		return ret, err
	}
	ret.Hash, ret.DHash = p.GetHash(), d.GetHash()
	gray := imaging.Grayscale(cropped)
	var mean, square float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			v := float64(gray.NRGBAAt(x-bounds.Min.X, y-bounds.Min.Y).R)
			mean += v
			square += v * v
			p := im.NRGBAAt(x, y)
			ret.RGB[0] += float64(p.R)
			ret.RGB[1] += float64(p.G)
			ret.RGB[2] += float64(p.B)
		}
	}
	n := float64(bounds.Dx() * bounds.Dy())
	mean /= n
	square /= n
	for i := range ret.RGB {
		ret.RGB[i] /= n
	}
	if mean > 8 && mean < 247 && math.Sqrt(math.Max(0, square-mean*mean)) > 10 {
		ret.Weight = 1
	}
	return ret, nil
}

func DownweightRepeats(frames []Frame) {
	counts := map[[2]uint64]int{}
	for _, f := range frames {
		counts[[2]uint64{f.Hash, f.DHash}]++
	}
	for i := range frames {
		n := counts[[2]uint64{frames[i].Hash, frames[i].DHash}]
		frames[i].Weight /= math.Sqrt(float64(n))
		// Four or more identical title/blank samples cannot establish a segment.
		if n >= 4 {
			frames[i].Weight = 0
		}
	}
}

// Nine disjoint bands guarantee a shared band for a 64-bit Hamming distance
// of at most eight. Common posting lists are bounded explicitly by the store.
func Bands(hash uint64) [9]uint16 {
	var ret [9]uint16
	for i := 0; i < 8; i++ {
		ret[i] = uint16((hash >> uint(7*i)) & 127)
	}
	ret[8] = uint16(hash >> 56)
	return ret
}
