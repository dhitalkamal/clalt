package dashboard

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// text from an adapter ends up on the terminal, so one with an escape sequence
// in it could move the cursor, set the window title or open a link. clean
// removes the sequences and turns the control characters into spaces.

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		// format characters (a right to left override) and line separators
		// change what is drawn or where a line ends. the joiners that scripts
		// and emoji need are kept.
		if unicode.IsControl(r) || (unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) && r != '\u200c' && r != '\u200d') {
			return ' '
		}
		return r
	}, ansi.Strip(s))
}

func cleanAll(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = clean(s)
	}
	return out
}

// cleanSnapshot is a copy of snap with every text field cleaned, except the
// ids, which are keys the actions use and are never drawn
func cleanSnapshot(snap Snapshot) Snapshot {
	out := Snapshot{Warnings: cleanAll(snap.Warnings)}
	if snap.Accounts == nil {
		return out
	}
	out.Accounts = make([]Account, len(snap.Accounts))
	for i, a := range snap.Accounts {
		a.Label, a.Email, a.Org, a.Plan = clean(a.Label), clean(a.Email), clean(a.Org), clean(a.Plan)
		a.Problem, a.LastIn = clean(a.Problem), clean(a.LastIn)
		a.Limits.Days = cleanAll(a.Limits.Days)
		a.Profile.Folder, a.Profile.ClaudeVersion, a.Profile.Gateway = clean(a.Profile.Folder), clean(a.Profile.ClaudeVersion), clean(a.Profile.Gateway)
		a.Sessions = cleanSessions(a.Sessions)
		a.History = cleanHistory(a.History)
		out.Accounts[i] = a
	}
	return out
}

func cleanSessions(in []Session) []Session {
	if in == nil {
		return nil
	}
	out := make([]Session, len(in))
	for i, s := range in {
		s.Folder, s.Branch, s.Model, s.Effort = clean(s.Folder), clean(s.Branch), clean(s.Model), clean(s.Effort)
		s.State = SessionState(clean(string(s.State)))
		out[i] = s
	}
	return out
}

func cleanHistory(h History) History {
	days := make([]Day, len(h.Days))
	for i, d := range h.Days {
		d.Label = clean(d.Label)
		days[i] = d
	}
	if h.Days == nil {
		days = nil
	}
	return History{Days: days, Projects: cleanShares(h.Projects), Models: cleanShares(h.Models)}
}

func cleanShares(in []Share) []Share {
	if in == nil {
		return nil
	}
	out := make([]Share, len(in))
	for i, s := range in {
		s.Name = clean(s.Name)
		if s.Activity != nil {
			acts := make([]Activity, len(s.Activity))
			for j, a := range s.Activity {
				a.Day, a.Model = clean(a.Day), clean(a.Model)
				acts[j] = a
			}
			s.Activity = acts
		}
		out[i] = s
	}
	return out
}
