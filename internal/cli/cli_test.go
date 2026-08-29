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

func TestNextInChain(t *testing.T) {
	chain := []string{"a", "b", "c"}
	if got := nextInChain(chain, "a"); got != "b" {
		t.Errorf("after a = %q, want b", got)
	}
	if got := nextInChain(chain, "c"); got != "a" { // wraps
		t.Errorf("after c = %q, want a (wrap)", got)
	}
	if got := nextInChain(chain, "zzz"); got != "a" { // not in chain -> first
		t.Errorf("unknown active = %q, want a", got)
	}
}

func TestCmdNextRotates(t *testing.T) {
	app, _, _ := newTestApp(t)
	_ = app.Live.Write(liveCred("w"))
	_ = app.cmdAdd([]string{"work"})
	_ = app.Live.Write(liveCred("p"))
	_ = app.cmdAdd([]string{"personal"}) // personal now active
	_ = app.Store.SetFallback([]string{"work", "personal"})

	if err := app.cmdNext(nil); err != nil { // non-live round robin
		t.Fatalf("cmdNext: %v", err)
	}
	// active was personal (last in chain) -> wraps to work
	if act, _ := app.Store.Active(); act != "work" {
		t.Errorf("active = %q, want work after rotate", act)
	}
}

func TestOverBudget(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	projDir := filepath.Join(dir, "projects", "p")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// today's usage: 100K sonnet output ~= $1.50
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	line := `{"type":"assistant","timestamp":"` + now + `","sessionId":"s","message":{"model":"claude-sonnet-5","usage":{"input_tokens":0,"output_tokens":100000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(projDir, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	_ = app.Store.SetBudget("work", 0.50, 0) // cap below the ~$1.50 spent

	over, budget, cost := app.overBudget("work")
	if !over {
		t.Errorf("expected over budget: cost %.2f vs cap %.2f", cost, budget)
	}
	// no budget set -> never over
	_ = app.Store.SetBudget("work", 0, 0)
	if over, _, _ := app.overBudget("work"); over {
		t.Error("no cap should never be over budget")
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

// TestStageForRunActivatesLiveOnMacOS proves the fix: on macOS `run` puts the
// account's credential into the live store (what claude reads), so a session
// does not prompt for a fresh login.
func TestStageForRunActivatesLiveOnMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific run credential path")
	}
	app, _, dir := newTestApp(t)
	t.Setenv("XDG_CONFIG_HOME", dir)
	// login-equivalent: credential in the vault, live store empty
	if err := app.Store.Save("personal", liveCred("tok-personal"), account.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := app.stageForRun("personal", config.ProfileDir("personal"), false); err != nil {
		t.Fatalf("stageForRun: %v", err)
	}
	live, err := app.Live.Read()
	if err != nil {
		t.Fatalf("live read: %v", err)
	}
	if live.AccessToken != "tok-personal" {
		t.Errorf("live store not activated to personal: %q", live.AccessToken)
	}
}

// newTestApp builds an App backed by a fake keychain and temp dirs so command
// flow can be exercised without touching the real Keychain or Claude Code login.
func newTestApp(t *testing.T) (*App, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	kc := keychain.NewFake()
	live := livestore.New(kc, filepath.Join(dir, ".credentials.json"))
	store := account.New(kc, live, filepath.Join(dir, "state.json"), filepath.Join(dir, "switches.jsonl"))
	out := &bytes.Buffer{}
	app := &App{
		Store: store, Live: live, KC: kc, OAuth: oauth.NewClient(),
		Cache: usage.NewCache(filepath.Join(dir, "cache")),
		// ApiBase empty -> no network profile/live lookups during tests
		Out: out, Err: out,
	}
	return app, out, dir
}

func TestDoctorReportsSigning(t *testing.T) {
	app, out, _ := newTestApp(t)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"work"})
	out.Reset()
	if err := app.cmdDoctor(nil); err != nil {
		t.Fatalf("cmdDoctor: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "code signing:") {
		t.Fatalf("doctor output missing code signing line:\n%s", s)
	}
	// the value must be one of the known kinds, never blank ("n/a" off macOS).
	known := []string{"stable", "adhoc", "unsigned", "unknown", "n/a"}
	ok := false
	for _, k := range known {
		if strings.Contains(s, "code signing:     "+k) {
			ok = true
		}
	}
	if !ok {
		t.Errorf("doctor code signing value not a known kind:\n%s", s)
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

func TestCmdSwitchWriteBackThroughCLI(t *testing.T) {
	app, _, _ := newTestApp(t)
	// two accounts; work is active and live
	_ = app.Live.Write(liveCred("work-old"))
	_ = app.cmdAdd([]string{"work"})
	_ = app.Store.Save("personal", liveCred("personal-tok"), account.Meta{})

	// Claude Code rotates work's token while it is live
	_ = app.Live.Write(liveCred("work-rotated"))

	if err := app.cmdSwitch([]string{"personal"}); err != nil {
		t.Fatalf("cmdSwitch: %v", err)
	}
	// work slot captured the rotated token
	wc, _, _ := app.Store.Get("work")
	if wc.AccessToken != "work-rotated" {
		t.Errorf("work slot = %q, want work-rotated", wc.AccessToken)
	}
	// live now holds personal
	lc, _ := app.Live.Read()
	if lc.AccessToken != "personal-tok" {
		t.Errorf("live = %q, want personal-tok", lc.AccessToken)
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
