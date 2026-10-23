package dashboard

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type flashExpiredMsg struct{ id int }

type launchMsg struct {
	launch Launch
	err    error
}

type execDoneMsg struct{ err error }

type actionDoneMsg struct {
	text string // what to say when it worked
	err  error
}

// firstLine is the first line of an error that says anything, cleaned for the
// terminal, so a blank line or a color code does not stand for the reason
func firstLine(s string) string {
	for _, line := range strings.Split(ansi.Strip(s), "\n") {
		if t := strings.TrimSpace(clean(line)); t != "" {
			return t
		}
	}
	return ""
}

// flashTTL keeps a failure on screen longer than a plain message
func (m model) flashTTL(isErr bool) time.Duration {
	if isErr {
		return 3 * m.opt.FlashFor
	}
	return m.opt.FlashFor
}

// say shows a message on the footer line until its timer fires
func (m model) say(text string, isErr bool) (model, tea.Cmd) {
	m.flash = flash{text: firstLine(text), err: isErr, id: m.flash.id + 1}
	id := m.flash.id
	return m, tea.Tick(m.flashTTL(isErr), func(time.Time) tea.Msg { return flashExpiredMsg{id: id} })
}

// fail shows an action's error. a bare Notice is only a message. an error
// that merely contains one is a failure, so a real error is never shown as a
// notice.
func (m model) fail(err error) (model, tea.Cmd) {
	if n, ok := err.(Notice); ok {
		return m.say(string(n), false)
	}
	m.logf("action failed: %v", err)
	return m.say(failureText(err), true)
}

// failureText is the text of an error that is made of several errors: the
// first one that is not a notice
func failureText(err error) string {
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range multi.Unwrap() {
			if _, notice := e.(Notice); !notice {
				return failureText(e)
			}
		}
	}
	return err.Error()
}

// working marks an action as in flight until its answer comes back, so a
// second key press cannot start it twice
func (m model) working(cmd tea.Cmd) (model, tea.Cmd) {
	m.busy = true
	return m, cmd
}

func (m model) onAction(k string) (model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	a, ok := m.selected()
	switch {
	case !ok:
		return m.say("No account selected.", false)
	case k == "s":
		return m.working(m.launch(a.ID, true))
	case !a.SignedIn:
		return m.say(a.Label+" is signed out. Press s to sign in.", false)
	case k == "enter":
		return m.working(m.launch(a.ID, false))
	case k == "e":
		return m.openBudget(a), nil
	}
	return m.askStop(a)
}

// accountKey is rename and sign out. they act on the accounts list, so the
// right pane only says where they work.
func (m model) accountKey(k string) (model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	a, ok := m.selected()
	if !ok {
		return m.say("No account selected.", false)
	}
	if m.focus != paneAccounts {
		what := "rename it"
		if k == "d" {
			what = "sign it out"
		}
		return m.say("Press tab to pick an account, then "+k+" to "+what+".", false)
	}
	switch {
	case k == "r":
		return m.askRename(a), nil
	case !a.SignedIn:
		return m.say(a.Label+" is already signed out.", false)
	}
	return m.askSignOut(a), nil
}

// launch asks the actions for something to run with the terminal released
func (m model) launch(id string, signIn bool) tea.Cmd {
	act, ctx := m.act, m.ctx
	return func() tea.Msg {
		var l Launch
		err := protect("action", func() (err error) {
			if signIn {
				l, err = act.SignIn(ctx, id)
			} else {
				l, err = act.Run(ctx, id)
			}
			return
		})
		return launchMsg{launch: l, err: err}
	}
}

// minRun is the least time a launch keeps the terminal. a variable so a test can lengthen it. bubbletea v2.1.0 can
// leave its renderer stopped when an exec returns at once, which a refusal
// that was already shown can do.
var minRun = 50 * time.Millisecond

// heldLaunch is a Launch that does not return before min has passed
type heldLaunch struct {
	Launch
	min time.Duration
}

func (h heldLaunch) Run() error {
	start := time.Now()
	err := h.Launch.Run()
	if left := h.min - time.Since(start); left > 0 {
		time.Sleep(left)
	}
	return err
}

func (m model) onLaunch(msg launchMsg) (tea.Model, tea.Cmd) {
	var cannot error
	if starter, ok := msg.launch.(interface{ StartError() error }); ok && msg.launch != nil {
		cannot = starter.StartError()
	}
	// the launch that runs next keeps the dashboard busy until execDoneMsg
	if msg.err != nil || msg.launch == nil || cannot != nil {
		m.busy = false
	}
	switch {
	case msg.err != nil:
		return m.fail(msg.err)
	case msg.launch == nil:
		return m.say("Nothing was started.", false)
	case cannot != nil:
		return m.fail(cannot)
	}
	return m, tea.Exec(heldLaunch{msg.launch, minRun}, func(err error) tea.Msg { return execDoneMsg{err: err} })
}

// onExecDone reloads, since claude may have changed a login or the usage. a
// launch that ran and ended with a status says it, except for the ways of
// leaving claude that are normal: ctrl+c, which is 130, and a signal. an error
// that has no status, such as a failure to run at all, is a red message.
func (m model) onExecDone(msg execDoneMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	var cmds []tea.Cmd
	var exit interface{ ExitCode() int }
	if msg.err != nil {
		var c tea.Cmd
		if !errors.As(msg.err, &exit) {
			m.logf("action failed: %v", msg.err)
			m, c = m.say(msg.err.Error(), true)
		} else if code := exit.ExitCode(); code > 0 && code != 130 {
			m.logf("action failed: claude exited with status %d", code)
			m, c = m.say(fmt.Sprintf("claude exited with status %d", code), false)
		}
		cmds = append(cmds, c)
	}
	m, load := m.reload()
	return m, tea.Batch(append(cmds, load)...)
}

// actCmd runs a blocking action off the UI loop
func (m model) actCmd(done string, fn func() error) tea.Cmd {
	return func() tea.Msg { return actionDoneMsg{text: done, err: protect("action", fn)} }
}

func (m model) onActionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		return m.fail(msg.err)
	}
	m, say := m.say(msg.text, false)
	m, load := m.reload()
	return m, tea.Batch(say, load)
}
