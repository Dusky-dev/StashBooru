package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageEstimateRequestRejectsUnsafeArguments(t *testing.T) {
	for _, request := range []storageEstimateRequest{
		{StackID: 0, Fidelity: "byte-exact"},
		{StackID: -4, Fidelity: "byte-exact"},
		{StackID: 1, Fidelity: ""},
		{StackID: 1, Fidelity: "lossy"},
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
