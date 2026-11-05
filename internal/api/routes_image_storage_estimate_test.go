package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageEstimateRequestRejectsUnsafeArguments(t *testing.T) {
	for _, request := range []storageEstimateRequest{
		{StackID: 0, Fidelity: "byte-exact"},
		{StackID: -4, Fidelity: "byte-exact"},
		{StackID: 1, Fidelity: ""},
		{StackID: 1, Fidelity: "lossy"},
		{Auto: true, StackID: 1, Fidelity: "byte-exact"},
		{Auto: true, StackIDs: []int{2}, Fidelity: "byte-exact"},
	} {
		if err := validateStorageEstimateRequest(request); err == nil {
			t.Fatalf("accepted invalid storage estimate request: %+v", request)
		}
	}
	for _, fidelity := range []string{"byte-exact", "pixel-exact"} {
		if err := validateStorageEstimateRequest(storageEstimateRequest{StackID: 37, Fidelity: fidelity}); err != nil {
			t.Fatalf("valid %q estimate rejected: %v", fidelity, err)
		}
	}
}

func TestStorageEstimateRejectsNonRegularSources(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source.png")
	if err := os.WriteFile(source, []byte("source unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := estimateRegularSource(source); err != nil {
		t.Fatalf("regular source rejected: %v", err)
	}
	for _, path := range []string{"relative.png", filepath.Join(parent, "missing.png"), parent} {
		if err := estimateRegularSource(path); err == nil {
			t.Errorf("nonregular source accepted: %q", path)
		}
	}
	link := filepath.Join(parent, "alias.png")
	if err := os.Symlink(source, link); err == nil {
		if err := estimateRegularSource(link); err == nil {
			t.Fatal("symbolic link accepted as direct image source")
		}
	}
}

func TestStorageEstimateMultiStackValidation(t *testing.T) {
	for _, selection := range []storageEstimateRequest{
		{StackIDs: []int{1, 1}, Fidelity: "byte-exact"},
		{StackIDs: []int{-1, 2}, Fidelity: "byte-exact"},
		{StackID: 1, StackIDs: []int{2}, Fidelity: "byte-exact"},
		{StackIDs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, Fidelity: "byte-exact"},
		{StackIDs: []int{1, 2}, Fidelity: "unknown"},
	} {
		if err := validateStorageEstimateRequest(selection); err == nil {
			t.Fatalf("unsafe multi-stack estimate accepted: %+v", selection)
		}
	}
	for _, selection := range []storageEstimateRequest{
		{StackID: 1, Fidelity: "byte-exact"},
		{StackIDs: []int{1, 3, 5}, Fidelity: "byte-exact"},
		{StackIDs: []int{1, 2, 3, 4, 5, 6, 7, 8}, Fidelity: "pixel-exact"},
		{Auto: true, Fidelity: "byte-exact"},
	} {
		if err := validateStorageEstimateRequest(selection); err != nil {
			t.Fatalf("valid multi-stack selection rejected: %+v: %v", selection, err)
		}
	}
	ids, err := storageEstimateSelectedIDs(storageEstimateRequest{StackIDs: []int{21, 9}, Fidelity: "byte-exact"})
	if err != nil || len(ids) != 2 || ids[0] != 21 || ids[1] != 9 {
		t.Fatalf("unexpected selected stack order: %v, %v", ids, err)
	}
}

func TestStorageEstimateAutoSampleBudgetAndReportMetadata(t *testing.T) {
	folder := t.TempDir()
	good := filepath.Join(folder, "one.png")
	if err := os.WriteFile(good, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if !estimateAutoWithinBudget(estimateSourceGroup{Members: []estimateSourceMember{
		{ID: "image:1", Path: good}, {ID: "image:2", Path: good},
	}}) {
		t.Fatal("valid small read-only sample rejected")
	}
	if estimateAutoWithinBudget(estimateSourceGroup{Members: []estimateSourceMember{{ID: "image:1", Path: good}}}) {
		t.Fatal("singleton accepted")
	}
	symlink := filepath.Join(folder, "link.png")
	if err := os.Symlink(good, symlink); err == nil {
		if estimateAutoWithinBudget(estimateSourceGroup{Members: []estimateSourceMember{
			{ID: "image:1", Path: good}, {ID: "image:2", Path: symlink},
		}}) {
			t.Fatal("symlink accepted")
		}
	}
	original := []byte(`{"readOnly":true,"sourceBytes":5}`)
	value, err := estimateAnnotateAutoReport(original, storageEstimateSelection{
		Mode: "auto", TotalImages: 100, PHashIndexedImages: 50, SampledGroups: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(value), `"totalImages":100`) || !strings.Contains(string(value), `"readOnly":true`) {
		t.Fatalf("report metadata missing: %s", value)
	}
}

func TestStorageEstimatePortableMemberName(t *testing.T) {
	// P12/P13 permits a portable export label even when the real filename
	// has Unicode/punctuation. The original name is preserved as sourceName.
	for _, tc := range []struct {
		source string
		want   string
	}{
		{"/media/lacrima (夏) [variant] ♥.PNG", "image-34526.png"},
		{"/media/creator:patreon:123.jxl", "image-34526.jxl"},
		{"/media/file", "image-34526.img"},
		{"/media/file.üñí", "image-34526.img"},
		{"/media/file.xxxxxxxxxxxxxxxxxxxx", "image-34526.img"},
	} {
		if got := estimatePortableMemberName(34526, tc.source); got != tc.want {
			t.Errorf("portable estimate name for %q: got %q, want %q", tc.source, got, tc.want)
		}
	}

	member := estimateSourceMember{
		ID:   "image:34526",
		Name: estimatePortableMemberName(34526, "/srv/artist (summer) [1].jpg"),
		Path: "/srv/artist (summer) [1].jpg",
	}
	b, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"name":"image-34526.jpg"`) ||
		!strings.Contains(string(b), `"id":"image:34526"`) ||
		!strings.Contains(string(b), `"path":"/srv/artist (summer) [1].jpg"`) {
		t.Fatalf("estimator input must retain original path and identity: %s", b)
	}
}

func TestStorageEstimateAutomaticSampleSizeAndDisjointness(t *testing.T) {
	for _, tc := range []struct {
		request storageEstimateRequest
		want    int
		valid   bool
	}{
		{storageEstimateRequest{Auto: true, Fidelity: "byte-exact"}, 4, true},
		{storageEstimateRequest{Auto: true, SampleSize: 4, Fidelity: "byte-exact"}, 4, true},
		{storageEstimateRequest{Auto: true, SampleSize: 8, Fidelity: "byte-exact"}, 8, true},
		{storageEstimateRequest{Auto: true, SampleSize: 1, Fidelity: "byte-exact"}, 0, false},
		{storageEstimateRequest{Auto: true, SampleSize: 9, Fidelity: "byte-exact"}, 0, false},
		{storageEstimateRequest{Auto: true, SampleSize: -8, Fidelity: "byte-exact"}, 0, false},
		{storageEstimateRequest{StackID: 10, SampleSize: 8, Fidelity: "byte-exact"}, 0, false},
	} {
		got, err := storageEstimateAutoGroupLimit(tc.request)
		if tc.valid && (err != nil || got != tc.want) {
			t.Errorf("sample limit: got %d, %v; want %d for %+v", got, err, tc.want, tc.request)
		}
		if !tc.valid && err == nil {
			t.Errorf("accepted invalid automatic sample size: %+v", tc.request)
		}
		if (validateStorageEstimateRequest(tc.request) == nil) != tc.valid {
			t.Errorf("request validation mismatch for %+v", tc.request)
		}
	}

	a := estimateSourceGroup{Members: []estimateSourceMember{
		{ID: "image:1", Path: "/media/image-one.png"},
		{ID: "image:2", Path: "/media/image-two.png"},
	}}
	seen := make(map[string]bool)
	if !estimateAutoDisjoint(a, seen) {
		t.Fatal("initial sample rejected")
	}
	estimateAutoRemember(a, seen)
	for _, collision := range []estimateSourceGroup{
		{Members: []estimateSourceMember{{ID: "image:1", Path: "/elsewhere/one.png"}, {ID: "image:3", Path: "/media/three.png"}}},
		{Members: []estimateSourceMember{{ID: "image:3", Path: "/media/image-one.png"}, {ID: "image:4", Path: "/media/four.png"}}},
		{Members: []estimateSourceMember{{ID: "image:5", Path: "/media/five.png"}, {ID: "image:5", Path: "/media/six.png"}}},
		{Members: []estimateSourceMember{{ID: "image:5", Path: "/media/five.png"}, {ID: "image:6", Path: "/media/five.png"}}},
	} {
		if estimateAutoDisjoint(collision, seen) {
			t.Fatalf("overlapping sources counted twice: %+v", collision)
		}
	}
	distinct := estimateSourceGroup{Members: []estimateSourceMember{
		{ID: "image:6", Path: "/media/six.png"},
		{ID: "image:7", Path: "/media/seven.png"},
	}}
	if !estimateAutoDisjoint(distinct, seen) {
		t.Fatal("independent group rejected")
	}
}
