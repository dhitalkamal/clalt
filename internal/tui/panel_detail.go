package tui

import (
	"strings"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// renderDetail draws the right-hand pane for the selected account, or the command
// palette when it is open. It is never focusable; the title carries the account.
func (m dashModel) renderDetail(innerW, innerH int) string {
	if m.mode == "palette" {
		return panelBox("Command palette", innerW, innerH, m.paletteView(innerW), true)
	}
	a := m.active()
	if a == nil {
		return panelBox("Detail", innerW, innerH, dimStyle.Render("no account selected"), false)
	}
	title := a.Name + " (" + dashPlan(a.Plan) + ")"
	return panelBox(title, innerW, innerH, m.detailBody(a, innerW), false)
}

// detailBody assembles the meters, sparklines, and figures for one account.
func (m dashModel) detailBody(a *AccountView, innerW int) string {
	barW := innerW / 3
	if barW < 8 {
		barW = 8
	}
	if barW > 22 {
		barW = 22
	}
	s := a.Stats
	var b strings.Builder

	b.WriteString(localWindowRow("5h", a.Five, barW) + "\n")
	b.WriteString(localWindowRow("7d", a.Seven, barW) + "\n")

	b.WriteString(detailLabel("today") + costStyle.Render(money(s.Today.Cost)) + "  " +
		barStyle.Render(humanTokens(s.Today.Billable)) + dimStyle.Render(" tokens") + "\n")
	b.WriteString(detailLabel("trend") + sparkStyle.Render(accountSpark(a)) + dimStyle.Render("  (7d)") + "\n")

	if len(s.Models) > 0 {
		b.WriteString(detailLabel("models") + modelsInline(s.Models) + "\n")
	}
	b.WriteString(detailLabel("hour") + sparkStyle.Render(hourSpark(s.HourOfDay)) + "\n")
	if len(s.Projects) > 0 {
		b.WriteString(detailLabel("proj") + projectsInline(s.Projects) + "\n")
	}

	email := a.Email
	if email == "" {
		email = "-"
	}
	budget := dimStyle.Render("no budget")
	if a.DailyBudget > 0 {
		budget = dimStyle.Render("budget " + money(a.DailyBudget) + "/day")
	}
	b.WriteString(detailLabel("email") + textStyle.Render(email) + "   " + budget + "\n")
	b.WriteString(detailLabel("login") + signInState(a))
	return b.String()
}

func detailLabel(s string) string { return dimStyle.Render(padRight(s, 8)) }

// localWindowRow renders a local window as a proxy meter plus a token count,
// since there is no local hard cap.
func localWindowRow(label string, w usage.WindowStat, barW int) string {
	const ref = 2_000_000.0
	frac := float64(w.Tokens) / ref
	if frac > 1 {
		frac = 1
	}
	return detailLabel(label) + gradedMeter(frac, barW) + " " +
		dimStyle.Render(padLeft(humanTokens(w.Tokens), 6)+" tok")
}

// accountSpark renders a 7-day token trend from the account's daily series.
func accountSpark(a *AccountView) string {
	d := a.Daily
	if len(d) > 7 {
		d = d[len(d)-7:]
	}
	vals := make([]int64, len(d))
	for i, bkt := range d {
		vals[i] = bkt.Tokens
	}
	return sparkline(vals)
}

// hourSpark renders the lifetime message-by-hour shape as a 24-cell sparkline.
func hourSpark(hours [24]int) string {
	vals := make([]int64, 24)
	for i, v := range hours {
		vals[i] = int64(v)
	}
	return sparkline(vals)
}

// modelsInline renders the top models as inline "name ###" mini bars.
func modelsInline(models []usage.ModelUsage) string {
	var max int64 = 1
	for _, mu := range models {
		if mu.Billable > max {
			max = mu.Billable
		}
	}
	var parts []string
	for i, mu := range models {
		if i >= 3 {
			break
		}
		frac := float64(mu.Billable) / float64(max)
		parts = append(parts, modelWord(mu.Model)+" "+barStyle.Render(horizBar(frac, 3)))
	}
	return strings.Join(parts, "  ")
}

// projectsInline renders the top projects as inline "name ##" mini bars.
func projectsInline(projects []usage.ProjectUsage) string {
	var max int64 = 1
	for _, p := range projects {
		if p.Billable > max {
			max = p.Billable
		}
	}
	var parts []string
	for i, p := range projects {
		if i >= 3 {
			break
		}
		frac := float64(p.Billable) / float64(max)
		parts = append(parts, trunc(p.Project, 8)+" "+barStyle.Render(horizBar(frac, 3)))
	}
	return strings.Join(parts, "  ")
}

// modelWord is the short friendly word for a model id (opus, sonnet, haiku).
func modelWord(id string) string {
	f := strings.Fields(shortModel(id))
	if len(f) == 0 {
		return "?"
	}
	return f[0]
}

// signInState renders the sign-in from the state file, colored by urgency.
func signInState(a *AccountView) string {
	if !a.SignedIn {
		return lipglossFg(colWarn).Render("signed out")
	}
	state := okStyle.Render("signed in")
	if a.LoginDue.IsZero() {
		return state
	}
	due := dimStyle.Render("  sign in again by " + a.LoginDue.Local().Format("2006-01-02"))
	if a.needsLogin() {
		due = lipglossFg(colWarn).Render("  sign in again by " + a.LoginDue.Local().Format("2006-01-02"))
	}
	return state + due
}
