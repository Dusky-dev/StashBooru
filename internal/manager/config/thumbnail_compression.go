package config

// Experimental encoding settings live in the existing persisted UI map, but
// are always validated by the server before being applied to media generation.
func experimentalQuality(settings map[string]interface{}, key string, fallback int) int {
	var quality int
	switch value := settings[key].(type) {
	case int:
		quality = value
	case int64:
		quality = int(value)
	case float64:
		quality = int(value)
	default:
		return fallback
	}
	if quality < 10 || quality > 100 {
		return fallback
	}
	return quality
}

func (i *Config) IsExperimentalImageThumbnailEnabled() bool {
	value, _ := i.GetUIConfiguration()["thumbnailImageEnabled"].(bool)
	return value
}

func (i *Config) IsExperimentalClipPreviewEnabled() bool {
	value, _ := i.GetUIConfiguration()["thumbnailVideoEnabled"].(bool)
	return value
}

func (i *Config) IsExperimentalAnimatedThumbnailEnabled() bool {
	value, _ := i.GetUIConfiguration()["thumbnailAnimatedEnabled"].(bool)
	return value
}

func (i *Config) GetAnimatedThumbnailQuality() int {
	return experimentalQuality(i.GetUIConfiguration(), "thumbnailAnimatedQuality", 70)
}

func (i *Config) GetImageThumbnailFormat() string {
	if !i.IsExperimentalImageThumbnailEnabled() {
		return "jpg"
	}

	value, _ := i.GetUIConfiguration()["thumbnailImageFormat"].(string)
	switch value {
	case "jpg", "webp", "jxl":
		return value
	default:
		return "jpg"
	}
}

func (i *Config) GetImageThumbnailQuality() int {
	if !i.IsExperimentalImageThumbnailEnabled() {
		return 70
	}
	return experimentalQuality(i.GetUIConfiguration(), "thumbnailImageQuality", 70)
}

func (i *Config) GetClipPreviewFormat() string {
	if !i.IsExperimentalClipPreviewEnabled() {
		return "webm"
	}
	value, _ := i.GetUIConfiguration()["thumbnailVideoFormat"].(string)
	if value == "mp4" {
		return "mp4"
	}
	return "webm"
}

func (i *Config) GetClipPreviewQuality() int {
	if !i.IsExperimentalClipPreviewEnabled() {
		return 75
	}
	return experimentalQuality(i.GetUIConfiguration(), "thumbnailVideoQuality", 75)
}
