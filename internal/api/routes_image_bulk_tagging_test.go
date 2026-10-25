package api

import (
	"encoding/json"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
)

func TestMergeBulkImageTagsKeepsFilenameIdentitiesAndSourceOrder(t *testing.T) {
	filename := []camietagger.Tag{
		{Name: "hero_(example)", Category: "character", Score: 1, Source: "filename"},
		{Name: "artist_a", Category: "artist", Score: 1, Source: "filename"},
	}
	camie := []camietagger.Tag{
		{Name: "other_hero", Category: "character", Score: .95},
		{Name: "smile", Category: "general", Score: .55, Source: "model"},
	}
	eva := []camietagger.Tag{
		{Name: "third_hero", Category: "character", Score: .99, Source: "eva02"},
		{Name: "smile", Category: "general", Score: .99, Source: "eva02"},
		{Name: "blue_eyes", Category: "general", Score: .8, Source: "eva02"},
		{Name: "safe", Category: "rating", Score: .9, Source: "eva02"},
	}

	merged := mergeBulkImageTags(filename, camie, eva)
	if len(merged) != 5 {
		t.Fatalf("expected 5 tags after filename priority and duplicates, got %d: %+v", len(merged), merged)
	}
	want := []string{
		"character:Hero (Example)",
		"artist:artist a",
		"general:smile",
		"general:blue eyes",
		"rating:safe",
	}
	for i, prediction := range merged {
		if got := prediction.Category + ":" + prediction.Name; got != want[i] {
			t.Fatalf("merged[%d] = %q, want %q", i, got, want[i])
		}
	}
	if merged[2].Score != .55 || merged[2].Source != "model" {
		t.Fatalf("EVA02 should not replace higher-priority Camie prediction: %+v", merged[2])
	}
}

func TestMergeBulkImageTagsAllowsEvaCharactersWithoutFilenameIdentity(t *testing.T) {
	camie := []camietagger.Tag{{Name: "first", Category: "character", Score: .8}}
	eva := []camietagger.Tag{{Name: "second", Category: "character", Score: .7, Source: "eva02"}}
	result := mergeBulkImageTags(nil, camie, eva)
	if len(result) != 2 {
		t.Fatalf("expected both inferred Characters without authoritative filename, got %d", len(result))
	}
}

func TestBulkRequestDecodesExplicitSourceChoices(t *testing.T) {
	var request bulkImageTagRequest
	raw := []byte(`{"imageIDs":[10,11],"all":false,"confirmed":true,"useCamie":false,"useEva02":true,"threshold":0.492,"eva02Threshold":0.35,"limit":50,"applyTags":true}`)
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	if len(request.ImageIDs) != 2 || request.All || !request.Confirmed || request.UseCamie || !request.UseEva02 || !request.ApplyTags {
		t.Fatalf("incorrect request decode: %+v", request)
	}
}
