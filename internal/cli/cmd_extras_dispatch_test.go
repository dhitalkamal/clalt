package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
	"github.com/dhitalkamal/clalt/internal/runcmd"
	"github.com/dhitalkamal/clalt/internal/usage"
)

func TestTheSessionCommandsAreInTheCommandTable(t *testing.T) {
	for _, name := range []string{"resume", "launch", "jump", "notify"} {
		e := newEnv(t)

		e.run(name)

		if strings.Contains(e.errOut.String(), "unknown command") {
			t.Errorf("%s is not in the command table", name)
		}
	}
}

func TestHelpListsTheSessionCommands(t *testing.T) {
	e := newEnv(t)

	e.run("help")

	out := e.out.String()
	for _, want := range []string{"clalt resume", "clalt launch <name>", "clalt jump <session>", "clalt notify"} {
		if !strings.Contains(out, want) {
			t.Errorf("the help is missing %q:\n%s", want, out)
		}
	}
}

func TestSessionDepsDefaultToTheRealOnes(t *testing.T) {
	// newEnv points HOME and XDG_CONFIG_HOME at a temp dir, so the real
	// registry and attribution adapters never see the developer's sessions
	newEnv(t)
	a := &App{}
	d := a.deps()

	// in the tests the runner is the refusing one, see session_deps_test.go
	if _, ok := productionRunner().(runcmd.Exec); !ok {
		t.Errorf("the runner of the real deps is %T, want runcmd.Exec", productionRunner())
	}
	t.Setenv("CLALT_TEST_PROBE", "probe")
	if got := d.env("CLALT_TEST_PROBE"); got != "probe" {
		t.Errorf("env lookup gave %q", got)
	}
	if exe, err := d.exe(); err != nil || exe == "" {
		t.Errorf("exe = %q, %v", exe, err)
	}
	if running, err := d.sessions(); err != nil || len(running) != 0 {
		t.Errorf("sessions = %v, %v, want none in an empty home", running, err)
	}
	if id, ok := d.lastAccount("any"); ok || id != "" {
		t.Errorf("lastAccount = %q, %v, want unknown in an empty home", id, ok)
	}

	// one record and one log line in the temp home prove the adapters are real
	addRecord(t, registry.Record{Handle: "dddd111122223333", Account: "work", ClaudeSessionID: "sess-1"})
	line := usage.SessionAccount{Ts: time.Now(), SessionID: "sess-1", Account: "work"}
	if err := usage.AppendSessionAccount(config.SessionAccountsPath(), line); err != nil {
		t.Fatal(err)
	}
	if running, err := d.sessions(); err != nil || len(running) != 1 || running[0].Handle != "dddd111122223333" {
		t.Errorf("sessions = %v, %v, want the planted record", running, err)
	}
	if id, ok := d.lastAccount("sess-1"); !ok || id != "work" {
		t.Errorf("lastAccount = %q, %v, want work", id, ok)
	}
	if a.deps() != d {
		t.Error("the deps are made again on every call")
	}
}
