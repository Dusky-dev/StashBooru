package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

type booruTagStoreFixture struct {
	entries map[string]*models.Tag
	created []string
	fail    bool
}

func (s *booruTagStoreFixture) FindByName(_ context.Context, name string, _ bool) (*models.Tag, error) {
	if s.fail {
		return nil, fmt.Errorf("tag lookup failed")
	}
	return s.entries[strings.ToLower(name)], nil
}

func (s *booruTagStoreFixture) Create(_ context.Context, input *models.CreateTagInput) error {
	if s.fail {
		return fmt.Errorf("tag create failed")
	}
	input.ID = len(s.entries) + 1
	s.entries[strings.ToLower(input.Name)] = input.Tag
	s.created = append(s.created, input.Name)
	return nil
}

func TestBooruDownloadTagsAreBoundedAndSanitized(t *testing.T) {
	items := []booru.Tag{
		{Name: " solo "},
		{Name: "SOLO"},
		{Name: ""},
		{Name: "bad\nvalue"},
		{Name: strings.Repeat("x", 201)},
		{Name: "blue_eyes", Category: "general"},
	}
	for i := range 600 {
		items = append(items, booru.Tag{Name: fmt.Sprintf("tag_%d", i)})
	}
	got := boundedBooruDownloadTags(items)
	if len(got) != booruMaxDownloadTags || got[0].Name != "solo" || got[1].Name != "blue_eyes" || got[2].Name != "tag_0" {
		t.Fatalf("unexpected bounded tags: %d %v", len(got), got[:min(len(got), 3)])
	}
}

func TestBooruDownloadTagsReuseExistingAndRetrySafely(t *testing.T) {
	store := &booruTagStoreFixture{entries: map[string]*models.Tag{
		"solo": {ID: 17, Name: "Solo"},
	}}
	tags := []booru.Tag{{Name: "solo"}, {Name: "blue_eyes"}, {Name: "BLUE_EYES"}}
	first, err := ensureBooruImportTagIDs(context.Background(), store, tags)
	if err != nil || len(first) != 2 || first[0] != 17 || first[1] == 0 || len(store.created) != 1 {
		t.Fatalf("unexpected tag import: ids=%v created=%v err=%v", first, store.created, err)
	}
	second, err := ensureBooruImportTagIDs(context.Background(), store, tags)
	if err != nil || len(store.created) != 1 || second[0] != first[0] || second[1] != first[1] {
		t.Fatalf("retry duplicated tags: ids=%v created=%v err=%v", second, store.created, err)
	}
	store.fail = true
	if _, err := ensureBooruImportTagIDs(context.Background(), store, tags); err == nil {
		t.Fatal("tag repository errors must not be ignored")
	}
}

func TestBooruImportKeepsNativeBooruCategories(t *testing.T) {
	// Some APIs include a flat tag list and categorized fields. The
	// categorized version must replace the earlier unknown/general type.
	tags := []booru.Tag{
		{Name: "Saber", Category: "unknown"},
		{Name: "artist_name", Category: "unknown"},
		{Name: "fate_(series)", Category: "general"},
		{Name: "blue_eyes", Category: "general"},
		{Name: "source_request", Category: "meta"},
		{Name: "mystery", Category: "unknown"},
		{Name: "saber", Category: "character"},
		{Name: "ARTIST_NAME", Category: "artist"},
		{Name: "fate_(series)", Category: "copyright"},
	}
	ordinary, typed := splitBooruImportTags(tags)
	if len(typed) != 3 || typed[0].Category != "character" || typed[1].Category != "artist" || typed[2].Category != "copyright" {
		t.Fatalf("native Stash metadata categories were lost: %+v", typed)
	}
	if typed[0].Source != "booru:download" || typed[0].Score != 1 || typed[0].RawName != "Saber" {
		t.Fatalf("native resolver did not receive verified booru provenance: %+v", typed[0])
	}
	if len(ordinary) != 3 || ordinary[0].Name != "blue_eyes" || ordinary[1].Name != "source_request" || ordinary[2].Name != "mystery" {
		t.Fatalf("untyped/general/meta tags should remain ordinary tags: %+v", ordinary)
	}
}

func TestBooruDownloadTagsLateCategoryPromotionAcrossLimit(t *testing.T) {
	tags := []booru.Tag{{Name: "character_name", Category: "unknown"}}
	for i := range booruMaxDownloadTags + 20 {
		tags = append(tags, booru.Tag{Name: fmt.Sprintf("ordinary_%d", i), Category: "general"})
	}
	// Categories arriving after the snapshot limit can still classify
	// an earlier retained name without expanding the snapshot.
	tags = append(tags, booru.Tag{Name: "CHARACTER_NAME", Category: "character"})
	bounded := boundedBooruDownloadTags(tags)
	if len(bounded) != booruMaxDownloadTags || bounded[0].Category != "character" {
		t.Fatalf("late typed metadata must win without exceeding bounds: %d %+v", len(bounded), bounded[0])
	}
}

func TestBooruDownloadTagSettingAndMediaSnapshot(t *testing.T) {
	rs := booruBrowserRoutes{configPath: t.TempDir()}
	preferences, err := rs.downloadSettings()
	if err != nil || preferences.ImportTags {
		t.Fatalf("tag import must default to off: %+v %v", preferences, err)
	}
	w := httptest.NewRecorder()
	rs.UpdateDownloadSettings(w, toolRequest("POST", "/", `{"importTags":true}`, "application/json"))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	preferences, err = rs.downloadSettings()
	if err != nil || !preferences.ImportTags {
		t.Fatalf("tag import preference was not persisted: %+v %v", preferences, err)
	}

	source := booru.Defaults()[0]
	post := booru.Post{ID: "tag-fixture", Tags: []booru.Tag{
		{Name: "some_general_tag", Category: "general"},
		{Name: "a_character", Category: "character"},
		{Name: "a_second_general_tag", Category: "general"},
	}}
	key := rememberBooruPost(source, post, true)
	_, saved, ok := rs.mediaSnapshot(key)
	if !ok || len(saved.Tags) != 3 {
		t.Fatalf("opt-in snapshot lost provider tags: %+v", saved.Tags)
	}
	key = rememberBooruPost(source, post)
	_, saved, ok = rs.mediaSnapshot(key)
	if !ok || len(saved.Tags) != 1 || saved.Tags[0].Name != "a_character" {
		t.Fatalf("default snapshot should retain only filename taxonomy: %+v", saved.Tags)
	}
}
