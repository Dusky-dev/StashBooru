package api

// Tests run without a configuration directory; use the default settings.
func init() {
	loadDuplicateDetectionConfig = func() (duplicateDetectionConfig, error) {
		return defaultDuplicateDetectionConfig(), nil
	}
}
