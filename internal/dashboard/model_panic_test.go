package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// an adapter bug must show up as a message, not end the program

type panicSource struct{ value any }

func (p panicSource) Load(context.Context) (Snapshot, error) { panic(p.value) }

type panicActions struct {
	fakeActions
	value any
}

func (p *panicActions) Run(context.Context, string) (Launch, error)       { panic(p.value) }
func (p *panicActions) SignIn(context.Context, string) (Launch, error)    { panic(p.value) }
func (p *panicActions) Rename(context.Context, string, string) error      { panic(p.value) }
func (p *panicActions) StopSession(context.Context, string, string) error { panic(p.value) }

func TestAPanicInLoadBecomesAnError(t *testing.T) {
	m := newModel(context.Background(), panicSource{"disk on fire"}, &fakeActions{}, testOptions())
	l, ok := RunCmd(m.loadCmd(), 2*time.Second).(loadedMsg)
	if !ok || l.err == nil || l.err.Error() != "load panicked: disk on fire" {
		t.Fatalf("the panic is the error: %#v", l)
	}
	shown, _ := upd(m, l)
	shown, _ = upd(shown, tea.WindowSizeMsg{Width: 100, Height: 30})
	if out := Screen(shown); !strings.Contains(out, "load panicked: disk on fire") {
		t.Errorf("the pane says what happened:\n%s", out)
	}
}

func TestAPanicWithAnErrorValueKeepsItsMessage(t *testing.T) {
	m := newModel(context.Background(), panicSource{errors.New("nil map")}, &fakeActions{}, testOptions())
	if l, _ := RunCmd(m.loadCmd(), 2*time.Second).(loadedMsg); l.err == nil || l.err.Error() != "load panicked: nil map" {
		t.Errorf("an error value reads as its text: %#v", l.err)
	}
}

func TestAPanicInAnActionIsShownAsAFailure(t *testing.T) {
	act := &panicActions{value: "bad state"}
	m, _ := actionModel(&act.fakeActions)
	m.act = act
	shown := pressSettle(m, "r", "x", "enter")
	if got := footerOf(shown); got != "  action panicked: bad state" || !shown.flash.err {
		t.Errorf("rename: %q (error %v)", got, shown.flash.err)
	}
	shown = pressSettle(m, "2", "x", "y")
	if got := footerOf(shown); got != "  action panicked: bad state" {
		t.Errorf("stop: %q", got)
	}
	if out := Screen(shown); !strings.Contains(out, "robin") {
		t.Error("the dashboard carries on")
	}
}

func TestAPanicWhileStartingRunOrSignInIsShownAsAFailure(t *testing.T) {
	act := &panicActions{value: "no profile"}
	m, _ := actionModel(&act.fakeActions)
	m.act = act
	for _, key := range []string{"enter", "s"} {
		if got := footerOf(pressSettle(m, key)); got != "  action panicked: no profile" {
			t.Errorf("%s: %q", key, got)
		}
	}
}
