package dashboard_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dhitalkamal/clalt/internal/dashboard"
	"github.com/dhitalkamal/clalt/internal/dashboard/demo"
)

var (
	press    = dashboard.Press
	typeText = dashboard.TypeText
	screen   = dashboard.Screen
	raw      = dashboard.Raw
	quits    = dashboard.Quits
	settle   = dashboard.Settle
)

var viewNames = []string{"Overview", "Sessions", "Limits", "History", "Profile"}

var accountNames = []string{"robin", "morgan", "casey", "jordan", "harriet"}

// clock stands still until told otherwise, so the header age is the same in
// every run
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// demoModel is the dashboard on the demo data, loaded, four seconds after the
// load like the approved frames
func demoModel(opt dashboard.Options, w, h int) tea.Model {
	c := &clock{t: demo.Epoch}
	opt.Now = c.now
	m := dashboard.Boot(dashboard.New(context.Background(), demo.Source(c.now), demo.Actions(), opt), w, h)
	c.advance(4 * time.Second)
	return m
}

type config struct{ ascii, noColor bool }

func sized(cfg config, w, h int) tea.Model {
	return demoModel(dashboard.Options{ASCII: cfg.ascii, NoColor: cfg.noColor}, w, h)
}

func lineWith(t *testing.T, out, want string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, want) {
			return l
		}
	}
	t.Fatalf("no line contains %q in:\n%s", want, out)
	return ""
}

func rightTitle(out string) string { return strings.Split(out, "\n")[1] }

// checkFrame fails when a frame has the wrong number of lines or a line that
// is wider than the terminal.
func checkFrame(t *testing.T, label, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != h {
		t.Errorf("%s: %d lines, want %d", label, len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got > w {
			t.Errorf("%s: line %d is %d wide, terminal is %d: %q", label, i, got, w, l)
			return
		}
	}
}

// walk visits the model in every account, view, history tab and pane focus
// combination at one size. Models are values, so branching from a shared
// parent is safe.
func walk(cfg config, w, h int, visit func(label string, m tea.Model)) {
	base := sized(cfg, w, h)
	for ai, name := range accountNames {
		am := base
		for i := 0; i < ai; i++ {
			am, _ = press(am, "down")
		}
		for _, key := range []string{"1", "2", "3", "4", "5"} {
			vm, _ := press(am, key)
			tabs := 1
			if key == "4" {
				tabs = 3
			}
			for tab := 0; tab < tabs; tab++ {
				sm := vm
				if tab > 0 {
					sm, _ = press(sm, "tab")
					for j := 0; j < tab; j++ {
						sm, _ = press(sm, "l")
					}
					sm, _ = press(sm, "tab")
				}
				for _, right := range []bool{false, true} {
					fm := sm
					if right {
						fm, _ = press(fm, "tab")
					}
					visit(fmt.Sprintf("%s view %s tab %d right=%v %dx%d", name, key, tab, right, w, h), fm)
				}
			}
		}
	}
}

type nilSource struct{}

func (nilSource) Load(context.Context) (dashboard.Snapshot, error) { return dashboard.Snapshot{}, nil }
