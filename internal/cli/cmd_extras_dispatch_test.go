package cli

import (
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/runcmd"
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
		t.Errorf("sessions = %v, %v, want none until step 3a is wired in", running, err)
	}
	if id, ok := d.lastAccount("any"); ok || id != "" {
		t.Errorf("lastAccount = %q, %v, want unknown until step 3a is wired in", id, ok)
	}
	if a.deps() != d {
		t.Error("the deps are made again on every call")
	}
}
