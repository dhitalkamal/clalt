package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeClaudeScript stands in for claude. a login is a file inside
// CLAUDE_CONFIG_DIR, which is how the real one keys it to the folder, and the
// FAKE_CLAUDE_* variables steer what it does. it records what it was asked next
// to itself: one line per auth call in calls, and for a session the arguments,
// the environment and the folder it ran in. FAKE_CLAUDE_SESSION_HOLDS makes a
// session wait until a session.release file appears, and note in signals every
// TERM, HUP or INT it gets, ending with the code a shell would report.
const fakeClaudeScript = `#!/bin/sh
dir=$(dirname "$0")
cfg="$CLAUDE_CONFIG_DIR"
login="$cfg/.fake-login"
case "$1 $2" in
  "auth login")
    echo "login $cfg|$*" >> "$dir/calls"
    env | sort > "$dir/login.env"
    if [ -n "$FAKE_CLAUDE_READS_STDIN" ]; then read -r line; echo "$line" > "$dir/login.stdin"; fi
    if [ -n "$FAKE_CLAUDE_LOGIN_FAILS" ]; then echo "login was cancelled" >&2; exit 1; fi
    if [ -n "$FAKE_CLAUDE_LOGIN_NOOP" ]; then exit 0; fi
    if [ -n "$FAKE_CLAUDE_LOGIN_WRITES_THEN_FAILS" ]; then
      mkdir -p "$cfg"
      printf '%s\n%s\n%s\n' "$FAKE_CLAUDE_EMAIL" "${FAKE_CLAUDE_ORG:-Example Org}" "${FAKE_CLAUDE_PLAN:-max}" > "$login"
      echo "could not save the profile: network error" >&2
      exit 1
    fi
    mkdir -p "$cfg"
    org="${FAKE_CLAUDE_ORG:-Example Org}"
    if [ -n "$FAKE_CLAUDE_NO_ORG" ]; then org=""; fi
    printf '%s\n%s\n%s\n' "$FAKE_CLAUDE_EMAIL" "$org" "${FAKE_CLAUDE_PLAN:-max}" > "$login"
    echo "Login successful"
    exit 0 ;;
  "auth status")
    echo "status $cfg" >> "$dir/calls"
    env | sort > "$dir/status.env"
    if [ -f "$cfg/.fake-warn" ]; then cat "$cfg/.fake-warn" >&2; fi
    if [ -n "$FAKE_CLAUDE_STATUS_HANGS" ]; then exec sleep 10; fi
    if [ -n "$FAKE_CLAUDE_STATUS_BROKEN" ]; then printf '%b\n' "${FAKE_CLAUDE_STATUS_ERR:-keychain is locked}" >&2; exit 3; fi
    if [ -n "$FAKE_CLAUDE_API_KEY_HELPER" ]; then
      printf '{"loggedIn":true,"authMethod":"api_key_helper","configDirectory":"%s"}\n' "$cfg"
      exit 0
    fi
    if [ -f "$login" ]; then
      { read -r email; read -r org; read -r plan; } < "$login"
      printf '{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"%s","email":"%s","orgId":"org-1","orgName":"%s","subscriptionType":"%s","configDirectory":"%s","projectsDirectory":"%s/projects"}\n' "${FAKE_CLAUDE_PROVIDER:-firstParty}" "$email" "$org" "$plan" "${FAKE_CLAUDE_STATUS_FOLDER:-$cfg}" "$cfg"
      exit 0
    fi
    printf '{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty","configDirectory":"%s"}\n' "$cfg"
    exit 1 ;;
  "auth logout")
    echo "logout $cfg" >> "$dir/calls"
    env | sort > "$dir/logout.env"
    if [ -n "$FAKE_CLAUDE_LOGOUT_HANGS" ]; then exec sleep 10; fi
    if [ -n "$FAKE_CLAUDE_LOGOUT_FAILS" ]; then printf '%b\n' "${FAKE_CLAUDE_LOGOUT_ERR:-could not reach the keychain}" >&2; exit 1; fi
    rm -f "$login"
    exit 0 ;;
esac
if [ "$1" = "--version" ]; then echo "${FAKE_CLAUDE_VERSION:-2.1.303} (Claude Code)"; exit 0; fi
printf '%s\n' "$@" > "$dir/session.argv"
env | sort > "$dir/session.env"
pwd > "$dir/session.cwd"
cat "$XDG_CONFIG_HOME"/clalt/sessions/*.json > "$dir/session.records" 2>/dev/null
if [ -n "$FAKE_CLAUDE_SESSION_HOLDS" ]; then
  trap 'echo TERM >> "$dir/signals"; exit 143' TERM
  trap 'echo HUP >> "$dir/signals"; exit 129' HUP
  trap 'echo INT >> "$dir/signals"; exit 130' INT
  echo $$ > "$dir/session.pid"
  echo holding > "$dir/session.holding"
  while [ ! -f "$dir/session.release" ]; do sleep 0.05; done
fi
if [ -n "$FAKE_CLAUDE_SESSION_PRINTS" ]; then echo "hello from claude"; cat > "$dir/session.stdin"; fi
if [ "$FAKE_CLAUDE_EXIT" = "term" ]; then kill -TERM $$; fi
exit "${FAKE_CLAUDE_EXIT:-0}"
`

// fakeClaudeVars are cleared at the start of every test, so a variable from the
// developer's shell cannot change an answer.
var fakeClaudeVars = []string{
	"FAKE_CLAUDE_EMAIL", "FAKE_CLAUDE_ORG", "FAKE_CLAUDE_PLAN", "FAKE_CLAUDE_READS_STDIN",
	"FAKE_CLAUDE_LOGIN_FAILS", "FAKE_CLAUDE_LOGIN_NOOP", "FAKE_CLAUDE_STATUS_BROKEN",
	"FAKE_CLAUDE_API_KEY_HELPER", "FAKE_CLAUDE_LOGOUT_FAILS", "FAKE_CLAUDE_VERSION", "FAKE_CLAUDE_EXIT",
	"FAKE_CLAUDE_SESSION_PRINTS", "FAKE_CLAUDE_STATUS_HANGS", "FAKE_CLAUDE_LOGOUT_HANGS",
	"FAKE_CLAUDE_STATUS_ERR", "FAKE_CLAUDE_LOGOUT_ERR", "FAKE_CLAUDE_NO_ORG",
	"FAKE_CLAUDE_LOGIN_WRITES_THEN_FAILS", "FAKE_CLAUDE_PROVIDER", "FAKE_CLAUDE_STATUS_FOLDER",
	"FAKE_CLAUDE_SESSION_HOLDS",
}

type fakeClaude struct {
	t   *testing.T
	dir string
}

// installFakeClaude makes the fake the claude binary for the test.
func installFakeClaude(t *testing.T) *fakeClaude {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("these tests use sh scripts as the claude binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatal(err)
	}
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
