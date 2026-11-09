package api

import "testing"

func TestValidateThumbnailCompressionRequestRequiresAcknowledgement(t *testing.T) {
	req := thumbnailCompressionRequest{Category: "image", Enabled: true, Format: "jxl", Quality: 55}
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("enabled cache rebuild must require acknowledgement")
	}
	req.Acknowledged = true
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatal(err)
	}
	req.Category = "animated"
	req.Format = "jxl"
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("animated JXL must be rejected")
	}
	req.Format = "webp"
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatal(err)
	}
	req.Category = "clip"
	req.Format = "mp4"
	req.Quality = 10
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatal(err)
	}
	req.Quality = 101
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("invalid quality accepted")
	}
}
