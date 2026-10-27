package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// a report that could not be written was lost, and a script that reads it has to
// be told: exit 1, and the reason. it is a different failure from a command that
// went wrong, and it must not look like success.
type brokenWriter struct{ after int }

func (w *brokenWriter) Write(p []byte) (int, error) {
	if w.after > 0 {
		w.after--
		return len(p), nil
	}
	return 0, errors.New("disk full")
}

func TestAReportThatCannotBeWrittenExitsOne(t *testing.T) {
	for _, args := range [][]string{
		{"doctor"}, {"doctor", "--json"}, {"usage"}, {"usage", "--json"}, {"sessions"}, {"sessions", "--json"},
		{"whoami"}, {"whoami", "--json"}, {"list"}, {"list", "--json"}, {"budget"}, {"budget", "--json"},
		{"version"}, {"help"}, {}, {"rename", "work", "office"}, {"logout", "work"},
		{"migrate"}, {"migrate", "--json"},
	} {
		name := strings.Join(args, " ")
		if name == "" {
			name = "no command"
		}
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.signIn("work", "w@example.com")
			e.app.Out = &brokenWriter{}
			e.errOut.Reset()

			code := e.run(args...)

			msg := e.errOut.String()
			if code != 1 || !strings.Contains(msg, "the output could not be written") || !strings.Contains(msg, "disk full") {
				t.Errorf("exit %d, stderr %q", code, msg)
			}
		})
	}
}

// the part that was written before the failure does not make the rest fine
func TestAReportThatFailsHalfWayExitsOneToo(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.app.Out = &brokenWriter{after: 2}
	e.errOut.Reset()

	code := e.run("doctor")

	if code != 1 || !strings.Contains(e.errOut.String(), "the output could not be written") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

// a question that never reached the user is a lost write like any other, even
// when the answer came from a script and everything after it was written fine
type failsOn struct {
	marker string
	out    io.Writer
}

func (w failsOn) Write(p []byte) (int, error) {
	if strings.Contains(string(p), w.marker) {
		return 0, errors.New("disk full")
	}
	return w.out.Write(p)
}

func TestAPromptThatCannotBeWrittenExitsOne(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.app.Out = failsOn{marker: "[y/N]", out: e.out}
	e.answer("n\n")
	e.errOut.Reset()

	code := e.run("remove", "work")

	if code != 1 || !strings.Contains(e.errOut.String(), "the output could not be written") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

// the state of a run does not leak into the next: a failed write is about one
// command
func TestAWriteErrorIsForgottenByTheNextCommand(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.app.Out = &brokenWriter{}
	if code := e.run("list"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}

	e.app.Out = e.out
	e.reset()

	if code := e.run("list"); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// a session is claude's output and goes to claude's terminal, not through the
// writer that the reports use
func TestASessionKeepsTheRealOutput(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("SESSION_PRINTS", "1")

	if code := e.run("run", "work"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if !strings.Contains(e.out.String(), "hello from claude") {
		t.Errorf("stdout = %q", e.out.String())
	}
}

// a command that failed for its own reason says that, and exits the way it did:
// the failed write does not take its place
func TestACommandThatFailedKeepsItsOwnErrorWhenTheOutputBreaksToo(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.claude.set("LOGOUT_FAILS", "1")
	e.app.Out = failsOn{marker: "[y/N]", out: e.out}
	e.answer("y\n")
	e.errOut.Reset()

	code := e.run("remove", "work")

	msg := e.errOut.String()
	if code != 1 || !strings.Contains(msg, "work") || strings.Contains(msg, "the output could not be written") {
		t.Errorf("exit %d, stderr %q: want the refusal of the sign-out and not the lost question", code, msg)
	}
}

// doctor ends with a bare exit code when a check failed, and the report is all
// it says. when that report is lost the code alone would be a mystery, so the
// lost output is named too, and the code stays
func TestADoctorWhoseReportIsLostStillSaysWhyAndKeepsItsCode(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(e.home, "no-such-claude"))
	e.app.Out = &brokenWriter{}
	e.errOut.Reset()

	code := e.run("doctor")

	if code != 1 || !strings.Contains(e.errOut.String(), "the output could not be written") {
		t.Errorf("exit %d, stderr %q", code, e.errOut.String())
	}
}

// finish keeps a session's exit code when the output broke as well, and says so
func TestFinishKeepsABareExitCodeAndNamesTheLostOutput(t *testing.T) {
	e := newEnv(t)
	e.app.writeErr = errors.New("disk full")

	err := e.app.finish(exitError{3})

	var exit exitError
	if !errors.As(err, &exit) || exit.code != 3 {
		t.Errorf("finish = %v, want the exit code 3", err)
	}
	if !strings.Contains(e.errOut.String(), "the output could not be written: disk full") {
		t.Errorf("stderr %q", e.errOut.String())
	}
}

// a write error that came back wrapped is still the write error, not a second
// failure of the command
func TestFinishRecognisesAWrappedWriteError(t *testing.T) {
	e := newEnv(t)
	disk := errors.New("disk full")
	e.app.writeErr = disk

	err := e.app.finish(fmt.Errorf("render the table: %w", disk))

	if err == nil || !strings.Contains(err.Error(), "the output could not be written") {
		t.Errorf("finish = %v", err)
	}
}
