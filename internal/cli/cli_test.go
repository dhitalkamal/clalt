package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/credential"
	"github.com/dhitalkamal/clalt/internal/keychain"
	"github.com/dhitalkamal/clalt/internal/livestore"
	"github.com/dhitalkamal/clalt/internal/oauth"
	"github.com/dhitalkamal/clalt/internal/profile"
	"github.com/dhitalkamal/clalt/internal/usage"
)

func TestParseRunArgs(t *testing.T) {
	// --link is accepted as a no-op (sharing is now default); --isolated opts out
	acct, opts, cargs, err := parseRunArgs([]string{"work", "--refresh", "--isolated", "--link", "--", "--resume", "foo"})
	if err != nil {
		t.Fatalf("parseRunArgs: %v", err)
	}
	if acct != "work" || !opts.refresh || !opts.isolated {
		t.Errorf("acct=%q opts=%+v", acct, opts)
	}
	if len(cargs) != 2 || cargs[0] != "--resume" || cargs[1] != "foo" {
		t.Errorf("claude args = %v", cargs)
	}
	// args before -- that follow the account also pass through
	_, _, cargs, _ = parseRunArgs([]string{"work", "--model", "opus"})
	if len(cargs) != 2 || cargs[0] != "--model" {
		t.Errorf("passthrough = %v", cargs)
	}
	if _, _, _, err := parseRunArgs(nil); err == nil {
		t.Error("empty args should error")
	}
}

func TestStageProfileCredStagesCredential(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir) // ProfileDir hangs off XDG
	_ = app.Live.Write(liveCred("tok-work"))
	_ = app.cmdAdd([]string{"work"})

	profDir := config.ProfileDir("work")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := app.stageProfileCred("work", profDir, false); err != nil {
		t.Fatalf("stageProfileCred: %v", err)
	}
	if !profile.HasCredential(profDir) {
		t.Fatal("profile dir should hold a staged credential")
	}
	got, ok, _ := profile.Read(profDir)
	if !ok || got.AccessToken != "tok-work" {
		t.Errorf("staged credential wrong: ok=%v %+v", ok, got)
	}
}

// TestDarwinRunEnvInjectsTokenNoKeychain proves the keychain-free fix: on macOS
// `run` resolves the account token from the vault and hands it to the child via
// CLAUDE_CODE_OAUTH_TOKEN, and does NOT write Claude Code's live store (the
// macOS Keychain item that used to trigger the recurring password prompt).
func TestDarwinRunEnvInjectsTokenNoKeychain(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific run credential path")
	}
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)
	// login-equivalent: credential in the vault, live store empty
	if err := app.Store.Save("personal", liveCred("tok-personal"), account.Meta{}); err != nil {
		t.Fatal(err)
	}
	env, err := app.darwinRunEnv("personal", false)
	if err != nil {
		t.Fatalf("darwinRunEnv: %v", err)
	}
	want := "CLAUDE_CODE_OAUTH_TOKEN=tok-personal"
	found := false
	for _, e := range env {
		if e == want {
			found = true
		}
	}
	if !found {
		t.Errorf("run env missing token injection %q, got %v", want, env)
	}
	// the live store (keychain-backed on macOS) must be left untouched
	if live, rerr := app.Live.Read(); rerr == nil && live.AccessToken != "" {
		t.Errorf("run must not write the live store, but found %q", live.AccessToken)
	}
}

// newTestApp builds an App backed by a fake keychain and temp dirs so command
// flow can be exercised without touching the real Keychain or Claude Code login.
func newTestApp(t *testing.T) (*App, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	kc := keychain.NewFake()
	live := livestore.New(kc, filepath.Join(dir, ".credentials.json"))
	store := account.New(kc, filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl"))
	out := &bytes.Buffer{}
	app := &App{
		Store: store, Live: live, KC: kc, OAuth: oauth.NewClient(),
		Cache: usage.NewCache(filepath.Join(dir, "cache")),
		// ApiBase empty -> no network profile/live lookups during tests
		Out: out, Err: out,
	}
	return app, out, dir
}

// TestDoctorIsKeychainFree proves doctor no longer diagnoses the macOS Keychain:
// it reports the account state without any keychain-sync or code-signing lines
// (both were only relevant to the old keychain-backed live store).
func TestDoctorIsKeychainFree(t *testing.T) {
	app, out, _ := newTestApp(t)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	out.Reset()
	if err := app.cmdDoctor(nil); err != nil {
		t.Fatalf("cmdDoctor: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "active account:") {
		t.Errorf("doctor output missing active account line:\n%s", s)
	}
	for _, gone := range []string{"code signing", "keychain", "live store sync"} {
		if strings.Contains(strings.ToLower(s), gone) {
			t.Errorf("doctor should be keychain-free but still mentions %q:\n%s", gone, s)
		}
	}
}

func liveCred(access string) credential.Credential {
	return credential.Credential{
		AccessToken: access, RefreshToken: "r-" + access,
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(), SubscriptionType: "max",
	}
}

func TestCmdAddThenList(t *testing.T) {
	app, out, _ := newTestApp(t)
	// simulate a current Claude Code login
	if err := app.Live.Write(liveCred("tok-work")); err != nil {
		t.Fatal(err)
	}
	if err := app.cmdAdd([]string{"work"}); err != nil {
		t.Fatalf("cmdAdd: %v", err)
	}
	out.Reset()
	if err := app.cmdList(); err != nil {
		t.Fatalf("cmdList: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "work") || !strings.Contains(s, "*") {
		t.Errorf("list output missing active work account:\n%s", s)
	}
}

func TestCmdListJSON(t *testing.T) {
	app, out, _ := newTestApp(t)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	app.JSON = true
	out.Reset()
	if err := app.cmdList(); err != nil {
		t.Fatal(err)
	}
	var views []accountView
	if err := json.Unmarshal(out.Bytes(), &views); err != nil {
		t.Fatalf("list --json not valid JSON: %v\n%s", err, out.String())
	}
	if len(views) != 1 || views[0].Name != "work" || !views[0].Active {
		t.Errorf("unexpected views: %+v", views)
	}
}

func TestCmdRemoveActiveGuardThroughCLI(t *testing.T) {
	app, _, _ := newTestApp(t)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	// removing the active account must fail (guard); --force only skips the prompt
	if err := app.cmdRemove([]string{"work", "--force"}); err == nil {
		t.Error("removing active account should error even with --force")
	}
}

func TestCmdUsageJSONEmpty(t *testing.T) {
	app, out, dir := newTestApp(t)
	// point Claude projects dir at an empty temp dir via env
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	app.JSON = true
	out.Reset()
	if err := app.cmdUsage(nil); err != nil {
		t.Fatalf("cmdUsage: %v", err)
	}
	var reports []accountReport
	if err := json.Unmarshal(out.Bytes(), &reports); err != nil {
		t.Fatalf("usage --json invalid: %v\n%s", err, out.String())
	}
	if len(reports) != 1 || reports[0].Tokens != 0 {
		t.Errorf("expected 1 report with 0 tokens, got %+v", reports)
	}
}

func TestUnattributedUsageFoldsIntoActive(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	// seed a transcript with usage but NO switch history yet
	projDir := filepath.Join(dir, "projects", "proj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","timestamp":"2026-08-24T10:00:00Z","sessionId":"s1","message":{"model":"claude-sonnet-5","usage":{"input_tokens":100,"output_tokens":200}}}` + "\n"
	if err := os.WriteFile(filepath.Join(projDir, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"main"}) // becomes active, no switch events

	reports, err := app.buildReports(false, "")
	if err != nil {
		t.Fatalf("buildReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d reports", len(reports))
	}
	// pre-switch usage should surface under the active account, not vanish
	if reports[0].Tokens != 300 {
		t.Errorf("active account tokens = %d, want 300 (unattributed folded in)", reports[0].Tokens)
	}
}

func TestExportImportRoundTripThroughCLI(t *testing.T) {
	app, _, dir := newTestApp(t)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	_ = app.Store.Save("personal", liveCred("p"), account.Meta{Email: "p@x.com"})

	file := filepath.Join(dir, "backup.clalt")
	app.In = strings.NewReader("hunter2\nhunter2\n") // passphrase + confirm
	if err := app.cmdExport([]string{file}); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}

	// wipe and reimport into a fresh app
	app2, _, _ := newTestApp(t)
	app2.In = strings.NewReader("hunter2\n")
	if err := app2.cmdImport([]string{file}); err != nil {
		t.Fatalf("cmdImport: %v", err)
	}
	if _, _, err := app2.Store.Get("personal"); err != nil {
		t.Errorf("personal not restored: %v", err)
	}
	pc, _, _ := app2.Store.Get("personal")
	if pc.AccessToken != "p" {
		t.Errorf("restored token = %q, want p", pc.AccessToken)
	}
}
