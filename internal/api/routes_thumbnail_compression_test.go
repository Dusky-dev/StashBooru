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

func TestValidateDeleteOldRequiresConfirmationAndNativeCategory(t *testing.T) {
	req := thumbnailCompressionRequest{Action: "deleteOld", Category: "image"}
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("deleteOld must require acknowledgement")
	}
	req.Acknowledged = true
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatal(err)
	}
	req.Category = "animated"
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("animated sources do not have a deletable legacy cache")
	}
	req.Action = "invalid"
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("invalid cache action accepted")
	}
}

func TestValidateAdaptiveThumbnailSettings(t *testing.T) {
	req := thumbnailCompressionRequest{
		Category: "image", Enabled: true, Acknowledged: true,
		Format: "jxl", Quality: 75, Resolution: 480,
		Mode: "adaptive", MinQuality: 55, Budget: 100,
	}
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatal(err)
	}
	req.Resolution = 500
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("invalid resolution accepted")
	}
	req.Resolution = 480
	req.MinQuality = 90
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("minimum quality above max accepted")
	}
	req.MinQuality = 55
	req.Budget = 200
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("invalid size budget accepted")
	}
	req.Budget = 100
	req.Mode = "unknown"
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("unsupported adaptive mode accepted")
	}
}

func TestAVIFThumbnailCompressionRequest(t *testing.T) {
	req := thumbnailCompressionRequest{
		Category: "image", Enabled: true, Format: "avif",
		Quality: 60, Resolution: 320, Mode: "fixed", Acknowledged: true,
	}
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatalf("valid still AVIF rejected: %v", err)
	}
	req.Mode = "adaptive"
	req.MinQuality = 40
	req.Budget = 100
	if err := validateThumbnailCompressionRequest(req); err != nil {
		t.Fatalf("valid adaptive AVIF rejected: %v", err)
	}
	req.Category = "animated"
	if err := validateThumbnailCompressionRequest(req); err == nil {
		t.Fatal("animated AVIF output must not be accepted")
	}
}
