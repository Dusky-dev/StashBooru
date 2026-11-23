package api

import (
	"context"
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

	merged := mergeBulkImageTags(filename, nil, camie, eva)
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
	result := mergeBulkImageTags(nil, nil, camie, eva)
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

func TestBulkRequestFilenameOnlyNeedsNoModelOptIn(t *testing.T) {
	config := defaultCamieConfig()
	config.FilenameEnabled = true
	base := func() bulkImageTagRequest {
		var request bulkImageTagRequest
		request.Confirmed = true
		request.ImageIDs = []int{1, 2}
		request.ApplyCharacters = true
		return request
	}

	filenameOnly := base()
	filenameOnly.UseFilename = true
	if _, err := validateBulkImageTagRequest(&filenameOnly, config, 0); err != nil {
		t.Fatalf("filename-only bulk tagging must not need the model opt-in: %v", err)
	}

	withModel := base()
	withModel.UseFilename = true
	withModel.UseCamie = true
	if status, err := validateBulkImageTagRequest(&withModel, config, 0); err == nil || status != 409 {
		t.Fatalf("model bulk tagging must stay opt-in, got %d %v", status, err)
	}
	config.BulkTaggingEnabled = true
	withModel = base()
	withModel.UseCamie = true
	if _, err := validateBulkImageTagRequest(&withModel, config, 0); err != nil {
		t.Fatalf("opted-in model bulk tagging rejected: %v", err)
	}
	if withModel.Threshold != config.Threshold || withModel.Limit != config.Limit {
		t.Fatalf("model defaults not filled: %+v", withModel)
	}

	config.FilenameEnabled = false
	filenameOff := base()
	filenameOff.UseFilename = true
	if status, err := validateBulkImageTagRequest(&filenameOff, config, 0); err == nil || status != 409 {
		t.Fatalf("filename source must be rejected while filename reading is off, got %d %v", status, err)
	}

	noSource := base()
	if _, err := validateBulkImageTagRequest(&noSource, config, 0); err == nil {
		t.Fatal("a request without any source must be rejected")
	}
	single := base()
	single.UseFilename = true
	single.ImageIDs = []int{1}
	config.FilenameEnabled = true
	if _, err := validateBulkImageTagRequest(&single, config, 0); err == nil {
		t.Fatal("a single Image must use individual review")
	}
}

func TestBulkFilenameOnlyJobPathNeedsNoWorkers(t *testing.T) {
	config := defaultCamieConfig()
	config.FilenameEnabled = true
	var request bulkImageTagRequest
	request.Confirmed = true
	request.ImageIDs = []int{1, 2}
	request.UseFilename = true
	request.ApplyCharacters = true
	request.ApplyArtist = true
	request.ApplyCopyright = true
	if _, err := validateBulkImageTagRequest(&request, config, 0); err != nil {
		t.Fatal(err)
	}

	run := func(path string) []camietagger.Tag {
		t.Helper()
		// No Camie or EVA02 client is started for a filename-only job.
		predictions, err := inferBulkImageTags(context.Background(), 1, path, config, request, bulkImageTagClients{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		predictions = filterCamieBulkPredictionsV2(predictions, request.camieBulkRequestV2)
		predictions, err = validateCamiePredictionsV2(predictions)
		if err != nil {
			t.Fatal(err)
		}
		return predictions
	}

	matched := run("/library/[artist_a](series_b).hero_(series_b)_0123456789abcdef0123456789abcdef.png")
	got := map[string]bool{}
	for _, prediction := range matched {
		got[prediction.Category+":"+prediction.Name] = true
	}
	for _, want := range []string{"artist:artist a", "copyright:Series B", "character:Hero (Series B)"} {
		if !got[want] {
			t.Fatalf("missing %q in %+v", want, matched)
		}
	}
	if unmatched := run("/library/holiday.png"); len(unmatched) != 0 {
		t.Fatalf("a non-matching filename must apply nothing, got %+v", unmatched)
	}
}

func TestBulkSkipsAmbiguousCharacters(t *testing.T) {
	predictions := []camietagger.Tag{
		{Name: "Hero", Category: "character"},
		{Name: "Sidekick", Category: "character"},
		{Name: "smile", Category: "general"},
	}
	enriched := []camietagger.Tag{
		{Name: "Hero", Category: "character", TargetCandidates: []camietagger.TargetCandidate{{ID: 1}, {ID: 2}}},
		{Name: "Sidekick", Category: "character"},
		{Name: "smile", Category: "general"},
	}
	kept, ambiguous := withoutAmbiguousBulkCharacters(predictions, enriched)
	if len(kept) != 2 || kept[0].Name != "Sidekick" || len(ambiguous) != 1 || ambiguous[0] != "Hero" {
		t.Fatalf("kept %+v, ambiguous %v", kept, ambiguous)
	}
}
