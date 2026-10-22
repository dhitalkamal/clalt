package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/version"
)

// resetVersionForTest pins version vars for a deterministic dispatch test.
func resetVersionForTest(t *testing.T) {
	t.Helper()
	origV, origC, origD := version.Version, version.Commit, version.Date
	version.Version, version.Commit, version.Date = "v0.1.0", "8ee2a33abcdef", "2026-09-26"
	t.Cleanup(func() { version.Version, version.Commit, version.Date = origV, origC, origD })
}

func TestCmdVersionPrintsHumanReadableString(t *testing.T) {
	resetVersionForTest(t)
	app, out, _ := newTestApp(t)
	if err := app.cmdVersion(nil); err != nil {
		t.Fatalf("cmdVersion: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != version.String() {
		t.Errorf("cmdVersion output = %q, want %q", got, version.String())
	}
}

func TestCmdVersionJSON(t *testing.T) {
	resetVersionForTest(t)
	app, out, _ := newTestApp(t)
	if err := app.cmdVersion([]string{"--json"}); err != nil {
		t.Fatalf("cmdVersion: %v", err)
	}
	var data version.Data
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		t.Fatalf("version --json invalid: %v\n%s", err, out.String())
	}
	if data.Version != "v0.1.0" || data.Commit != "8ee2a33" || data.Date != "2026-09-26" {
		t.Errorf("unexpected version data: %+v", data)
	}
}

func TestRunDispatchesVersionCommand(t *testing.T) {
	resetVersionForTest(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	code := Run([]string{"version"})
	if code != 0 {
		t.Errorf("Run([version]) exit code = %d, want 0", code)
	}
}

func TestRunDispatchesDashDashVersionFlag(t *testing.T) {
	resetVersionForTest(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	code := Run([]string{"--version"})
	if code != 0 {
		t.Errorf("Run([--version]) exit code = %d, want 0", code)
	}
}

func TestRunDispatchesDashVFlag(t *testing.T) {
	resetVersionForTest(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	code := Run([]string{"-v"})
	if code != 0 {
		t.Errorf("Run([-v]) exit code = %d, want 0", code)
	}
}
