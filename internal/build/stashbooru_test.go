package build

import "testing"

func TestCompareStableVersions(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            int
		valid           bool
	}{
		{"1.10.0", "1.9.0", 1, true}, {"1.0.0", "1.0.0", 0, true},
		{"1.0.0", "1.1.0", -1, true}, {"2.0.0", "1.99.99", 1, true},
		{"v0.30.0", "1.0.0", 0, false}, {"1.1.0-rc.1", "1.0.0", 0, false},
		{"01.0.0", "1.0.0", 0, false}, {"1.0.0", "unknown", 0, false},
		{"999999999999999999999.0.0", "1.0.0", 0, false},
	} {
		got, valid := CompareStableVersions(tc.latest, tc.current)
		if got != tc.want || valid != tc.valid {
			t.Errorf("%s versus %s: got %d, %v", tc.latest, tc.current, got, valid)
		}
	}
	if _, valid := CompareStableVersions(StashBooruVersion(), StashBooruVersion()); !valid {
		t.Fatal("invalid embedded release version")
	}
}
