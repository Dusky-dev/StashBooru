package api

import (
	"encoding/json"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/pixaitagger"
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

func TestRemoteWorkerProcessingSelection(t *testing.T) {
	enabled, disabled := true, false
	tests := []struct {
		name        string
		config      visualSimilarityRemoteConfig
		requested   string
		wantBackend string
		wantURL     string
	}{
		{"legacy remote", visualSimilarityRemoteConfig{URL: "https://worker.test", Token: "secret"}, "remote", "remote", "https://worker.test"},
		{"enabled remote without fallback", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled}, "remote", "remote", "https://worker.test"},
		{"auto with fallback disabled", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled}, "auto", "remote", "https://worker.test"},
		{"legacy auto without fallback", visualSimilarityRemoteConfig{URL: "https://worker.test"}, "auto", "remote", "https://worker.test"},
		{"enabled remote with fallback", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled, FallbackLocal: true}, "remote", "remote", "https://worker.test"},
		{"enabled auto", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled, FallbackLocal: true}, "auto", "auto", "https://worker.test"},
		{"explicit local", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled}, "local", "local", ""},
		{"disabled remote fails closed", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &disabled}, "remote", "remote", ""},
		{"disabled remote with fallback still fails closed", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &disabled, FallbackLocal: true}, "remote", "remote", ""},
		{"disabled auto without fallback is local", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &disabled}, "auto", "local", ""},
		{"disabled auto fails closed", visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &disabled, FallbackLocal: true}, "auto", "local", ""},
		{"legacy no worker", visualSimilarityRemoteConfig{}, "remote", "remote", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotBackend, gotURL, gotToken := tc.config.processingBackend(tc.requested)
			if gotBackend != tc.wantBackend || gotURL != tc.wantURL {
				t.Fatalf("worker policy returned %q and %q, want %q and %q", gotBackend, gotURL, tc.wantBackend, tc.wantURL)
			}
			if tc.wantURL == "" && gotToken != "" {
				t.Fatal("disabled remote must not expose its saved token to worker clients")
			}
			if tc.wantURL != "" && gotToken != tc.config.Token {
				t.Fatal("enabled remote worker lost its configured token")
			}
		})
	}
}

func TestModelManagementNeverFallsBackToAnotherWorker(t *testing.T) {
	disabled, enabled := false, true
	remoteConfig := visualSimilarityRemoteConfig{
		URL: "https://worker.test",
		Token: "secret",
		Enabled: &enabled,
		FallbackLocal: true,
	}
	if backend, url := remoteConfig.modelManagementTarget(); backend != "remote" || url != remoteConfig.URL {
		t.Fatalf("enabled management status target should be remote: %q, %q", backend, url)
	}
	camieRemote, err := camieModelManagementClient(remoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer camieRemote.Close()
	if _, ok := camieRemote.(*camietagger.RemoteClient); !ok {
		t.Fatalf("Camie model management unexpectedly switched to %T", camieRemote)
	}
	pixaiRemote, err := pixaiModelManagementClient(remoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pixaiRemote.Close()
	if _, ok := pixaiRemote.(*pixaitagger.RemoteClient); !ok {
		t.Fatalf("PixAI model management unexpectedly switched to %T", pixaiRemote)
	}

	remoteConfig.Enabled = &disabled
	if backend, url := remoteConfig.modelManagementTarget(); backend != "local" || url != "" {
		t.Fatalf("disabled management status target should be local: %q, %q", backend, url)
	}
	camieLocal, err := camieModelManagementClient(remoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer camieLocal.Close()
	if _, ok := camieLocal.(*camietagger.Client); !ok {
		t.Fatalf("disabled Camie remote still selected %T", camieLocal)
	}
	pixaiLocal, err := pixaiModelManagementClient(remoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pixaiLocal.Close()
	if _, ok := pixaiLocal.(*pixaitagger.Client); !ok {
		t.Fatalf("disabled PixAI remote still selected %T", pixaiLocal)
	}
}

func TestLocalEva02InstallAllowedWithDisabledRemoteOrFallback(t *testing.T) {
	enabled, disabled := true, false
	tests := []struct {
		config visualSimilarityRemoteConfig
		want   bool
	}{
		{visualSimilarityRemoteConfig{}, true},
		{visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &disabled}, true},
		{visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled, FallbackLocal: true}, true},
		{visualSimilarityRemoteConfig{URL: "https://worker.test", Enabled: &enabled}, false},
		{visualSimilarityRemoteConfig{URL: "https://worker.test"}, false},
	}
	for _, tc := range tests {
		if got := tc.config.allowLocalModelDownload(); got != tc.want {
			t.Fatalf("local EVA02 download allowed=%v want=%v for %+v", got, tc.want, tc.config)
		}
	}
}
