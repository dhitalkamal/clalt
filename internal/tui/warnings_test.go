package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// a reload that fails must say so. the data that is on screen is the last that
// loaded, and without a word it reads as current.
func TestAFailedReloadIsShownAndTheOldDataStays(t *testing.T) {
	m := newTestModel(&fakeController{})
	m.refreshing = true

	next, _ := m.Update(dataMsg{err: errors.New("parse state.json: boom")})
	nm := next.(dashModel)

	if !strings.HasPrefix(nm.flash, "failed: reload") || !strings.Contains(nm.flash, "boom") {
		t.Errorf("flash = %q: the verdict comes first, then what failed and why", nm.flash)
	}
	if nm.refreshing {
		t.Error("the spinner is still going")
	}
	if len(nm.data.Accounts) != len(m.data.Accounts) || nm.data.Accounts[0].Name != "work" {
		t.Error("the data that was on screen was replaced by a failed load")
	}
	if !strings.Contains(nm.footer(), "failed: reload") {
		t.Errorf("the footer does not show it: %q", nm.footer())
	}
}

// a reload that works takes the flash of an earlier failed one away, and a key
// does too, like any other flash
func TestAReloadThatWorksAfterAFailedOneClearsTheFlash(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	next, _ := m.Update(dataMsg{err: errors.New("boom")})
	m = next.(dashModel)

	next, _ = m.Update(dataMsg{data: f.data})
	m = next.(dashModel)

	if m.flash != "" {
		t.Errorf("flash = %q after a reload that worked", m.flash)
	}
}

// what the loader could not read is part of the data and stays in the header for
// as long as it holds: an empty list of sessions that is really an unreadable
// folder must not look like no sessions
func TestADataWarningIsShownInTheHeader(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)
	m.data.Warning = "the running sessions could not be read: permission denied"

	header := m.header()

	if !strings.Contains(header, "the running sessions could not be read") {
		t.Errorf("header = %q", header)
	}
	if w := lipgloss.Width(header); w > m.viewWidth() {
		t.Errorf("the header is %d wide in %d columns", w, m.viewWidth())
	}
	if strings.Contains(header, "\n") {
		t.Error("the header is more than one line")
	}
}

func TestALongWarningIsCutToTheHeaderAndKeepsTheVersionAndTheCount(t *testing.T) {
	m := newTestModel(&fakeController{})
	m.width = 80
	m.data.Warning = strings.Repeat("usage could not be read. ", 20)

	header := m.header()

	if w := lipgloss.Width(header); w > m.viewWidth() {
		t.Errorf("the header is %d wide in %d columns", w, m.viewWidth())
	}
	for _, want := range []string{"v0.1.0", "signed in: 3/4", "usage could not be read"} {
		if !strings.Contains(header, want) {
			t.Errorf("header is missing %q: %q", want, header)
		}
	}
}

func TestTheStaticFrameShowsTheWarningToo(t *testing.T) {
	d := sampleData()
	d.Warning = "usage could not be read"

	if frame := Render(d, 100, 40); !strings.Contains(frame, "usage could not be read") {
		t.Errorf("the frame does not show the warning:\n%s", frame)
	}
}

// the verdict of an action that failed comes first, so that a long reason cut to
// the width of the footer still says what happened
func TestAFailedActionHasTheVerdictFirst(t *testing.T) {
	for _, c := range []struct {
		keys []string
		verb string
	}{
		{[]string{"l", "y"}, "logout"},
		{[]string{"d", "y"}, "remove"},
	} {
		f := &fakeController{failNext: errors.New("could not sign work out: boom")}
		m := newTestModel(f)

		m = pump(m, c.keys...)

		if !strings.HasPrefix(m.flash, "failed: "+c.verb) || !strings.Contains(m.flash, "boom") {
			t.Errorf("%s: flash = %q", c.verb, m.flash)
		}
	}
}

func TestAnActionThatWorkedKeepsItsFlash(t *testing.T) {
	f := &fakeController{}
	m := newTestModel(f)

	m = pump(m, "l", "y")

	if m.flash != "signed out work" {
		t.Errorf("flash = %q", m.flash)
	}
}

// text that did not come clean still cannot break the line or reach the terminal
func TestAWarningWithLineBreaksAndEscapesStaysOnOneLine(t *testing.T) {
	m := newTestModel(&fakeController{})
	m.data.Warning = "usage\ncould not\r\nbe read \x1b[31mred\x07"

	header := m.header()

	if strings.ContainsAny(header, "\n\r\a") || strings.Contains(header, "\x1b[31m") {
		t.Errorf("header = %q", header)
	}
	if !strings.Contains(header, "usage could not be read") {
		t.Errorf("the words are gone: %q", header)
	}
	m.flash = ""
	next, _ := m.Update(dataMsg{err: errors.New("bad\nline \x1b]0;owned\x07")})
	if flash := next.(dashModel).flash; strings.ContainsAny(flash, "\n\a\x1b") {
		t.Errorf("flash = %q", flash)
	}
}
