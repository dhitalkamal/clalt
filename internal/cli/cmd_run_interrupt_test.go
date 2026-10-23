//go:build !windows

package cli

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// a headless -p run, a script and a supervisor have no terminal whose Ctrl-C
// reaches claude by itself, so a SIGINT sent to clalt alone (kill -INT) would be
// swallowed and claude would run on. it is passed on, and clalt ends with the
// code claude ends with, as it does for TERM and HUP
func TestWithoutATerminalAnInterruptSentToClaltIsPassedOnToClaude(t *testing.T) {
	r := startHeadlessRun(t)

	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	code, ended := r.ended(10 * time.Second)
	if got := r.signals(); got != "INT" {
		t.Errorf("claude noted %q, want INT. clalt ended: %v, exit %d: %s", got, ended, code, r.said())
	}
	if !ended || code != 130 {
		t.Errorf("clalt ended: %v, exit %d, want 130, claude's own: %s", ended, code, r.said())
	}
	if left := sessionRecordFiles(r.e); len(left) != 0 {
		t.Errorf("the session record was left behind: %v", left)
	}
}

// what decides is the input of clalt, which claude is given as it is. a reader
// that is no file reaches claude through a pipe, and a file that is no terminal
// is none either
func TestTheInputIsATerminalOnlyWhenItIsOne(t *testing.T) {
	e := newEnv(t)

	e.app.In = strings.NewReader("")
	if e.app.inputIsTerminal() {
		t.Error("a reader that is no file counts as a terminal")
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	e.app.In = null
	if e.app.inputIsTerminal() {
		t.Error("the null device counts as a terminal")
	}
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	e.app.In = file
	if e.app.inputIsTerminal() {
		t.Error("a plain file counts as a terminal")
	}
}

// the seam that the signal tests use: what it says wins over what the input is
func TestTheTerminalSeamOverridesWhatTheInputIs(t *testing.T) {
	e := newEnv(t)
	e.app.In = strings.NewReader("")

	e.app.terminal = func() bool { return true }
	if !e.app.inputIsTerminal() {
		t.Error("the seam says a terminal and the answer is no")
	}
	e.app.terminal = func() bool { return false }
	if e.app.inputIsTerminal() {
		t.Error("the seam says no terminal and the answer is yes")
	}
}

// the input is a pipe, but clalt is in the foreground group of a terminal: that
// terminal's Ctrl-C reaches claude by itself, and passing one on as well would
// give claude a second
func TestInAForegroundGroupAnInterruptIsNotPassedOnWhateverTheInputIs(t *testing.T) {
	r := startForegroundRun(t)

	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	if code, ended := r.ended(600 * time.Millisecond); ended {
		t.Fatalf("clalt ended with %d on the interrupt while claude was still running: %s", code, r.said())
	}
	r.release()
	if code, ended := r.ended(10 * time.Second); !ended || code != 0 {
		t.Errorf("after claude ended: ended %v, exit %d: %s", ended, code, r.said())
	}
	if got := r.signals(); got != "" {
		t.Errorf("claude was sent %q by clalt", got)
	}
}
