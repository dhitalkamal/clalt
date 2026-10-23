package crew

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func start(t *testing.T, w *world, id string) (StartResult, error) {
	t.Helper()
	return w.manager().Start(context.Background(), StartRequest{Account: id, Folder: w.folder(id)})
}

func TestStartRunsAGatewayForTheAccountAndRecordsIt(t *testing.T) {
	w := newWorld(t)
	folder := w.folder("work")
	res, err := w.manager().Start(context.Background(), StartRequest{Account: "work", Folder: folder})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := PathsFor("work")
	want := Record{
		Account: "work", PID: 4242, Port: 5500, Home: p.Home, Version: "0.9.0",
		StartedAt: time.Date(2026, 10, 23, 15, 5, 12, 0, time.UTC), ProcStart: psStart,
	}
	if res.Record != want || res.Already || res.Warning != "" {
		t.Errorf("result = %+v, want the record %+v", res, want)
	}
	if got, found, _ := ReadRecord("work"); !found || got != want {
		t.Errorf("the record on disk is %+v", got)
	}
	if len(w.started) != 1 {
		t.Fatalf("%d gateways were started", len(w.started))
	}
	s := w.started[0]
	if s.Bin != "/fake/kirocrew" || !slices.Equal(s.Args, []string{"gateway", "--port", "5500", "--no-open", "--no-tunnel"}) || s.Dir != p.Home {
		t.Errorf("started %s %v in %s", s.Bin, s.Args, s.Dir)
	}
	if w.started[0].Env == nil || valueOf(t, s.Env, "CLAUDE_CONFIG_DIR") != folder || valueOf(t, s.Env, "KIROCREW_PORT") != "5500" {
		t.Errorf("environment = %v", s.Env)
	}
}

func TestAGatewayIsNeverStartedWithoutTheFlagsThatKeepItQuiet(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	if len(w.started) != 1 {
		t.Fatalf("%d gateways were started", len(w.started))
	}
	args := w.started[0].Args
	for _, want := range []string{"--no-open", "--no-tunnel"} {
		if !slices.Contains(args, want) {
			t.Errorf("%s is missing from %v", want, args)
		}
	}
	for _, never := range []string{"--json-ready", "--approval", "--test-mode", "yolo"} {
		if slices.Contains(args, never) {
			t.Errorf("%s was passed: %v", never, args)
		}
	}
}

func TestEachGatewayGetsItsOwnHomeAndItsOwnPort(t *testing.T) {
	w := newWorld(t)
	for _, id := range []string{"work", "personal"} {
		if _, err := start(t, w, id); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.started) != 2 {
		t.Fatalf("%d gateways were started", len(w.started))
	}
	a, b := w.started[0], w.started[1]
	if valueOf(t, a.Env, "KIROCREW_HOME") == valueOf(t, b.Env, "KIROCREW_HOME") || valueOf(t, a.Env, "KIRO_HOME") == valueOf(t, b.Env, "KIRO_HOME") {
		t.Error("two gateways share a home")
	}
	if recordedPortOf(t, "work") == recordedPortOf(t, "personal") {
		t.Error("two gateways share a port")
	}
}

func TestThePortSkipsTheOnesInOtherRecordsAndTheBusyOnes(t *testing.T) {
	w := newWorld(t)
	if err := WriteRecord("other", Record{PID: 99, Port: 5500}); err != nil {
		t.Fatal(err)
	}
	m := w.manager()
	m.o.Busy = func(p int) bool { return p == 5501 }
	if _, err := m.Start(context.Background(), StartRequest{Account: "work", Folder: w.folder("work")}); err != nil {
		t.Fatal(err)
	}
	if got := recordedPortOf(t, "work"); got != 5502 {
		t.Errorf("port = %d, want 5502", got)
	}
}

func TestTheVersionIsCheckedBeforeAnythingIsSetOrStarted(t *testing.T) {
	w := newWorld(t)
	w.version = "kirocrew 0.10.0\n"
	_, err := start(t, w, "work")
	if err == nil || !strings.Contains(err.Error(), "0.10.0") || !strings.Contains(err.Error(), "0.9.x") {
		t.Fatalf("err = %v", err)
	}
	if len(w.started) != 0 {
		t.Error("a gateway was started")
	}
	for _, c := range w.calls {
		if slices.Contains(c.Args, "config") {
			t.Error("the config was touched before the version was accepted")
		}
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("a record was written")
	}
}

func TestKirocrewThatCannotBeRunIsAClearError(t *testing.T) {
	w := newWorld(t)
	m := w.manager()
	m.o.Binary = func() (string, error) { return "", os.ErrNotExist }
	if _, err := m.Start(context.Background(), StartRequest{Account: "work", Folder: w.folder("work")}); err == nil {
		t.Fatal("no error")
	}
	if len(w.started) != 0 || len(w.calls) != 0 {
		t.Error("something ran")
	}
}

func readPins(t *testing.T) map[string]any {
	t.Helper()
	p, _ := PathsFor("work")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readFile(p.LocalConfig)), &cfg); err != nil {
		t.Fatalf("config.local.json: %v", err)
	}
	return cfg
}

func TestTheDefaultsAreWrittenToTheLocalConfigAndKirocrewConfigIsNotRun(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	for _, c := range w.calls {
		if slices.Contains(c.Args, "config") {
			t.Errorf("kirocrew config was run: %v", c.Args)
		}
	}
	agent, _ := readPins(t)["agent"].(map[string]any)
	roles, _ := agent["role_models"].(map[string]any)
	if agent["acp_backend"] != "claude" || agent["member_acp_backend"] != "claude" || agent["approval_mode"] != "interactive" ||
		roles["background"] != "haiku" || roles["subagent"] != "haiku" || len(roles) != 2 {
		t.Errorf("agent = %v", agent)
	}
	p, _ := PathsFor("work")
	// windows has no unix permission bits
	if fi, err := os.Stat(p.LocalConfig); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Errorf("config.local.json: %v, %v", fi, err)
	}
	if left, _ := filepath.Glob(filepath.Join(p.Home, ".tmp-*")); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestPinningKeepsEveryOtherKeyOfAnExistingLocalConfig(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	before := `{"slack":{"allowed_users":["u1"]},"agent":{"model":"opus","acp_backend":"kas","role_models":{"background":"opus","other":"x"}},"timezone":"UTC"}`
	if err := os.WriteFile(p.LocalConfig, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	cfg := readPins(t)
	agent := cfg["agent"].(map[string]any)
	roles := agent["role_models"].(map[string]any)
	if agent["model"] != "opus" || agent["acp_backend"] != "claude" || roles["background"] != "haiku" || roles["other"] != "x" || cfg["timezone"] != "UTC" {
		t.Errorf("cfg = %v", cfg)
	}
	if slack, _ := cfg["slack"].(map[string]any); slack["allowed_users"] == nil {
		t.Errorf("an unknown section was lost: %v", cfg)
	}
}

func TestALocalConfigThatIsNotJsonStopsTheStartAndIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.LocalConfig, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := start(t, w, "work"); err == nil || !strings.Contains(err.Error(), p.LocalConfig) {
		t.Errorf("err = %v", err)
	}
	if len(w.started) != 0 || readFile(p.LocalConfig) != "{broken" {
		t.Error("a gateway was started or the file changed")
	}
}

func TestTheHomeIsMadePrivateAndTheAccountFolderIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	folder := w.folder("work")
	if _, err := w.manager().Start(context.Background(), StartRequest{Account: "work", Folder: folder}); err != nil {
		t.Fatal(err)
	}
	p, _ := PathsFor("work")
	// windows has no unix permission bits
	if fi, err := os.Stat(p.Home); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700) {
		t.Errorf("home: %v, %v", fi, err)
	}
	if entries, _ := os.ReadDir(folder); len(entries) != 0 {
		t.Errorf("something was written into the account folder: %v", entries)
	}
}

func TestStartNamesWhatItDidNotPassOnAndNeverTheValues(t *testing.T) {
	w := newWorld(t)
	res, err := start(t, w, "work")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Removed, "ANTHROPIC_API_KEY") || !slices.Contains(res.Removed, "KIROCREW_CC_PERMISSION_MODE") {
		t.Errorf("Removed = %v", res.Removed)
	}
	for _, name := range res.Removed {
		if strings.Contains(name, "=") {
			t.Errorf("a value in %q", name)
		}
	}
}

func TestTheLogOfTheRunBeforeIsKept(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Log, []byte("the last run\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(p.PrevLog); got != "the last run\n" {
		t.Errorf("the previous log = %q", got)
	}
	if len(w.logs) != 1 || w.logs[0] != p.Log {
		t.Errorf("the gateway was told to log to %v, want %s", w.logs, p.Log)
	}
}

func TestInstalledVersionAsksKirocrewWithAHomeThatIsNotTheUsers(t *testing.T) {
	w := newWorld(t)
	v, err := w.manager().InstalledVersion(context.Background())
	if err != nil || v != "0.9.0" {
		t.Fatalf("v = %q, %v", v, err)
	}
	env := w.calls[0].Env
	if got := valueOf(t, env, "KIROCREW_HOME"); !strings.HasPrefix(got, Root()) {
		t.Errorf("KIROCREW_HOME = %q, want one inside %s", got, Root())
	}
	w.version = "garbage\n"
	if _, err := w.manager().InstalledVersion(context.Background()); err == nil {
		t.Error("an unreadable version was accepted")
	}
}
