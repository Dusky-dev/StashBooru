package api

import (
	"encoding/json"
	"testing"
)

func TestBulkTaggingDisabledByDefaultAndSavedAsOptIn(t *testing.T) {
	defaults := defaultCamieConfig()
	if defaults.BulkTaggingEnabled {
		t.Fatal("bulk tagging must remain disabled unless explicitly enabled")
	}
	if err := validateCamieConfig(defaults); err != nil {
		t.Fatalf("invalid default Camie config: %v", err)
	}

	// Existing server config files without the new field must remain safe.
	var legacy camieConfig
	if err := json.Unmarshal([]byte(`{"threshold":0.492,"eva02Threshold":0.35,"limit":50,"filenameEnabled":true,"filenameLayout":"[%artist%](%copyright%).%character%_%md5%.%ext%"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.BulkTaggingEnabled {
		t.Fatal("legacy config unexpectedly enables bulk tagging")
	}

	defaults.BulkTaggingEnabled = true
	encoded, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var restored camieConfig
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.BulkTaggingEnabled {
		t.Fatal("saved bulk-tagging option was lost")
	}
}
