package dashboard_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// glyphs by code point so the source stays ascii
var (
	fullCell   = string(rune(0x2588))
	trackCell  = string(rune(0x2591))
	roundedTop = string(rune(0x256d))
)

var basicColorRe = regexp.MustCompile(`^(3[0-7]|9[0-7])$`)

func inSet(p string, set ...string) bool {
	for _, s := range set {
		if p == s {
			return true
		}
	}
	return false
}

// textAttrs are the non color parameters the design uses
var textAttrs = []string{"", "0", "1", "2", "4", "22", "24"}

func isSixteenColor(p string) bool {
	return inSet(p, textAttrs...) || p == "39" || basicColorRe.MatchString(p)
}

func TestColorsStayInTheSixteenColorPalette(t *testing.T) {
	for _, sz := range [][2]int{{100, 30}, {80, 24}} {
		walk(config{}, sz[0], sz[1], func(label string, m tea.Model) {
			for _, p := range dashboard.SGRParams(raw(m)) {
				if !isSixteenColor(p) {
					t.Errorf("%s: SGR parameter %q is outside the 16 color palette", label, p)
					return
				}
			}
		})
	}
	base := sized(config{}, 100, 30)
	help, _ := press(base, "?")
	budget, _ := press(base, "e")
	stop, _ := press(base, "2", "x")
	for name, m := range map[string]tea.Model{"help": help, "budget": budget, "stop": stop} {
		for _, p := range dashboard.SGRParams(raw(m)) {
			if !isSixteenColor(p) {
				t.Errorf("%s: SGR parameter %q is outside the 16 color palette", name, p)
			}
		}
	}
}

func TestNoColorEnvDropsEveryColor(t *testing.T) {
	sawAttr := false
	check := func(label string, m tea.Model) {
		for _, p := range dashboard.SGRParams(raw(m)) {
			if !inSet(p, textAttrs...) {
				t.Errorf("%s: NO_COLOR output has color parameter %q", label, p)
				return
			}
			if p == "1" || p == "2" || p == "4" {
				sawAttr = true
			}
		}
	}
	walk(config{noColor: true}, 100, 30, check)
	base := sized(config{noColor: true}, 100, 30)
	help, _ := press(base, "?")
	check("help", help)
	if !sawAttr {
		t.Error("NO_COLOR should still keep bold, dim and underline for hierarchy")
	}
	// the environment variable reaches the options the same way
	opt := dashboard.EnvOptions(func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	})
	for _, p := range dashboard.SGRParams(raw(demoModel(opt, 100, 30))) {
		if !inSet(p, textAttrs...) {
			t.Errorf("a model built from the environment emits color parameter %q", p)
		}
	}
}

func TestUsageBarsAreGreenYellowRedOverATrack(t *testing.T) {
	robin := raw(sized(config{}, 100, 30))
	if !strings.Contains(robin, "\x1b[32m"+fullCell) {
		t.Error("26% and 42% bars should fill green")
	}
	if !strings.Contains(robin, "\x1b[90m"+trackCell) {
		t.Error("the empty track should be bright black")
	}
	casey, _ := press(sized(config{}, 100, 30), "j", "j")
	out := raw(casey)
	if !strings.Contains(out, "\x1b[31m"+fullCell) {
		t.Error("the 95% bar should fill red")
	}
	if !strings.Contains(out, "\x1b[33m"+fullCell) {
		t.Error("the 80% bar should fill yellow")
	}
}

func TestSessionStatesAreColored(t *testing.T) {
	m, _ := press(sized(config{}, 100, 30), "2")
	out := raw(m)
	if !strings.Contains(out, "\x1b[32mworking") {
		t.Error("working should be green")
	}
	if !strings.Contains(out, "\x1b[33mwaiting") {
		t.Error("waiting should be yellow")
	}
}

func TestChromeIsQuiet(t *testing.T) {
	m := sized(config{}, 100, 30)
	check := func(name string, m tea.Model, leftFocused bool) {
		top := strings.Split(raw(m), "\n")[1]
		first, second := strings.Index(top, roundedTop), strings.LastIndex(top, roundedTop)
		leftDim := strings.HasSuffix(top[:first], "\x1b[90m")
		rightDim := strings.HasSuffix(top[:second], "\x1b[90m")
		if leftDim == leftFocused || rightDim != leftFocused {
			t.Errorf("%s: focused border should be default and the other dim (left dim %v, right dim %v)", name, leftDim, rightDim)
		}
	}
	check("accounts focused", m, true)
	right, _ := press(m, "tab")
	check("detail focused", right, false)

	out := raw(m)
	if !strings.Contains(out, "\x1b[1mrobin") {
		t.Error("the selected account should be bold")
	}
	if !strings.Contains(out, "\x1b[1menter") || !strings.Contains(out, "\x1b[90mrun") {
		t.Error("footer key names should be bold and descriptions dim")
	}
	active := regexp.MustCompile("\x1b\\[([0-9;]*)mOverview").FindStringSubmatch(out)
	if active == nil || !strings.Contains(";"+active[1]+";", ";1;") || !strings.Contains(";"+active[1]+";", ";4;") {
		t.Errorf("the active tab should be bold and underlined, got %v", active)
	}
	if !strings.Contains(out, "\x1b[90mSessions") {
		t.Error("inactive tabs should be dim")
	}
}
