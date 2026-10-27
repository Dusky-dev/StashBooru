package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func TestBooruAutoTagSettingsDefaultOffAndWhitelist(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	defaultSettings, err := rs.downloadSettings()
	if err != nil || defaultSettings.AutoTagImages || !defaultSettings.AutoTagCamie ||
		len(defaultSettings.AutoTagSources) != 0 {
		t.Fatalf("auto-tagging must default to opt-out: %+v err=%v", defaultSettings, err)
	}
	danbooru, gelbooru := booru.Defaults()[0], booru.Defaults()[2]
	settings := booruDownloadSettings{
		MaxFileSizeMiB: 256, AutoTagImages: true,
		AutoTagCamie: true, AutoTagEva02: true,
		AutoTagSources: []string{booruAutoTagSourceKey(danbooru)},
	}
	write := func(value booruDownloadSettings, want int) {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		rs.UpdateDownloadSettings(w, toolRequest("POST", "/", string(body), "application/json"))
		if w.Code != want {
			t.Fatalf("settings write returned %d, want %d: %s", w.Code, want, w.Body.String())
		}
	}
	write(settings, 200)
	loaded, err := rs.downloadSettings()
	if err != nil || !loaded.autoTagsSource(danbooru) || loaded.autoTagsSource(gelbooru) ||
		!loaded.AutoTagCamie || !loaded.AutoTagEva02 {
		t.Fatalf("source whitelist/model options were not persisted: %+v err=%v", loaded, err)
	}
	changed := danbooru
	changed.URL = "https://another.example.org"
	if loaded.autoTagsSource(changed) {
		t.Fatal("changed site inherited previously allowed auto-tagging")
	}
	alias := danbooru
	alias.ID = "another-name"
	if loaded.autoTagsSource(alias) {
		t.Fatal("same-site alias inherited another source's auto-tagging permission")
	}
	settings.AutoTagSources = nil
	write(settings, 400)
	settings.AutoTagSources = []string{booruAutoTagSourceKey(danbooru)}
	settings.AutoTagCamie, settings.AutoTagEva02 = false, false
	write(settings, 400)
	settings.AutoTagCamie = true
	settings.AutoTagSources = []string{"untrusted"}
	write(settings, 400)
	settings.AutoTagSources = []string{booruAutoTagSourceKey(danbooru), booruAutoTagSourceKey(danbooru)}
	write(settings, 400)
	settings.AutoTagImages = false
	settings.AutoTagSources = nil
	write(settings, 200)
	loaded, err = rs.downloadSettings()
	if err != nil || loaded.AutoTagImages || loaded.autoTagsSource(danbooru) {
		t.Fatalf("disabling auto-tagging failed: %+v err=%v", loaded, err)
	}
}

func TestBooruDownloadedAutoTaggingOnlyAppliesToImagesAndKeepsFailuresNonFatal(t *testing.T) {
	settings := booruDownloadSettings{AutoTagImages: true, AutoTagCamie: true, AutoTagEva02: true}
	calls := 0
	rs := booruBrowserRoutes{autoTagImage: func(_ context.Context, id int, camie, eva bool) error {
		calls++
		if id != 42 || !camie || !eva {
			t.Fatalf("wrong tagging target or selected models: id=%d camie=%v eva=%v", id, camie, eva)
		}
		return nil
	}}
	result := booruLibraryResult{Kind: "image", ID: "42", Notice: "Converted."}
	if !rs.autoTagDownloadedImage(context.Background(), settings, &result) || calls != 1 ||
		!strings.Contains(result.Notice, "Automatic Image tagging completed") {
		t.Fatalf("automatic tagging did not finish: calls=%d result=%+v", calls, result)
	}
	scene := booruLibraryResult{Kind: "scene", ID: "42"}
	if rs.autoTagDownloadedImage(context.Background(), settings, &scene) || calls != 1 {
		t.Fatal("scene was incorrectly passed to Image tagging")
	}
	rs.autoTagImage = func(context.Context, int, bool, bool) error {
		return errors.New("worker unavailable")
	}
	failed := booruLibraryResult{Kind: "image", ID: "42"}
	if rs.autoTagDownloadedImage(context.Background(), settings, &failed) ||
		!strings.Contains(failed.Notice, "failed") {
		t.Fatalf("tagging failure should be shown as a nonfatal import notice: %+v", failed)
	}
}
