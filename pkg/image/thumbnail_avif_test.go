package image

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
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

func TestAVIFLosslessDecodeBridgeSourceDetection(t *testing.T) {
	f := &models.ImageFile{Format: "jpegxl"}
	if !isJXLSource(f) {
		t.Fatal("FFprobe JPEG XL codec should be decoded by libvips")
	}
	f.Format = "jxl"
	if !isJXLSource(f) {
		t.Fatal("JXL alias should be decoded by libvips")
	}
	f.Format = "png"
	if isJXLSource(f) {
		t.Fatal("PNG must not be routed as JPEG XL")
	}
}

func TestAVIFLosslessPNGValidation(t *testing.T) {
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0}
	jpg := []byte{255, 216, 255, 219}
	if !isPNGEncoded(png) || isPNGEncoded(jpg) || isPNGEncoded(nil) {
		t.Fatal("lossless PNG validation failed")
	}
}
