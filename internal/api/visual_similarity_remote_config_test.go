package api

import (
	"encoding/json"
	"testing"
)

func TestRemoteWorkerEnabledLegacyAndExplicitControls(t *testing.T) {
	var legacy visualSimilarityRemoteConfig
	if err := json.Unmarshal([]byte(`{"url":"http://worker.test:8000","token":"secret"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if !legacy.isEnabled() || legacy.FallbackLocal {
		t.Fatal("saved legacy remote workers must remain enabled without implicit fallback")
	}

	disabled := false
	legacy.Enabled = &disabled
	legacy.FallbackLocal = true
	if legacy.isEnabled() {
		t.Fatal("explicitly disabled worker must not be used")
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip visualSimilarityRemoteConfig
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.isEnabled() || !roundTrip.FallbackLocal || roundTrip.Token != "secret" {
		t.Fatalf("worker controls or saved token lost: %+v", roundTrip)
	}
	enabled := true
	roundTrip.Enabled = &enabled
	if !roundTrip.isEnabled() {
		t.Fatal("explicitly enabled configured worker ignored")
	}
	roundTrip.URL = ""
	if roundTrip.isEnabled() {
		t.Fatal("cannot enable an unconfigured worker")
	}
}
