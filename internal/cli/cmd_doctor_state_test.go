package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// a state.json that cannot be read is a failed check of its own. the checks that
// need no accounts still run, since they may say what else is wrong.
func TestDoctorReportsAnUnreadableStateFileAndStillRunsTheOtherChecks(t *testing.T) {
	for name, body := range map[string]string{"corrupt": "{not json", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			plantState(t, body)

			code, rep := doctor(t, e)

			rep.want(t, "state", "file", "fail", "state.json")
			var seen int
			for _, c := range rep.Checks {
				if c.Scope == "state" && c.Check == "file" {
					seen++
				}
			}
			if seen != 1 {
				t.Errorf("%d state checks, want the one that failed", seen)
			}
			rep.want(t, "claude", "version", "ok", "")
			rep.want(t, "shell", "authEnv", "ok", "")
			rep.want(t, "settings", "authSettings", "ok", "")
			rep.want(t, "leftover", "oldVault", "ok", "")
			if code != 1 || rep.Failed != 1 {
				t.Errorf("exit %d, %d failed, want 1 and 1: %+v", code, rep.Failed, rep.Checks)
			}
			if raw, _ := os.ReadFile(config.StatePath()); string(raw) != body {
				t.Errorf("doctor changed state.json to %q", raw)
			}
		})
	}
}

func TestDoctorTextOutputShowsTheStateProblemToo(t *testing.T) {
	e := newEnv(t)
	plantState(t, "{not json")

	e.reset()
	code := e.run("doctor")

	out := e.out.String()
	if code != 1 || !strings.Contains(out, "fail  state:") || !strings.Contains(out, "1 check failed") {
		t.Errorf("exit %d:\n%s\nstderr: %s", code, out, e.errOut.String())
	}
}

// a passing check is a line too: it says the account list was looked at, and
// that is what an empty result of "no failures" cannot say
func TestDoctorSaysThatTheStateFileCanBeRead(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")

	code, rep := doctor(t, e)

	rep.want(t, "state", "file", "ok", config.StatePath())
	rep.want(t, "state", "file", "ok", "can be read")
	if code != 0 {
		t.Errorf("exit %d, %+v", code, rep.Checks)
	}
	// it comes before what it makes possible, the checks of each account
	at := map[string]int{}
	for i, c := range rep.Checks {
		at[c.Scope+"/"+c.Check] = i
	}
	if at["state/file"] > at["work/folder"] {
		t.Errorf("the state check comes after the account checks: %+v", rep.Checks)
	}
}

// before the first sign-in there is no file, which is not a problem
func TestDoctorSaysThereIsNoAccountListYetBeforeTheFirstSignIn(t *testing.T) {
	e := newEnv(t)

	code, rep := doctor(t, e)

	rep.want(t, "state", "file", "ok", "no account list yet")
	if code != 0 {
		t.Errorf("exit %d, %+v", code, rep.Checks)
	}
}
