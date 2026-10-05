package build

import (
	_ "embed"
	"regexp"
	"strconv"
	"strings"
)

//go:embed stashbooru-version.txt
var stashBooruVersion string

// StashBooruVersion is independent of the upstream Stash build version.
func StashBooruVersion() string { return strings.TrimSpace(stashBooruVersion) }

var stableVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// CompareStableVersions compares complete stable release versions numerically.
// Unknown/development versions are not interpreted as available updates.
func CompareStableVersions(latest, current string) (int, bool) {
	if !stableVersionPattern.MatchString(latest) || !stableVersionPattern.MatchString(current) {
		return 0, false
	}
	a, b := strings.Split(latest, "."), strings.Split(current, ".")
	comparison := 0
	for i := range a {
		av, ae := strconv.ParseUint(a[i], 10, 64)
		bv, be := strconv.ParseUint(b[i], 10, 64)
		if ae != nil || be != nil {
			return 0, false
		}
		if comparison == 0 {
			if av > bv {
				comparison = 1
			} else if av < bv {
				comparison = -1
			}
		}
	}
	return comparison, true
}
