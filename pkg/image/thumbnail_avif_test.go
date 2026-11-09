package image

import "testing"

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
