package claudecli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type fake struct {
	t   *testing.T
	dir string
}

// newFake installs the fake as the claude binary for the test. the real claude
// is never reachable from these tests: CLALT_CLAUDE_BIN wins over PATH.
func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLALT_CLAUDE_BIN", mustInstallFake(t, "fakeclaudecli", dir, "claude"))
	isolateHome(t, dir)
	return &fake{t: t, dir: dir}
}

// isolateHome points HOME, XDG_CONFIG_HOME and CLAUDE_CONFIG_DIR into dir. the
// folder checks look at the user's home and at clalt's own folder, which no test
// may touch, so every test that reaches them calls this first.
func isolateHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
}

func (f *fake) write(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// say sets what the fake prints for a command and the exit code it returns.
func (f *fake) say(name string, code int, out string) {
	f.t.Helper()
	f.write(name+".out", out)
	f.write(name+".code", strconv.Itoa(code))
}

func (f *fake) complain(name, text string) { f.t.Helper(); f.write(name+".err", text) }

func (f *fake) hang(mode string) { f.t.Helper(); f.write("hang", mode) }

// hangOn makes one command (status, login or logout) wait for a signal.
func (f *fake) hangOn(name, mode string) { f.t.Helper(); f.write(name+".hang", mode) }

func (f *fake) has(name string) bool {
	_, err := os.Stat(filepath.Join(f.dir, name))
	return err == nil
}

// calls returns the arguments of every call so far, one string per call.
func (f *fake) calls() []string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "calls"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// ran counts the calls that started with prefix.
func (f *fake) ran(prefix string) int {
	f.t.Helper()
	n := 0
	for _, c := range f.calls() {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fake) argv() []string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "argv"))
	if err != nil {
		f.t.Fatalf("the fake was never called: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func (f *fake) env() map[string]string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "env"))
	if err != nil {
		f.t.Fatalf("the fake was never called: %v", err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}

// envOf is the environment of the last call of one command (status, login,
// logout or version), where env is that of the last call of any.
func (f *fake) envOf(name string) map[string]string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "env."+name))
	if err != nil {
		f.t.Fatalf("no %s call was recorded: %v", name, err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}

// setAuthEnv fills the shell with every variable clalt must strip, plus one it
// must leave alone, and returns the secret each one carries.
func setAuthEnv(t *testing.T) map[string]string {
	t.Helper()
	secrets := map[string]string{}
	for _, n := range authEnvVars {
		secrets[n] = "secret-for-" + n
		t.Setenv(n, secrets[n])
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "/the/shared/home")
	t.Setenv("CLALT_TEST_KEEP", "kept")
	return secrets
}

// assertCleanEnv checks what the fake saw: no auth variable, the account
// folder as the config dir, everything else passed through.
func (f *fake) assertCleanEnv(configDir string) {
	f.t.Helper()
	env := f.env()
	for _, n := range authEnvVars {
		if v, ok := env[n]; ok {
			f.t.Errorf("%s reached claude (value length %d)", n, len(v))
		}
	}
	if env["CLAUDE_CONFIG_DIR"] != configDir {
		f.t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", env["CLAUDE_CONFIG_DIR"], configDir)
	}
	if env["CLALT_TEST_KEEP"] != "kept" {
		f.t.Error("an unrelated variable was dropped")
	}
}
