package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/claudecli"
	"github.com/dhitalkamal/clalt/internal/config"
)

// testEnv is a clalt install inside a temp dir. HOME, XDG_CONFIG_HOME and
// CLAUDE_CONFIG_DIR all point into it, so nothing a test does can see the real
// ~/.claude or ~/.config/clalt.
type testEnv struct {
	t      *testing.T
	home   string
	app    *App
	claude *fakeClaude
	out    *bytes.Buffer
	errOut *bytes.Buffer
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	unsetAuthEnv(t)

	// a machine that has run claude has a claude home, and clalt will not make
	// one. the tests about a machine that has not remove it again
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	e := &testEnv{t: t, home: home, app: app, claude: installFakeClaude(t), out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	app.Out, app.Err, app.In = e.out, e.errOut, strings.NewReader("")
	return e
}

// run sends args through the real dispatcher and returns the exit code.
func (e *testEnv) run(args ...string) int {
	e.t.Helper()
	return run(e.app, args)
}

// reset forgets what the last command printed.
func (e *testEnv) reset() {
	e.out.Reset()
	e.errOut.Reset()
}

// unsetAuthEnv removes the variables clalt strips from the test's own
// environment, so a key in the developer's shell cannot change what a test sees
// or prints. setting them first makes the test framework put them back.
func unsetAuthEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if claudecli.IsAuthEnvVar(name) {
			t.Setenv(name, "")
			os.Unsetenv(name)
		}
	}
}

// theClock is the time every test that cares about dates runs at.
var theClock = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// at makes the app's clock read t.
func (e *testEnv) at(t time.Time) { e.app.now = func() time.Time { return t } }

// account is the stored account for a label, or a test failure.
func (e *testEnv) account(label string) account.Account {
	e.t.Helper()
	acc, found, err := e.app.Store.ResolveLabel(label)
	if err != nil || !found {
		e.t.Fatalf("no account labeled %q (found=%v, err=%v)", label, found, err)
	}
	return acc
}

func (e *testEnv) hasAccount(label string) bool {
	e.t.Helper()
	_, found, err := e.app.Store.ResolveLabel(label)
	if err != nil {
		e.t.Fatal(err)
	}
	return found
}

// folder is where an account's claude config lives.
func (e *testEnv) folder(label string) string {
	e.t.Helper()
	return accountDir(e.t, e.account(label).Name)
}

// accountDir is the folder of an account id, through the one check that every
// id goes through before it becomes a folder name.
func accountDir(t *testing.T, id string) string {
	t.Helper()
	dir, err := config.AccountDirFor(id)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// signIn runs `clalt login` for label with the fake signing in as email, and
// fails the test unless that works.
func (e *testEnv) signIn(label, email string) {
	e.t.Helper()
	e.claude.signInAs(email)
	if code := e.run("login", label); code != 0 {
		e.t.Fatalf("login %s exited %d: %s", label, code, e.errOut.String())
	}
	e.reset()
}

// seedShared puts a projects folder, a settings file and one mcp server into
// the shared claude home, the way a machine that has used claude looks.
func (e *testEnv) seedShared() (home string) {
	e.t.Helper()
	home = filepath.Join(e.home, ".claude")
	for path, body := range map[string]string{
		filepath.Join(home, "projects", "-p", "a.jsonl"): "{}\n",
		filepath.Join(home, "settings.json"):             `{"model":"opus"}`,
		filepath.Join(e.home, ".claude.json"):            `{"mcpServers":{"github":{"command":"gh-mcp"}}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			e.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	return home
}

// makeReadOnly stops new entries from being created in dir for the rest of the
// test. it reports false where that cannot work: windows has no such bits and
// root ignores them.
func makeReadOnly(t *testing.T, dir string) bool {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	return true
}

// plantState writes a state.json by hand, the way an older clalt left it.
func plantState(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(config.StatePath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.StatePath(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// answer is what the user types at the next prompts.
func (e *testEnv) answer(text string) {
	e.app.In = strings.NewReader(text)
	e.app.reader = nil
}

// skipWindows skips a test over a real difference of windows, which the reason
// names.
func skipWindows(t *testing.T, reason string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip(reason)
	}
}
