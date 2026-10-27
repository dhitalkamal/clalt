package claudecli

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestVersionParsesClaudeOutput(t *testing.T) {
	cases := map[string]string{
		"2.1.303 (Claude Code)\n": "2.1.303",
		"2.1.303":                 "2.1.303",
		"  v2.2.0 (Claude Code) ": "2.2.0",
		"10.0.1-beta.2 (x)":       "10.0.1",
	}
	f := newFake(t)
	for out, want := range cases {
		f.say("version", 0, out)

		got, err := Version(context.Background())

		if err != nil || got != want {
			t.Errorf("output %q: Version() = %q, %v, want %q", out, got, err, want)
		}
		if argv := f.argv(); !reflect.DeepEqual(argv, []string{"--version"}) {
			t.Errorf("claude was called with %v", argv)
		}
	}
}

func TestVersionRejectsOutputWithoutAVersion(t *testing.T) {
	f := newFake(t)
	for _, out := range []string{"", "claude", "unknown version\n", "(Claude Code) 2.1.303"} {
		f.say("version", 0, out)
		if got, err := Version(context.Background()); err == nil {
			t.Errorf("output %q: Version() = %q, want an error", out, got)
		}
	}
}

func TestVersionFailureCarriesTheFirstStderrLine(t *testing.T) {
	f := newFake(t)
	f.say("version", 3, "")
	f.complain("version", "broken install\nsecond line")

	_, err := Version(context.Background())

	if err == nil || !strings.Contains(err.Error(), "broken install") || strings.Contains(err.Error(), "second line") {
		t.Errorf("error = %v", err)
	}
}

func TestCompareVersionsIsNumeric(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.1.303", "2.1.301", 1},
		{"2.1.301", "2.1.301", 0},
		{"2.1.300", "2.1.301", -1},
		{"2.1.9", "2.1.10", -1},
		{"2.10.0", "2.9.9", 1},
		{"3.0.0", "2.99.99", 1},
		{"2.1", "2.1.0", 0},
		{"2.1", "2.1.1", -1},
		{"2.1.0.1", "2.1.0", 1},
	}
	for _, c := range cases {
		got, err := CompareVersions(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, %v, want %d", c.a, c.b, got, err, c.want)
		}
		// and the other way round
		back, err := CompareVersions(c.b, c.a)
		if err != nil || back != -c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, %v, want %d", c.b, c.a, back, err, -c.want)
		}
	}
}

func TestCompareVersionsRejectsWhatIsNotAVersion(t *testing.T) {
	for _, bad := range []string{"", "2.x.1", "2..1", ".1", "2.1.", "-2.1", "+2.1", "two", "2.1.1 (x)"} {
		if got, err := CompareVersions(bad, "2.1.301"); err == nil {
			t.Errorf("CompareVersions(%q, ...) = %d, want an error", bad, got)
		}
		if got, err := CompareVersions("2.1.301", bad); err == nil {
			t.Errorf("CompareVersions(..., %q) = %d, want an error", bad, got)
		}
	}
}

func TestMinVersion(t *testing.T) {
	if MinVersion != "2.1.301" {
		t.Errorf("MinVersion = %q, the plan says 2.1.301", MinVersion)
	}
	if got, err := CompareVersions(MinVersion, "2.1.300"); err != nil || got <= 0 {
		t.Errorf("MinVersion does not compare as newer than 2.1.300: %d, %v", got, err)
	}
}

func TestVersionStripsAuthVariablesButDoesNotSetTheConfigDir(t *testing.T) {
	f := newFake(t)
	setAuthEnv(t)
	f.say("version", 0, "2.1.303 (Claude Code)\n")

	if _, err := Version(context.Background()); err != nil {
		t.Fatal(err)
	}

	env := f.env()
	for _, n := range authEnvVars {
		if _, ok := env[n]; ok {
			t.Errorf("%s reached claude", n)
		}
	}
	// clalt does not point this call at a folder, so claude has no reason to make one
	if env["CLAUDE_CONFIG_DIR"] != "/the/shared/home" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want the ambient value left alone", env["CLAUDE_CONFIG_DIR"])
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	if _, err := Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v, ok := f.env()["CLAUDE_CONFIG_DIR"]; ok {
		t.Errorf("CLAUDE_CONFIG_DIR = %q was set for a version check", v)
	}
}
