package image

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAVIFQualityCRFIsMonotonic(t *testing.T) {
	if avifQualityCRF(100) != 0 || avifQualityCRF(60) != 22 || avifQualityCRF(10) != 49 {
		t.Fatalf("unexpected AVIF quality mapping: 100=%d 60=%d 10=%d", avifQualityCRF(100), avifQualityCRF(60), avifQualityCRF(10))
	}
	for q := 10; q < 100; q++ {
		if avifQualityCRF(q) < avifQualityCRF(q+1) {
			t.Fatalf("quality %d maps to higher fidelity than %d in CRF", q, q+1)
		}
	}
}

func TestAVIFPreflightChecksContainerSignature(t *testing.T) {
	good := []byte{0, 0, 0, 32, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f', 0, 0, 0, 0}
	bad := []byte{0, 0, 0, 32, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0, 0, 0, 0}
	if !isAVIFEncoded(good) || isAVIFEncoded(bad) || isAVIFEncoded(nil) {
		t.Fatal("invalid AVIF signature detection")
	}
}

func TestAVIFLosslessJXLHandoff(t *testing.T) {
	original := []byte("jpegxl-original")
	losslessPNG := []byte("lossless-png")
	for _, decodeFirst := range []bool{true, false} {
		calls := []string{}
		out, err := encodeAVIFWithLosslessFallback(original, decodeFirst,
			func(in []byte) ([]byte, error) {
				if string(in) == string(original) {
					calls = append(calls, "ffmpeg-original")
					return nil, fmt.Errorf("no JPEG XL decoder")
				}
				if string(in) != string(losslessPNG) {
					t.Fatalf("unexpected AVIF input: %q", in)
				}
				calls = append(calls, "ffmpeg-png")
				return []byte("encoded-avif"), nil
			},
			func() ([]byte, error) {
				calls = append(calls, "vips-png")
				return losslessPNG, nil
			})
		if err != nil || string(out) != "encoded-avif" {
			t.Fatalf("lossless handoff failed: %q %v", out, err)
		}
		want := "vips-png,ffmpeg-png"
		if !decodeFirst {
			want = "ffmpeg-original,vips-png,ffmpeg-png"
		}
		if strings.Join(calls, ",") != want {
			t.Fatalf("unexpected decode/encode sequence: %v, want %s", calls, want)
		}
	}
}

func TestAVIFLosslessFallbackPreservesError(t *testing.T) {
	sourceError := errors.New("jpegxl decoder missing")
	out, err := encodeAVIFWithLosslessFallback([]byte("jxl"), true,
		func([]byte) ([]byte, error) { return nil, sourceError },
		func() ([]byte, error) { return nil, errors.New("vips decode failed") })
	if out != nil || !errors.Is(err, sourceError) || !strings.Contains(err.Error(), "vips decode failed") {
		t.Fatalf("expected both errors preserved, got %v", err)
	}
}

func TestVipsPNGThumbnailOutputIsLossless(t *testing.T) {
	if got := vipsThumbnailOutput("png", 10); got != ".png[compression=3,strip]" {
		t.Fatalf("unexpected PNG handoff options: %s", got)
	}
}
