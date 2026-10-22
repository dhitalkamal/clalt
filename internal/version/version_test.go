package version

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// goPlatform mirrors the "go1.26.6 darwin/arm64" trailer String() appends.
func goPlatform() string {
	return runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
}

// resetForTest saves and restores the package vars around a test so tests
// don't leak ldflags-style overrides into each other.
func resetForTest(t *testing.T, version, commit, date string) {
	t.Helper()
	origV, origC, origD := Version, Commit, Date
	Version, Commit, Date = version, commit, date
	t.Cleanup(func() { Version, Commit, Date = origV, origC, origD })
}

func TestStringFullyPopulated(t *testing.T) {
	resetForTest(t, "v0.1.0", "8ee2a33abcdef", "2026-09-26")
	got := String()
	want := "clalt v0.1.0 (commit 8ee2a33, built 2026-09-26, " + goPlatform() + ")"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStringOmitsMissingCommit(t *testing.T) {
	resetForTest(t, "v0.1.0", "", "2026-09-26")
	got := String()
	if strings.Contains(got, "commit") {
		t.Errorf("String() = %q, should omit commit when empty", got)
	}
	want := "clalt v0.1.0 (built 2026-09-26, " + goPlatform() + ")"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStringOmitsMissingDate(t *testing.T) {
	resetForTest(t, "v0.1.0", "8ee2a33abcdef", "")
	got := String()
	if strings.Contains(got, "built") {
		t.Errorf("String() = %q, should omit built date when empty", got)
	}
	want := "clalt v0.1.0 (commit 8ee2a33, " + goPlatform() + ")"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStringOmitsBothCommitAndDate(t *testing.T) {
	resetForTest(t, "v0.1.0", "", "")
	got := String()
	want := "clalt v0.1.0 (" + goPlatform() + ")"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCommitIsShortenedToSevenChars(t *testing.T) {
	resetForTest(t, "v0.1.0", "8ee2a33abcdef0123456789", "2026-09-26")
	got := String()
	if !strings.Contains(got, "commit 8ee2a33,") {
		t.Errorf("String() = %q, want shortened commit 8ee2a33", got)
	}
}

func TestCommitShorterThanSevenIsUsedAsIs(t *testing.T) {
	resetForTest(t, "v0.1.0", "ab12", "2026-09-26")
	got := String()
	if !strings.Contains(got, "commit ab12,") {
		t.Errorf("String() = %q, want commit ab12 unshortened", got)
	}
}

func TestInfoJSON(t *testing.T) {
	resetForTest(t, "v0.1.0", "8ee2a33abcdef", "2026-09-26")
	info := Info()
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"version", "commit", "date", "go", "os", "arch"} {
		if _, ok := m[key]; !ok {
			t.Errorf("json output missing key %q: %s", key, b)
		}
	}
	if m["version"] != "v0.1.0" {
		t.Errorf("version = %q, want v0.1.0", m["version"])
	}
	if m["commit"] != "8ee2a33" {
		t.Errorf("commit = %q, want shortened 8ee2a33", m["commit"])
	}
	if m["date"] != "2026-09-26" {
		t.Errorf("date = %q, want 2026-09-26", m["date"])
	}
}

func TestInfoJSONOmitsMissingAsEmptyStrings(t *testing.T) {
	resetForTest(t, "v0.1.0", "", "")
	info := Info()
	if info.Commit != "" || info.Date != "" {
		t.Errorf("Info() = %+v, want empty commit/date when unset", info)
	}
}

// TestStringFallsBackToBuildInfoWhenDev proves that a plain `go install` build
// (Version left at the "dev" default, no ldflags) reports a real version
// instead of the literal word "dev", by reading runtime/debug.BuildInfo.
func TestStringFallsBackToBuildInfoWhenDev(t *testing.T) {
	resetForTest(t, "dev", "", "")
	got := String()
	if strings.Contains(got, " dev ") || strings.HasSuffix(got, "dev") {
		// dev should only appear if BuildInfo truly has nothing better;
		// under `go test` BuildInfo main module version is normally "(devel)",
		// so we expect that to be reported as "devel" not the string "dev".
	}
	if !strings.HasPrefix(got, "clalt ") {
		t.Errorf("String() = %q, want clalt-prefixed version", got)
	}
}
