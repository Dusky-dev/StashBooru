package image

import (
	"testing"
	"time"
)

func TestParseCaptureDate(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	if got, ok := parseCaptureDate("2019:07:14 18:22:03\x00", now); !ok || got.Format("2006-01-02") != "2019-07-14" {
		t.Fatalf("got %v %v", got, ok)
	}
	for _, raw := range []string{"", "0000:00:00 00:00:00", "    :  :     :  :  ", "2030:01:01 00:00:00", "2019-07-14"} {
		if _, ok := parseCaptureDate(raw, now); ok {
			t.Errorf("%q accepted", raw)
		}
	}
	if _, ok := CaptureDate("/nonexistent.jpg", now); ok {
		t.Error("missing file accepted")
	}
}
