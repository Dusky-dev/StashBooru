package match

import "testing"

func TestOrganizedFilterFollowsSetting(t *testing.T) {
	defer func(previous func() bool) { SkipOrganized = previous }(SkipOrganized)

	SkipOrganized = func() bool { return true }
	if f := organizedFilter(); f == nil || *f {
		t.Fatalf("with the flag in use, only unorganized media match: %v", f)
	}
	SkipOrganized = func() bool { return false }
	if f := organizedFilter(); f != nil {
		t.Fatalf("with the flag off, organized media must not be skipped: %v", *f)
	}
}
