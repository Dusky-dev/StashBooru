package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Test fixtures use the same booru content identifier.
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilenameBooruMD5(t *testing.T) {
	const hash = "ABCDEF0123456789ABCDEF0123456789"
	path := filepath.Join("somewhere", "[artist].character_"+hash+".jxl")
	if got := filenameBooruMD5(path); got != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("filenameBooruMD5() = %q", got)
	}
	if got := filenameBooruMD5("no-hash-here.png"); got != "" {
		t.Fatalf("filenameBooruMD5() without hash = %q", got)
	}
}

func TestCalculateBooruMD5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.bin")
	contents := []byte("stashbooru booru hash test")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("%x", md5.Sum(contents)) //nolint:gosec // Booru identifier fixture.
	got, err := calculateBooruMD5(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("calculateBooruMD5() = %q, want %q", got, expected)
	}
}

func TestLookupImageBooruMetadataFallsBackToFileHash(t *testing.T) {
	contents := []byte("actual file content")
	actualHash := fmt.Sprintf("%x", md5.Sum(contents)) //nolint:gosec // Booru identifier fixture.
	const staleHash = "11111111111111111111111111111111"
	path := filepath.Join(t.TempDir(), "image_"+staleHash+".jpg")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	var lookedUp []string
	provider := booruProvider{name: "TestBooru"}
	post := &booruPost{ID: "42", MD5: actualHash}
	lookup := func(_ context.Context, hash string) (booruProvider, *booruPost, error) {
		lookedUp = append(lookedUp, hash)
		if hash == staleHash {
			return booruProvider{}, nil, errBooruNoMatch
		}
		if hash == actualHash {
			return provider, post, nil
		}
		return booruProvider{}, nil, fmt.Errorf("unexpected hash %s", hash)
	}

	gotProvider, gotPost, gotHash, source, err := lookupImageBooruMetadata(context.Background(), path, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if gotProvider.name != provider.name || gotPost != post || gotHash != actualHash || source != "file" {
		t.Fatalf("unexpected result: provider=%q post=%v hash=%q source=%q", gotProvider.name, gotPost, gotHash, source)
	}
	wantLookups := []string{staleHash, actualHash}
	if !reflect.DeepEqual(lookedUp, wantLookups) {
		t.Fatalf("lookups = %#v, want %#v", lookedUp, wantLookups)
	}
}

func TestLookupImageBooruMetadataDoesNotRepeatSameHash(t *testing.T) {
	contents := []byte("same hash in filename")
	hash := fmt.Sprintf("%x", md5.Sum(contents)) //nolint:gosec // Booru identifier fixture.
	path := filepath.Join(t.TempDir(), hash+".png")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	calls := 0
	lookup := func(_ context.Context, got string) (booruProvider, *booruPost, error) {
		calls++
		if got != hash {
			t.Fatalf("lookup hash = %q, want %q", got, hash)
		}
		return booruProvider{}, nil, errBooruNoMatch
	}

	_, _, gotHash, source, err := lookupImageBooruMetadata(context.Background(), path, lookup)
	if !errors.Is(err, errBooruNoMatch) {
		t.Fatalf("error = %v, want errBooruNoMatch", err)
	}
	if calls != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls)
	}
	if gotHash != hash || source != "file" {
		t.Fatalf("hash/source = %q/%q", gotHash, source)
	}
}

func TestParseBooruArrayAndCategorizedTags(t *testing.T) {
	fixture := []byte(`[{"id":123,"md5":"abc","tag_string":"solo rem_(re:zero) re:zero artist_name","tag_string_general":"solo","tag_string_character":"rem_(re:zero)","tag_string_copyright":"re:zero","tag_string_artist":"artist_name","tag_string_meta":"highres"}]`)
	post, err := parseBooruArray(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "123" {
		t.Fatalf("post ID = %q", post.ID)
	}

	tags := booruTags(post, "Danbooru", nil)
	got := map[string]string{}
	for _, tag := range tags {
		got[tag.RawName] = tag.Category
	}
	want := map[string]string{
		"artist_name":   "artist",
		"re:zero":       "copyright",
		"rem_(re:zero)": "character",
		"solo":          "general",
		"highres":       "meta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories = %#v, want %#v", got, want)
	}
}

func TestParseGelbooruTagCategories(t *testing.T) {
	fixture := []byte(`{"tag":[{"name":"artist_name","type":1},{"name":"series_name","type":"3"},{"name":"character_name","type":4},{"name":"solo","type":0},{"name":"highres","type":5}]}`)
	got, err := parseGelbooruTagCategories(fixture)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"artist_name":    "artist",
		"series_name":    "copyright",
		"character_name": "character",
		"solo":           "general",
		"highres":        "meta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories = %#v, want %#v", got, want)
	}
}

func TestBooruTagsUsesResolvedCategoriesForFlatPost(t *testing.T) {
	post := &booruPost{Tags: "artist_name series_name character_name solo"}
	categories := map[string]string{
		"artist_name":    "artist",
		"series_name":    "copyright",
		"character_name": "character",
	}
	tags := booruTags(post, "Gelbooru", categories)
	got := map[string]string{}
	for _, tag := range tags {
		got[tag.RawName] = tag.Category
	}
	if got["artist_name"] != "artist" || got["series_name"] != "copyright" || got["character_name"] != "character" || got["solo"] != "general" {
		t.Fatalf("unexpected flat-post categories: %#v", got)
	}
}
