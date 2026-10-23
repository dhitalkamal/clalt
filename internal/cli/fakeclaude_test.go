package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClaudeVars are cleared at the start of every test, so a variable from the
// developer's shell cannot change an answer.
var fakeClaudeVars = []string{
	"FAKE_CLAUDE_EMAIL", "FAKE_CLAUDE_ORG", "FAKE_CLAUDE_PLAN", "FAKE_CLAUDE_READS_STDIN",
	"FAKE_CLAUDE_LOGIN_FAILS", "FAKE_CLAUDE_LOGIN_NOOP", "FAKE_CLAUDE_STATUS_BROKEN",
	"FAKE_CLAUDE_API_KEY_HELPER", "FAKE_CLAUDE_LOGOUT_FAILS", "FAKE_CLAUDE_VERSION", "FAKE_CLAUDE_EXIT",
	"FAKE_CLAUDE_SESSION_PRINTS", "FAKE_CLAUDE_STATUS_HANGS", "FAKE_CLAUDE_LOGOUT_HANGS",
	"FAKE_CLAUDE_STATUS_ERR", "FAKE_CLAUDE_LOGOUT_ERR", "FAKE_CLAUDE_NO_ORG",
	"FAKE_CLAUDE_LOGIN_WRITES_THEN_FAILS", "FAKE_CLAUDE_PROVIDER", "FAKE_CLAUDE_STATUS_FOLDER",
	"FAKE_CLAUDE_SESSION_HOLDS", "FAKE_CLAUDE_EXTRAS", "FAKE_CLAUDE_SIGNAL",
}

type fakeClaude struct {
	t   *testing.T
	dir string
}

// installFakeClaude makes the fake the claude binary for the test.
func installFakeClaude(t *testing.T) *fakeClaude {
	t.Helper()
	dir := t.TempDir()
	bin := mustInstallFake(t, "fakeclaude", dir, "claude")
	t.Setenv("CLALT_CLAUDE_BIN", bin)
	for _, v := range fakeClaudeVars {
		t.Setenv(v, "")
	}
	return &fakeClaude{t: t, dir: dir}
}

// signInAs is the email the next `auth login` signs in as.
func (f *fakeClaude) signInAs(email string) { f.t.Setenv("FAKE_CLAUDE_EMAIL", email) }

// set switches one of the FAKE_CLAUDE_* behaviors on for the rest of the test.
func (f *fakeClaude) set(name, value string) { f.t.Setenv("FAKE_CLAUDE_"+name, value) }

// plantLogin signs folder in without running a login, as claude would have
// left it.
func (f *fakeClaude) plantLogin(folder, email string) {
	f.t.Helper()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		f.t.Fatal(err)
	}
	body := email + "\nExample Org\nmax\n"
	if err := os.WriteFile(filepath.Join(folder, ".fake-login"), []byte(body), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// warnAbout makes every status call on folder write text to stderr first, the
// way claude does when it sets a corrupt .claude.json aside. the text is kept as
// given, so a test can put escape sequences in it.
func (f *fakeClaude) warnAbout(folder, text string) {
	f.t.Helper()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, ".fake-warn"), []byte(text+"\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// signOut drops the login of folder, as if claude had lost it.
func (f *fakeClaude) signOut(folder string) {
	f.t.Helper()
	if err := os.Remove(filepath.Join(folder, ".fake-login")); err != nil {
		f.t.Fatal(err)
	}
}

// signedIn reports whether the fake holds a login for folder.
func (f *fakeClaude) signedIn(folder string) bool {
	_, err := os.Stat(filepath.Join(folder, ".fake-login"))
	return err == nil
}

// calls returns the recorded auth calls of one kind (login, logout or status).
func (f *fakeClaude) calls(kind string) []string {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.dir, "calls"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.HasPrefix(line, kind+" ") {
			out = append(out, strings.TrimPrefix(line, kind+" "))
		}
	}
	return out
}

// callEnv is the environment the last auth call of one kind (login, status or
// logout) was started with.
func (f *fakeClaude) callEnv(kind string) map[string]string {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.dir, kind+".env"))
	if err != nil {
		f.t.Fatalf("no %s call was recorded: %v", kind, err)
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

// session returns what the last interactive session was started with.
func (f *fakeClaude) session() (argv []string, env map[string]string, cwd string) {
	f.t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(f.dir, name))
		if err != nil {
			f.t.Fatalf("no session was started: %v", err)
		}
		return string(b)
	}
	if a := strings.TrimSuffix(read("session.argv"), "\n"); a != "" {
		argv = strings.Split(a, "\n")
	}
	// the pair that clalt run puts first is not what the user asked claude for
	if len(argv) >= 2 && argv[0] == "--settings" && strings.HasPrefix(argv[1], `{"hooks"`) {
		if argv = argv[2:]; len(argv) == 0 {
			argv = nil
		}
	}
	env = map[string]string{}
	for _, line := range strings.Split(read("session.env"), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return argv, env, strings.TrimSpace(read("session.cwd"))
}

func (f *fakeClaude) sessionStarted() bool {
	_, err := os.Stat(filepath.Join(f.dir, "session.argv"))
	return err == nil
}

func (f *fakeClaude) stdinSeen() string {
	b, _ := os.ReadFile(filepath.Join(f.dir, "login.stdin"))
	return strings.TrimSpace(string(b))
}

// sessionStdin is what the last session read from its stdin, which the fake
// only does when it is set to print.
func (f *fakeClaude) sessionStdin() string {
	b, _ := os.ReadFile(filepath.Join(f.dir, "session.stdin"))
	return string(b)
}

// sessionRecords is what clalt had written to its sessions folder while the last
// session was running.
func (f *fakeClaude) sessionRecords() string {
	b, _ := os.ReadFile(filepath.Join(f.dir, "session.records"))
	return string(b)
}

// sessionSettings is the JSON that clalt run passed to the last session with
// --settings, or "" when it passed none.
func (f *fakeClaude) sessionSettings() string {
	b, err := os.ReadFile(filepath.Join(f.dir, "session.argv"))
	if err != nil {
		f.t.Fatalf("no session was started: %v", err)
	}
	argv := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(argv) >= 2 && argv[0] == "--settings" && strings.HasPrefix(argv[1], `{"hooks"`) {
		return argv[1]
	}
	return ""
}
