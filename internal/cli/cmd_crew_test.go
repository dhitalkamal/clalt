package cli

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/crew"
)

func TestCrewStartRunsAGatewayWithTheAccountsFolderAndNoStrayLogin(t *testing.T) {
	e, k := crewEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-never-shown")
	if code := e.run("crew", "start", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if out := e.out.String(); !strings.Contains(out, "http://127.0.0.1:5500") || !strings.Contains(out, "work") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(e.errOut.String(), "ANTHROPIC_API_KEY") || strings.Contains(e.out.String()+e.errOut.String(), "sk-ant-never-shown") {
		t.Errorf("stderr = %q", e.errOut.String())
	}
	env := k.gatewayEnv()
	if env["CLAUDE_CONFIG_DIR"] != e.folder("work") || env["KIROCREW_PORT"] != "5500" {
		t.Errorf("gateway env: %v", env)
	}
	if _, bad := env["ANTHROPIC_API_KEY"]; bad {
		t.Error("the api key reached the gateway")
	}
	if rec, found, _ := crew.ReadRecord(e.account("work").Name); !found || rec.Port != 5500 {
		t.Errorf("record = %+v", rec)
	}
}

func TestCrewStartRefusesAccountsThatCannotRunAGateway(t *testing.T) {
	e, k := crewEnv(t)
	e.signIn("other", "o@example.com")
	e.claude.signOut(e.folder("other"))
	if code := e.run("crew", "remove-me"); code != 2 {
		t.Errorf("an unknown crew command exited %d", code)
	}
	e.reset()
	e.run("remove", "--force", "work")
	e.reset()
	for _, name := range []string{"nobody", "work", "other"} {
		if code := e.run("crew", "start", name); code != 1 {
			t.Errorf("%s: exit %d, want 1", name, code)
		}
	}
	if _, err := os.Stat(k.dir + "/gateway.env"); err == nil {
		t.Error("a gateway was started for an account that could not run one")
	}
}

func TestCrewStartWithAKirocrewOutsideTheRangeStartsNothing(t *testing.T) {
	e, k := crewEnv(t)
	t.Setenv("FAKE_KIROCREW_VERSION", "0.10.0")
	if code := e.run("crew", "start", "work"); code != 1 || !strings.Contains(e.errOut.String(), "0.10.0") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if _, err := os.Stat(k.dir + "/gateway.env"); err == nil {
		t.Error("a gateway was started")
	}
}

func TestCrewStatusStopAndTheRecord(t *testing.T) {
	e, _ := crewEnv(t)
	e.run("crew", "start", "work")
	e.reset()
	if code := e.run("crew", "status", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("%v: %s", err, e.out.String())
	}
	r := rows[0]
	if r["account"] != "work" || r["state"] != "running" || r["port"] != float64(5500) || r["url"] != "http://127.0.0.1:5500" {
		t.Errorf("row = %v", r)
	}
	if strings.Contains(e.out.String(), "token") {
		t.Error("status mentions a token")
	}
	e.reset()
	e.run("crew", "status")
	if out := e.out.String(); !strings.Contains(out, "running") || !strings.Contains(out, "5500") {
		t.Errorf("status = %q", out)
	}

	e.reset()
	if code := e.run("crew", "stop", "work"); code != 0 || !strings.Contains(e.out.String(), "stopped") {
		t.Fatalf("exit %d: %s%s", code, e.out.String(), e.errOut.String())
	}
	if _, found, _ := crew.ReadRecord(e.account("work").Name); found {
		t.Error("the record is still there")
	}
	e.reset()
	e.run("crew", "status", "work")
	if !strings.Contains(e.out.String(), "stopped") {
		t.Errorf("status = %q", e.out.String())
	}
}

func TestCrewStatusWithNoGatewaysSaysSo(t *testing.T) {
	e, _ := crewEnv(t)
	if code := e.run("crew", "status"); code != 0 || !strings.Contains(e.out.String(), "no gateways") {
		t.Errorf("exit %d: %s", code, e.out.String())
	}
}

func TestCrewStartTwiceKeepsTheSameGateway(t *testing.T) {
	e, _ := crewEnv(t)
	e.run("crew", "start", "work")
	first, _, _ := crew.ReadRecord(e.account("work").Name)
	e.reset()
	if code := e.run("crew", "start", "work"); code != 0 || !strings.Contains(e.out.String(), "already running") {
		t.Fatalf("exit %d: %s", code, e.out.String())
	}
	if again, _, _ := crew.ReadRecord(e.account("work").Name); again.PID != first.PID {
		t.Error("a second gateway replaced the first")
	}
}

func TestCrewStopOfAGatewayThatDiedOnlyClearsItsRecord(t *testing.T) {
	e, k := crewEnv(t)
	e.run("crew", "start", "work")
	_ = os.WriteFile(k.dir+"/release", nil, 0o644)
	rec, _, _ := crew.ReadRecord(e.account("work").Name)
	waitGone(t, rec.PID)
	e.reset()
	if code := e.run("crew", "stop", "work"); code != 0 || !strings.Contains(e.out.String(), "not running") {
		t.Errorf("exit %d: %s%s", code, e.out.String(), e.errOut.String())
	}
}

func waitGone(t *testing.T, pid int) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		p, err := crew.Probe(t.Context(), crew.ExecRunner{}, os.Environ(), pid)
		if err == nil && !p.Alive {
			return
		}
	}
	t.Fatalf("pid %d did not end", pid)
}

func TestCrewLogsShowTheEndOfTheOutputWithoutTheToken(t *testing.T) {
	e, _ := crewEnv(t)
	e.run("crew", "start", "work")
	var out string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		e.reset()
		e.run("crew", "logs", "work", "-n", "5")
		if out = e.out.String(); strings.Contains(out, "gateway up") {
			break
		}
	}
	if !strings.Contains(out, "gateway up") || strings.Contains(out, "abc123secret") || !strings.Contains(out, "token=<hidden>") {
		t.Errorf("logs = %q", out)
	}
	e.reset()
	if code := e.run("crew", "logs", "nobody"); code != 1 {
		t.Errorf("logs of an unknown account exited %d", code)
	}
}

func TestCrewUsageErrorsAndHelp(t *testing.T) {
	e := newEnv(t)
	if runtime.GOOS == "windows" {
		// clalt crew answers "not available" there before it reads any argument
		e.run("help")
		if !strings.Contains(e.out.String(), "clalt crew start <name>") {
			t.Error("help lacks the crew commands")
		}
		return
	}
	for _, args := range [][]string{{"crew"}, {"crew", "start"}, {"crew", "stop", "a", "b"}, {"crew", "logs", "a", "-n", "x"}, {"crew", "status", "--bogus"}} {
		if code := e.run(args...); code != 2 {
			t.Errorf("%v exited %d: %s", args, code, e.errOut.String())
		}
		e.reset()
	}
	e.run("help")
	for _, want := range []string{"clalt crew start <name>", "clalt crew stop <name>", "clalt crew status", "clalt crew logs <name>"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("help lacks %q", want)
		}
	}
}

func TestTestsCannotReachARealKirocrewByDefault(t *testing.T) {
	if _, err := crew.Binary(); err == nil {
		t.Error("kirocrew resolves in a test that installed no fake")
	}
}
