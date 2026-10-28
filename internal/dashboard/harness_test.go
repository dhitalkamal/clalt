package dashboard

import (
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// the helpers here are exported so the external test package shares them

var sgrRe = regexp.MustCompile("\x1b\\[([0-9;:]*)m")

// SGRParams lists every SGR parameter used in s.
func SGRParams(s string) []string {
	var out []string
	for _, m := range sgrRe.FindAllStringSubmatch(s, -1) {
		out = append(out, strings.Split(m[1], ";")...)
	}
	return out
}

// IsASCII reports whether s has only 7-bit bytes.
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// KeyMsg builds the key press a terminal would send for a key name.
func KeyMsg(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(name)
	if len(r) != 1 {
		panic("KeyMsg: unknown key name " + name)
	}
	return tea.KeyPressMsg{Code: r[0], Text: name}
}

// Press sends each key in turn and returns the last command.
func Press(m tea.Model, names ...string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, n := range names {
		m, cmd = m.Update(KeyMsg(n))
	}
	return m, cmd
}

// TypeText sends each rune as its own key press.
func TypeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(KeyMsg(string(r)))
	}
	return m
}

// Resize gives a model a terminal size, like the first size message a real
// program gets.
func Resize(m tea.Model, w, h int) tea.Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// Screen is the view with the escape codes removed.
func Screen(m tea.Model) string { return ansi.Strip(m.View().Content) }

// Raw is the view as drawn, escape codes included.
func Raw(m tea.Model) string { return m.View().Content }

// Patience is how long a test waits for something that should happen. it is
// a ceiling, not a delay: every wait returns as soon as its condition holds, so
// a loaded machine gets room and a passing run costs nothing.
const Patience = 30 * time.Second

// RunCmd runs cmd and returns its message, or nil when it has not finished
// after wait. a timer command counts as blocked.
func RunCmd(cmd tea.Cmd, wait time.Duration) tea.Msg {
	if cmd == nil {
		return nil
	}
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		return msg
	case <-time.After(wait):
		return nil
	}
}

// Quits reports whether cmd ends the program. a blocked command, like the
// flash timer, counts as not quitting.
func Quits(cmd tea.Cmd) bool {
	_, ok := RunCmd(cmd, 150*time.Millisecond).(tea.QuitMsg)
	return ok
}

// Settle runs cmd and feeds every message it produces back into the model,
// following the commands that come out of that. commands that stay blocked,
// the timers, are left alone.
func Settle(m tea.Model, cmd tea.Cmd) tea.Model {
	queue := []tea.Cmd{cmd}
	for i := 0; i < 50 && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		msg := RunCmd(c, 40*time.Millisecond)
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		var next tea.Cmd
		m, next = m.Update(msg)
		queue = append(queue, next)
	}
	return m
}

// Boot starts a model the way a program would: it gets its size, then the
// first load finishes. the refresh tick is left alone.
func Boot(m tea.Model, w, h int) tea.Model {
	m = Resize(m, w, h)
	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		panic("Init should batch the first load and the refresh tick")
	}
	first := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func(c tea.Cmd) { first <- c() }(c)
	}
	m, _ = m.Update(<-first)
	return m
}
