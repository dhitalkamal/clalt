package office

import (
	"os"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/registry"
)

func entry(kind, ide, sid string) registry.Record {
	return registry.Record{Handle: runH, ClaudeSessionID: sid, Terminal: registry.Terminal{Kind: kind, IDE: ide}}
}

func TestActionsForASessionInKittyWithAnIdCanDoAll(t *testing.T) {
	a := actionsFor(entry("kitty", "", "sid"), "darwin")
	if !a.Move || !a.Focus || !a.Stop || a.Why != "" {
		t.Errorf("= %+v", a)
	}
	if b := actionsFor(entry("tmux", "", "sid"), "linux"); !b.Focus {
		t.Errorf("tmux = %+v", b)
	}
}

func TestActionsForASessionWithoutAClaudeIdCannotMoveYet(t *testing.T) {
	a := actionsFor(entry("kitty", "", ""), "darwin")
	if a.Move || !a.Focus || !a.Stop || !strings.Contains(a.Why, "session id") {
		t.Errorf("= %+v", a)
	}
}

func TestActionsForAnIdeOrPlainTerminalCannotFocus(t *testing.T) {
	for _, r := range []registry.Record{entry("ide", "vscode", "sid"), entry("terminal", "", "sid"), entry("", "", "sid")} {
		a := actionsFor(r, "darwin")
		if a.Focus || !a.Move || !a.Stop || !strings.Contains(a.Why, "front") {
			t.Errorf("%q = %+v", r.Terminal.Kind, a)
		}
	}
}

func TestActionsOnWindowsCannotMoveOrStop(t *testing.T) {
	a := actionsFor(entry("tmux", "", "sid"), "windows")
	if a.Move || a.Stop || !a.Focus || !strings.Contains(a.Why, "windows") {
		t.Errorf("= %+v", a)
	}
}

func TestRealSessionsCarryTheActionsOfTheirRecord(t *testing.T) {
	f := newFixture(t)
	f.record(runHandle, "work", "/p/x", os.Getpid())
	snap := f.build()
	if len(snap.Sessions) != 1 {
		t.Fatalf("%d sessions", len(snap.Sessions))
	}
	a := snap.Sessions[0].Actions
	// the fixture record has no terminal and no claude id
	if a.Focus || a.Move || a.Why == "" {
		t.Errorf("actions = %+v", a)
	}
}
