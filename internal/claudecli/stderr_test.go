package claudecli

import (
	"context"
	"strings"
	"testing"
)

const noisyStderr = "\n  \n" +
	"(node:4242) [DEP0040] DeprecationWarning: The `punycode` module is deprecated.\n" +
	"(Use `node --trace-deprecation ...` to show where the warning was created)\n" +
	"Warning: ~/.claude.json was corrupt, a copy is in ~/.claude.json.corrupted.17\n" +
	"a second line\n"

func TestMeaningfulLineSkipsBlanksAndRuntimeNoise(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"nothing", "", ""},
		{"only blanks", " \n\t\n", ""},
		{"only noise", "(node:1) [DEP0040] DeprecationWarning: x\n(Use `node --trace-deprecation ...` to show)\n", ""},
		{"noise first", noisyStderr, "Warning: ~/.claude.json was corrupt, a copy is in ~/.claude.json.corrupted.17"},
		{"trimmed", "   spaced out   \nnext", "spaced out"},
		{"a deprecation warning that is not from node", "Bun DeprecationWarning: x\nreal\n", "real"},
		{"another node warning", "(node:99) ExperimentalWarning: Fetch is experimental\nreal\n", "real"},
		{"node's hint line alone", "(Use `node --trace-warnings ...` to show where it was created)\nreal\n", "real"},
	}
	for _, c := range cases {
		if got := meaningfulLine([]byte(c.in)); got != c.want {
			t.Errorf("%s: meaningfulLine = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMeaningfulLineIsCutAtAValidBoundary(t *testing.T) {
	long := strings.Repeat("a", 199) + "\u00e9\u00e9"
	got := meaningfulLine([]byte(long))
	if len(got) > 200 || !strings.HasPrefix(long, got) {
		t.Errorf("len %d, want at most 200 and a prefix of the line", len(got))
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Error("the cut split a character")
		}
	}
}

func TestStatusKeepsStderrAsAWarningInBothStates(t *testing.T) {
	f := newFake(t)
	f.complain("status", noisyStderr)
	want := "Warning: ~/.claude.json was corrupt, a copy is in ~/.claude.json.corrupted.17"
	for _, c := range []struct {
		code int
		out  string
	}{{0, signedInJSON}, {1, signedOutJSON}} {
		f.say("status", c.code, c.out)

		st, err := Status(context.Background(), t.TempDir())

		if err != nil || st.Warning != want {
			t.Errorf("exit %d: Warning = %q (%v), want %q", c.code, st.Warning, err, want)
		}
	}
	f.complain("status", "")
	if st, err := Status(context.Background(), t.TempDir()); err != nil || st.Warning != "" {
		t.Errorf("without stderr: Warning = %q (%v)", st.Warning, err)
	}
}

func TestFailuresQuoteTheFirstMeaningfulStderrLine(t *testing.T) {
	f := newFake(t)
	f.say("status", 2, "")
	f.complain("status", noisyStderr)

	_, err := Status(context.Background(), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "corrupt") || strings.Contains(err.Error(), "DeprecationWarning") {
		t.Errorf("err = %v", err)
	}
}

func TestACallEndedByASignalNamesTheSignal(t *testing.T) {
	f := newFake(t)
	f.write("status.die", "KILL")

	_, err := Status(context.Background(), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "signal") || !strings.Contains(err.Error(), "killed") || strings.Contains(err.Error(), "-1") {
		t.Errorf("err = %v, want one that names the signal and not exit -1", err)
	}
	f.write("logout.die", "TERM")
	if err := Logout(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "terminated") {
		t.Errorf("Logout err = %v, want the signal named", err)
	}
	f.write("version.die", "KILL")
	if _, err := Version(context.Background()); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("Version err = %v, want the signal named", err)
	}
}
