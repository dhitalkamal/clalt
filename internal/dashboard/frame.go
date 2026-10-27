package dashboard

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m model) render() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	g := m.geom()
	if g.mode == modeTooSmall {
		return m.th.tooSmall(m.w, m.h)
	}
	if m.help {
		back := m
		back.th.p = m.th.p.dimmed()
		return strings.Join(overlay(back.frame(g), m.th.helpBox(m.w, m.h, m.helpAt), m.w, m.h), "\n")
	}
	if m.confirm.open && m.confirm.kind == confirmDelete {
		back := m
		back.th.p = m.th.p.dimmed()
		return strings.Join(overlay(back.frame(g), m.th.deleteBox(m.confirm, m.w, m.h), m.w, m.h), "\n")
	}
	return strings.Join(m.frame(g), "\n")
}

func (m model) frame(g geom) []string {
	lines := make([]string, 0, m.h)
	lines = append(lines, m.th.p.draw(m.headerLine()))
	if m.warnRows() > 0 {
		lines = append(lines, m.th.p.draw(m.warningLine()))
	}
	var left, right []string
	if g.leftW > 0 {
		left = m.accountsPane(g)
	}
	if g.rightW > 0 {
		right = m.detailPane(g)
	}
	for i := 0; i < g.paneH; i++ {
		switch {
		case left != nil && right != nil:
			lines = append(lines, " "+left[i]+" "+right[i])
		case left != nil:
			lines = append(lines, " "+left[i])
		default:
			lines = append(lines, " "+right[i])
		}
	}
	return append(lines, m.th.p.draw(m.footerLine()))
}

// headerLine shows the counts and how fresh the data is, or what went wrong
func (m model) headerLine() rich {
	if !m.have {
		if m.loadErr != nil {
			return headerLine(m.w, rich{bad("load failed")})
		}
		return headerLine(m.w, rich{dim("loading...")})
	}
	running := m.running()
	counts := fmt.Sprintf("%s, %d running", plural(len(m.snap.Accounts), "account"), running)
	if m.loadErr != nil {
		return m.failedHeader()
	}
	status := "updated " + agoText(m.now().Sub(m.loadedAt))
	if m.loading && m.now().Sub(m.loadStart) >= time.Second {
		status = "updating..."
	}
	return headerLine(m.w, rich{dim(counts + ", " + status)}, rich{dim(counts)}, rich{dim(fmt.Sprintf("%d running", running))})
}

// failedHeader says the refresh failed and how old the data on screen is. a
// long reason is cut first, then dropped, and the age goes last.
func (m model) failedHeader() rich {
	age := fmt.Sprintf(" (data from %s)", agoText(m.now().Sub(m.loadedAt)))
	reason := rich{bad("update failed: " + firstLine(m.loadErr.Error()))}
	// the name takes six columns and the gap two
	room := m.w - margin - 8 - len(age)
	var cands []rich
	if room >= len("update failed: ")+4 {
		cands = append(cands, cat(reason.ellipsize(room, m.th.g.ellipsis), rich{dim(age)}))
	}
	cands = append(cands, cat(rich{bad("update failed")}, rich{dim(age)}), rich{bad("update failed")})
	return headerLine(m.w, cands...)
}

// footerLine is the question being asked, else the latest message, else the
// key hints for the current mode
func (m model) footerLine() rich {
	switch {
	case m.entry.open:
		return m.entryLine()
	case m.confirm.open:
		return m.confirmLine()
	case m.busy:
		return rich{gap(2), dim("working...")}
	case m.flash.text != "":
		t := toneBold
		if m.flash.err {
			t = toneBad
		}
		return rich{gap(2), span{t, m.flash.text}}.ellipsize(m.w, m.th.g.ellipsis)
	case m.help:
		return hintsLine(helpHints, m.w)
	case m.filtering:
		return hintsLine(filterHints, m.w)
	case m.drillFocused():
		return hintsLine(drillHints, m.w)
	case m.removedSelected() && m.filter != "":
		return hintsLine(append([]hint{{"esc", "clear filter", 10}}, removedHints...), m.w)
	case m.removedSelected():
		return hintsLine(removedHints, m.w)
	case m.filter != "":
		return hintsLine(keptHints(), m.w)
	}
	return hintsLine(normalHints, m.w)
}

// warningLine is the first warning and how many more there are
func (m model) warningLine() rich {
	ws := m.snap.Warnings
	var more rich
	if len(ws) > 1 {
		more = rich{dim(fmt.Sprintf(" (+%d more)", len(ws)-1))}
	}
	first := rich{warn("! " + ws[0])}.ellipsize(max(m.w-margin-more.width(), 0), m.th.g.ellipsis)
	return cat(rich{gap(margin)}, first, more)
}

func (m model) entryLine() rich {
	e := m.entry
	line := rich{gap(2), dim(e.question()), txt(e.text)}
	if e.note != "" {
		line = append(line, gap(2), bad(e.note))
	}
	return line.clip(m.w)
}

// cursor is the real terminal cursor, shown while a field is being typed
func (m model) cursor() *tea.Cursor {
	g := m.geom()
	if m.help || g.mode == modeTooSmall {
		return nil
	}
	switch {
	case m.entry.open:
		x := 2 + ansi.StringWidth(m.entry.question()) + ansi.StringWidth(m.entry.text)
		c := tea.NewCursor(min(x, m.w-1), m.h-1)
		c.Shape = tea.CursorBar
		return c
	case m.filtering && g.leftW > 0:
		// margin, border and padding come before the "/ " prefix
		first := margin + 1 + 1
		x := min(first+2+len(m.filter), first+max(g.leftW-5, 1)-1)
		c := tea.NewCursor(x, 2+m.warnRows())
		c.Shape = tea.CursorBar
		return c
	}
	return nil
}
