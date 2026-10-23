package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// authCalls makes every auth call against dir and returns their errors by name.
func authCalls(dir string) map[string]error {
	ctx := context.Background()
	_, statusErr := Status(ctx, dir)
	_, loginErr := Login(ctx, dir, "")
	return map[string]error{"Status": statusErr, "Login": loginErr, "Logout": Logout(ctx, dir)}
}

// callAll adds a session to the auth calls: every call that points claude at
// dir. the session is the one that runs claude for hours in that folder, and the
// only one that is not held back by the version of claude.
func callAll(dir string) map[string]error {
	errs := authCalls(dir)
	_, errs["Session"] = Session(context.Background(), dir, nil)
	return errs
}

func TestCallsRefuseAFolderThatCannotBeUsed(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	f.say("logout", 0, "")
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"empty":    "",
		"relative": filepath.Join("relative", "dir"),
		"missing":  filepath.Join(t.TempDir(), "not-made"),
		"a file":   file,
	}
	for name, dir := range cases {
		for call, err := range callAll(dir) {
			if err == nil {
				t.Errorf("%s folder: %s accepted it", name, call)
			}
		}
	}
	// claude makes the folder it is given, so it must never be started for these
	if f.has("argv") {
		t.Error("claude was run for a folder that was refused")
	}
}

func TestCallsRefuseARelativeFolderThatExists(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	f.say("logout", 0, "")
	// "." exists, so only the check for an absolute path can refuse it
	t.Chdir(t.TempDir())
	for call, err := range callAll(".") {
		if err == nil || !strings.Contains(err.Error(), "absolute") {
			t.Errorf("%s(.) = %v, want a refusal that asks for an absolute path", call, err)
		}
	}
	if f.has("argv") {
		t.Error("claude was run for a relative folder")
	}
}

func TestCallsRefuseTheSharedHome(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(home, ".claude")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "via-link")
	if err := os.Symlink(shared, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, dir := range []string{shared, link} {
		for call, err := range callAll(dir) {
			if err == nil || !strings.Contains(err.Error(), "shared claude home") {
				t.Errorf("%s(%s) = %v, want a refusal that names the shared home", call, dir, err)
			}
		}
	}
	if f.has("argv") {
		t.Error("claude was run against the shared home")
	}

	// when CLAUDE_CONFIG_DIR points elsewhere, that folder is the shared home
	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	for call, err := range callAll(custom) {
		if err == nil {
			t.Errorf("%s accepted the custom shared home", call)
		}
	}
}

func TestCallsAcceptAnAccountFolder(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	xdg := filepath.Join(t.TempDir(), "xdg")
	dir := filepath.Join(xdg, "clalt", "accounts", "robin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	// a clalt run session has CLAUDE_CONFIG_DIR set to the account folder itself
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	if _, err := Status(context.Background(), dir); err != nil {
		t.Errorf("Status refused an account folder: %v", err)
	}
	if _, err := Session(context.Background(), dir, nil); err != nil {
		t.Errorf("Session refused an account folder: %v", err)
	}
}
