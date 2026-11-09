package image

import "testing"

func TestUniqueThumbnailValuesRetainsOrder(t *testing.T) {
	got := uniqueThumbnailValues(320, 320, 480, 320, 640)
	want := []int{320, 480, 640}
	if len(got) != len(want) {
		t.Fatalf("values: %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values: %v, want %v", got, want)
		}
	}
}
