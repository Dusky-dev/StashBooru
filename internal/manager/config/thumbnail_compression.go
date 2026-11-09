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

func (i *Config) GetImageThumbnailFormat() string {
	value, _ := i.GetUIConfiguration()["thumbnailImageFormat"].(string)
	switch value {
	case "jpg", "webp", "jxl":
		return value
	default:
		return "jpg"
	}
}

func (i *Config) GetImageThumbnailQuality() int {
	return experimentalQuality(i.GetUIConfiguration(), "thumbnailImageQuality", 70)
}

func (i *Config) GetClipPreviewFormat() string {
	value, _ := i.GetUIConfiguration()["thumbnailVideoFormat"].(string)
	if value == "mp4" {
		return "mp4"
	}
	return "webm"
}

func (i *Config) GetClipPreviewQuality() int {
	return experimentalQuality(i.GetUIConfiguration(), "thumbnailVideoQuality", 75)
}
