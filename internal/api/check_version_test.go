package api

import "testing"

func TestStashBooruReleaseVersion(t *testing.T) {
	for _, tc := range []struct {
		tag                      string
		draft, prerelease, valid bool
	}{
		{"stashbooru-v1.0.0", false, false, true}, {"stashbooru-v1.10.0", false, false, true},
		{"v0.30.0", false, false, false}, {"latest_develop", false, true, false},
		{"stashbooru-v1.1.0-rc.1", false, false, false}, {"stashbooru-v1.1.0", true, false, false},
		{"stashbooru-v1.1.0", false, true, false}, {"stashbooru-v01.0.0", false, false, false},
	} {
		_, err := stashBooruReleaseVersion(githubReleasesResponse{Tag_name: tc.tag, Draft: tc.draft, Prerelease: tc.prerelease})
		if (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}
