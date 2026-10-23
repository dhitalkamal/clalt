package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

func TestDoctorReportsTheOldVaultAndLeavesItAlone(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(config.VaultDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.VaultKeyPath(), []byte("not a real key"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, rep := doctor(t, e)

	rep.want(t, "leftover", "oldVault", "warn", config.VaultDir())
	rep.want(t, "leftover", "oldVault", "warn", config.VaultKeyPath())
	if code != 0 {
		t.Errorf("a leftover is a warning, exit %d", code)
	}
	if !exists(config.VaultDir()) || !exists(config.VaultKeyPath()) {
		t.Error("doctor deleted the vault")
	}
}

func TestDoctorPointsAtMigrateForOldProfilesWithHistoryNotCopiedYet(t *testing.T) {
	e := newEnv(t)
	old := filepath.Join(config.ProfilesDir(), "work", "projects", "-p")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "a.jsonl"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "b.jsonl"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "warn", "clalt migrate")
	rep.want(t, "leftover", "oldProfiles", "warn", "2 files")
	if exists(config.ClaudeProjectsDir()) {
		t.Error("the dry run copied something")
	}
}

// the folders are found without the shared history, so being inside a session
// does not stop doctor from saying that there are none
func TestDoctorHasNothingToSayAboutOldProfilesInsideARunSessionWhenThereAreNone(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	t.Setenv("CLAUDE_CONFIG_DIR", e.folder("work"))

	_, rep := doctor(t, e)

	rep.want(t, "leftover", "oldProfiles", "ok", "no old profile folders")
}

func TestOldKeepaliveFilesPerPlatform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	plist := filepath.Join(home, "Library", "LaunchAgents", "com.clalt.keepalive.plist")
	service := filepath.Join(home, ".config", "systemd", "user", "com.clalt.keepalive.service")
	timer := filepath.Join(home, ".config", "systemd", "user", "com.clalt.keepalive.timer")

	for _, goos := range []string{"darwin", "linux", "windows"} {
		if got := oldKeepaliveFiles(goos); len(got) != 0 {
			t.Errorf("%s: found %v in an empty home", goos, got)
		}
	}
	for _, p := range []string{plist, service, timer} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string][]string{
		"darwin":  {plist},
		"linux":   {service, timer},
		"windows": nil, // no file to look for: a scheduled task, which doctor does not query
		"plan9":   nil,
	}
	for goos, want := range cases {
		if got := oldKeepaliveFiles(goos); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: oldKeepaliveFiles = %v, want %v", goos, got, want)
		}
	}
}

func TestDoctorReportsAnOldKeepaliveJobWhereTheOSKeepsOne(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the old job only ever existed on macOS and linux")
	}
	e := newEnv(t)
	_, rep := doctor(t, e)
	rep.want(t, "leftover", "oldKeepalive", "ok", "")

	files := oldKeepaliveFilesIfPlanted(t, e.home)
	_, rep = doctor(t, e)

	for _, f := range files {
		rep.want(t, "leftover", "oldKeepalive", "warn", f)
	}
	for _, f := range files {
		if !exists(f) {
			t.Errorf("doctor deleted %s", f)
		}
	}
}

// oldKeepaliveFilesIfPlanted writes the files the old job left on this OS.
func oldKeepaliveFilesIfPlanted(t *testing.T, home string) []string {
	t.Helper()
	var files []string
	switch runtime.GOOS {
	case "darwin":
		files = []string{filepath.Join(home, "Library", "LaunchAgents", "com.clalt.keepalive.plist")}
	default:
		dir := filepath.Join(home, ".config", "systemd", "user")
		files = []string{filepath.Join(dir, "com.clalt.keepalive.service"), filepath.Join(dir, "com.clalt.keepalive.timer")}
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestOldCredentialsFileIsOnlyALeftoverOnMacOS(t *testing.T) {
	if !oldCredentialsMatter("darwin") || oldCredentialsMatter("linux") || oldCredentialsMatter("windows") {
		t.Error("on linux that file is claude's real login store, so it is never a leftover")
	}
}

func TestDoctorFlagsTheOldCredentialsFileOnMacOSWithoutReadingIt(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS has this check")
	}
	e := newEnv(t)
	_, rep := doctor(t, e)
	rep.want(t, "leftover", "oldCredentials", "ok", "")

	path := filepath.Join(e.home, ".claude", ".credentials.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"claudeAiOauth":{"accessToken":"tok-secret-value"}}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, rep = doctor(t, e)

	// the file cannot be read at all, so only its existence can have been checked
	rep.want(t, "leftover", "oldCredentials", "warn", path)
	if strings.Contains(e.out.String(), "tok-secret-value") {
		t.Error("the file's content leaked")
	}
}

// claude writes that file itself when it cannot save its login in the keychain,
// so on a Mac it may be the real login and not a leftover. the advice says how
// to tell and does not say to delete it outright. that is what doctor says
// when claude cannot answer for itself.
func TestTheOldCredentialsAdviceSaysHowToTellAndDoesNotSayToDeleteItBlindly(t *testing.T) {
	e := newEnv(t)
	e.claude.set("STATUS_BROKEN", "1")
	path := filepath.Join(e.home, ".claude", ".credentials.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := credentialsCheck()

	if c.Status != statusWarn || !strings.Contains(c.Detail, path) {
		t.Fatalf("status %s, detail %q", c.Status, c.Detail)
	}
	for _, want := range []string{"claude auth status", "keychain", "your real login", "not signed in"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("the advice does not mention %q: %q", want, c.Detail)
		}
	}
	if strings.Contains(c.Detail, "delete it if you do not use it") || strings.Contains(c.Detail, "nothing needs it") {
		t.Errorf("the advice still treats the file as a plain leftover: %q", c.Detail)
	}
}

func TestDoctorSaysNothingAboutCredentialsOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", ".credentials.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	if c, ok := rep.find("leftover", "oldCredentials"); ok {
		t.Errorf("claude's own login store was reported: %+v", c)
	}
}
