package crew

import (
	"fmt"
	"regexp"
	"strings"
)

// Series is the Kiro Crew release line clalt will start a gateway of: 0.9.x,
// the one whose command line and config were read for docs/plans/crew.md. a
// version with anything after its digits (0.9.0rc1) is not a release and is
// refused. CLALT_CREW_ALLOW_UNTESTED=1 starts any version that can be read.
const Series = "0.9"

// AllowUntestedVar is the variable that lifts the version check.
const AllowUntestedVar = "CLALT_CREW_ALLOW_UNTESTED"

var (
	versionRe = regexp.MustCompile(`(?m)^\s*kirocrew\s+v?(\d[^\s]*)`)
	releaseRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
)

// ParseVersion finds the version in what `kirocrew --version` printed, which is
// "kirocrew 0.9.0", suffix included. lines before it do not matter.
func ParseVersion(out string) (string, error) {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("cannot read a Kiro Crew version from %q", firstLine(out))
	}
	return m[1], nil
}

// CheckVersion refuses every version but a plain 0.9.x release.
func CheckVersion(version string) error {
	if m := releaseRe.FindStringSubmatch(version); m != nil && m[1]+"."+m[2] == Series {
		return nil
	}
	return fmt.Errorf("kirocrew %s is not a release of the %s.x series that clalt supports. set %s=1 to try it anyway", version, Series, AllowUntestedVar)
}

// firstLine is the first line that says anything, cut short enough to put in an
// error.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			if len(line) > 200 {
				line = strings.ToValidUTF8(line[:200], "")
			}
			return line
		}
	}
	return ""
}
