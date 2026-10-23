package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

func settingsRead(t *testing.T, rs booruBrowserRoutes) booruSettingsResponse {
	t.Helper()
	w := httptest.NewRecorder()
	rs.Settings(w, httptest.NewRequest("GET", "/booru-browser/settings", nil))
	var result booruSettingsResponse
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("settings read failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"apiKey":`) || strings.Contains(w.Body.String(), "fixture-secret") {
		t.Fatal("saved API key leaked")
	}
	return result
}
func settingsWrite(rs booruBrowserRoutes, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/booru-browser/settings", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	rs.UpdateSettings(w, request)
	return w
}
func TestBooruSettingsCredentialLifecycleAndSources(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	if len(settingsRead(t, rs).Sources) != len(booru.Defaults()) {
		t.Fatal("presets missing")
	}
	save := func(action string, source booru.Source, clearCredentials bool, want int) {
		t.Helper()
		update := booruSettingsUpdate{Revision: settingsRead(t, rs).Revision, Action: action, Source: source, ClearAPIKey: clearCredentials}
		payload, err := json.Marshal(update)
		if err != nil {
			t.Fatal(err)
		}
		w := settingsWrite(rs, string(payload))
		if w.Code != want || strings.Contains(w.Body.String(), "fixture-secret") {
			t.Fatalf("unexpected write result: %d %s", w.Code, w.Body.String())
		}
	}
	source := booru.Source{ID: "custom", Name: "Custom", Engine: "gelbooru", URL: "https://board.example.org", MediaHosts: []string{"cdn.example.org"}, UserID: "42", APIKey: "fixture-secret"}
	save("save", source, false, 200)
	source.APIKey = ""
	source.Name = "Renamed"
	save("save", source, false, 200)
	path := filepath.Join(rs.configPath, "booru-browser.json")
	data, _ := os.ReadFile(path)
	loaded, err := booru.Configure(data)
	if err != nil || loaded[len(loaded)-1].APIKey != "fixture-secret" {
		t.Fatal("blank key did not retain credential")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("credential file is not private")
	}
	source.URL = "https://other.example.org"
	save("save", source, false, 400)
	save("save", source, true, 200)
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "fixture-secret") {
		t.Fatal("credential not removed")
	}
	source.UserID = ""
	source.Disabled = true
	save("save", source, false, 200)
	w := httptest.NewRecorder()
	rs.Sources(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), `"id":"custom"`) {
		t.Fatal("disabled source is searchable")
	}
	source.Disabled = false
	save("save", source, false, 200)
	source.Login = "unsupported"
	save("save", source, false, 400)
	source.Login = ""
	save("remove", source, false, 200)
	if len(settingsRead(t, rs).Sources) != len(booru.Defaults()) {
		t.Fatal("custom source was not removed")
	}
	preset := booru.Defaults()[0]
	preset.Disabled = true
	save("save", preset, false, 200)
	if !settingsRead(t, rs).Sources[0].Disabled {
		t.Fatal("preset toggle lost")
	}
	save("remove", preset, false, 400)
	save("reset", preset, false, 200)
	if settingsRead(t, rs).Sources[0].Disabled {
		t.Fatal("preset reset failed")
	}
	if err := os.WriteFile(path, []byte(`[{"id":"danbooru","disabled":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	view := settingsRead(t, rs)
	if view.Sources[0].URL != preset.URL || !view.Sources[0].Disabled {
		t.Fatal("legacy disabled preset lost")
	}
}
func TestBooruSettingsRejectUnsafeWritesAndConcurrentStaleEdits(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	revision := settingsRead(t, rs).Revision
	payload := fmt.Sprintf(`{"revision":%q,"action":"save","source":{"id":"custom","name":"Custom","engine":"philomena","url":"https://board.example.org","apiKey":"fixture-secret"}}`, revision)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- settingsWrite(rs, payload).Code }()
	}
	wg.Wait()
	close(results)
	sum := 0
	for status := range results {
		sum += status
	}
	if sum != 200+409 {
		t.Fatalf("concurrent saves did not detect stale revision: %d", sum)
	}
	before, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-browser.json"))
	for _, body := range []string{payload, `{}`, `{"apiKey":"fixture-secret"}`, payload + `{}`, strings.Repeat("x", (1<<20)+1)} {
		if w := settingsWrite(rs, body); w.Code == 200 || strings.Contains(w.Body.String(), "fixture-secret") {
			t.Fatal("unsafe payload accepted or echoed")
		}
	}
	for _, headers := range []map[string]string{{"Content-Type": "text/plain"}, {"Content-Type": "application/json", "Origin": "https://evil.example.org"}, {"Content-Type": "application/json", "Origin": "null"}, {"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}} {
		request := httptest.NewRequest("POST", "https://stash.example.org/booru-browser/settings", strings.NewReader(payload))
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		rs.UpdateSettings(w, request)
		if w.Code != 403 && w.Code != 415 {
			t.Fatal("cross-site/non-JSON write not rejected")
		}
	}
	bad := strings.Replace(payload, revision, settingsRead(t, rs).Revision, 1)
	bad = strings.Replace(bad, "https://board.example.org", "https://fixture-secret@127.0.0.1", 1)
	w := settingsWrite(rs, bad)
	if w.Code != 400 || strings.Contains(w.Body.String(), "fixture-secret") {
		t.Fatal("unsafe URL accepted or reflected")
	}
	after, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-browser.json"))
	if string(before) != string(after) {
		t.Fatal("rejected edit altered configuration")
	}
	if err := os.WriteFile(filepath.Join(rs.configPath, "booru-browser.json"), []byte(`{"apiKey":"fixture-secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	rs.Settings(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "fixture-secret") {
		t.Fatal("malformed configuration leaked")
	}
}
func TestBooruSettingsFailedSaveKeepsConfiguration(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	if err := os.Mkdir(filepath.Join(rs.configPath, "booru-browser.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := rs.writeSettings([]byte("fixture-secret")); err == nil {
		t.Fatal("expected atomic rename failure")
	}
	files, _ := os.ReadDir(rs.configPath)
	if len(files) != 1 {
		t.Fatal("temporary credential file leaked")
	}
}

func TestBooruPreviewAddsTagOnlyToOriginSourceBlacklist(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	danbooru := booru.Defaults()[0]
	danbooru.Login, danbooru.APIKey = "account", "fixture-secret"
	danbooru.BlacklistTags = []string{"old_tag"}
	gelbooru := booru.Defaults()[2]
	initial := booruSettingsUpdate{Revision: settingsRead(t, rs).Revision, Action: "save", Source: danbooru}
	payload, _ := json.Marshal(initial)
	if w := settingsWrite(rs, string(payload)); w.Code != 200 {
		t.Fatalf("fixture save failed: %s", w.Body.String())
	}
	add := func(source, instance, tag string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(booruSettingsUpdate{
			Action: "blacklist-add", Source: booru.Source{ID: source}, Instance: instance, Tag: tag,
		})
		return settingsWrite(rs, string(body))
	}
	for _, tag := range []string{"New Tag", "new_tag", "NEW_TAG"} {
		if w := add(danbooru.ID, danbooru.Instance(), tag); w.Code != 200 || strings.Contains(w.Body.String(), "fixture-secret") {
			t.Fatalf("blacklist update failed or leaked credentials: %d %s", w.Code, w.Body.String())
		}
	}
	data, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-browser.json"))
	active, err := booru.Configure(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range active {
		if source.ID == danbooru.ID {
			if source.APIKey != "fixture-secret" || source.Login != "account" {
				t.Fatal("blacklist action replaced credentials")
			}
			if len(source.BlacklistTags) != 2 || source.BlacklistTags[0] != "old_tag" || source.BlacklistTags[1] != "New_Tag" {
				t.Fatalf("blacklist did not deduplicate and append: %#v", source.BlacklistTags)
			}
			posts := booru.Result{Posts: []booru.Post{{Tags: []booru.Tag{{Name: "NEW TAG"}}}}}
			if source.FilterBlacklisted(posts).BlacklistedCount != 1 {
				t.Fatal("new blacklist tag does not hide matching posts")
			}
		}
		if source.ID == gelbooru.ID && len(source.BlacklistTags) != 0 {
			t.Fatal("blacklist leaked to another source")
		}
	}
	for _, bad := range []struct {
		source, instance, tag string
		status                int
	}{
		{"missing", danbooru.Instance(), "tag", 409},
		{danbooru.ID, gelbooru.Instance(), "tag", 409},
		{danbooru.ID, "", "tag", 409},
		{danbooru.ID, danbooru.Instance(), "bad\ntag", 400},
		{danbooru.ID, danbooru.Instance(), " tag ", 400},
		{danbooru.ID, danbooru.Instance(), strings.Repeat("x", 257), 400},
	} {
		if w := add(bad.source, bad.instance, bad.tag); w.Code != bad.status {
			t.Fatalf("accepted invalid action %#v: %d %s", bad, w.Code, w.Body.String())
		}
	}
	final, _ := os.ReadFile(filepath.Join(rs.configPath, "booru-browser.json"))
	if string(final) != string(data) {
		t.Fatal("rejected or duplicate update changed configuration")
	}
}

func TestBooruPreviewBlacklistAddsAreSerialized(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	source := booru.Defaults()[0]
	var wg sync.WaitGroup
	results := make(chan int, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload, _ := json.Marshal(booruSettingsUpdate{
				Action: "blacklist-add", Source: booru.Source{ID: source.ID},
				Instance: source.Instance(), Tag: fmt.Sprintf("tag_%d", i),
			})
			results <- settingsWrite(rs, string(payload)).Code
		}(i)
	}
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatalf("concurrent append failed: HTTP %d", code)
		}
	}
	settings := settingsRead(t, rs)
	for _, item := range settings.Sources {
		if item.ID == source.ID && len(item.BlacklistTags) != 8 {
			t.Fatalf("concurrent additions lost tags: %#v", item.BlacklistTags)
		}
	}
}
