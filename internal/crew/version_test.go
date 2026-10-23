package crew

import (
	"strings"
	"testing"
)

func TestTheVersionIsReadFromWhatKirocrewPrints(t *testing.T) {
	for out, want := range map[string]string{
		"kirocrew 0.9.0\n":                         "0.9.0",
		"kirocrew 0.9.0":                           "0.9.0",
		"kirocrew v0.10.1\n":                       "0.10.1",
		"warning: something odd\nkirocrew 0.8.0\n": "0.8.0",
		"kirocrew 0.9.0rc1\n":                      "0.9.0rc1",
		"kirocrew 0.10.0-rc.1\n":                   "0.10.0-rc.1",
		"  kirocrew   0.7.2  (build 20261013)  \n": "0.7.2",
	} {
		got, err := ParseVersion(out)
		if err != nil || got != want {
			t.Errorf("ParseVersion(%q) = %q, %v, want %q", out, got, err, want)
		}
	}
}

func TestOutputThatIsNotAVersionIsAnError(t *testing.T) {
	for _, out := range []string{"", "\n", "usage: kirocrew [-h]\n", "kirocrew\n", "python 3.12.4\n"} {
		if got, err := ParseVersion(out); err == nil {
			t.Errorf("ParseVersion(%q) = %q, want an error", out, got)
		}
	}
}

func TestOnlyTheNineSeriesIsSupported(t *testing.T) {
	for version, ok := range map[string]bool{
		"0.6.0": false, "0.7.2": false, "0.8.0": false, "0.8.9": false,
		"0.9.0": true, "0.9.9": true, "0.9.10": true, "0.10.0": false, "0.10.1": false, "1.0.0": false, "1.9.0": false,
		"0.9": false, "0.9.0.1": false, "0.9.0rc1": false, "0.9.0-beta": false, "0.10.0-rc.1": false, "0.9.0+build": false,
	} {
		if err := CheckVersion(version); (err == nil) != ok {
			t.Errorf("CheckVersion(%q) = %v, want ok=%v", version, err, ok)
		}
	}
}

func TestARefusedVersionNamesItAndTheSeriesAndTheOverride(t *testing.T) {
	err := CheckVersion("0.10.0")
	if err == nil {
		t.Fatal("0.10.0 was accepted")
	}
	for _, want := range []string{"0.10.0", "0.9.x", "CLALT_CREW_ALLOW_UNTESTED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not mention %q", err, want)
		}
	}
}

func TestAVersionThatCannotBeComparedIsRefused(t *testing.T) {
	if err := CheckVersion("nightly"); err == nil {
		t.Error("nightly was accepted")
	}
}
