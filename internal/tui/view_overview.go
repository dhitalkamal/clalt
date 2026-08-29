package tui

import "strings"

// viewOverview renders the accounts table inside a titled box: name, plan, a
// 7-day sparkline, color-graded 5h/7d meters (with reset countdowns when live,
// token counts locally), and expiry.
func (m dashModel) viewOverview() string {
	innerW := m.viewWidth() - 2
	accts := m.visibleAccounts()

	var maxFive, maxSeven int64 = 1, 1
	for _, a := range accts {
		if a.Five.Tokens > maxFive {
			maxFive = a.Five.Tokens
		}
		if a.Seven.Tokens > maxSeven {
			maxSeven = a.Seven.Tokens
		}
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render(
		padRight("  account", 18)+padRight("7d trend", 12)+
			padRight("5h", 16)+padRight("7d", 16)+"expires") + "\n\n")

	if len(accts) == 0 {
		b.WriteString(dimStyle.Render("  no accounts match " + m.query))
	}
	for i, a := range accts {
		cursor := "  "
		if i == m.sel {
			cursor = cursorStyle.Render("> ")
		}
		name := a.Name
		if a.Active {
			name = activeStyle.Render(name + " *")
		}
		row := cursor + padRight(name, 16) +
			sparkStyle.Render(padRight(accountSpark(a), 12)) +
			padRight(m.windowCell(a, true, maxFive), 16) +
			padRight(m.windowCell(a, false, maxSeven), 16) +
			dimStyle.Render(humanUntil(a.ExpiresAt))
		if i == m.sel {
			row = rowSelStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}
	title := "ACCOUNTS"
	if m.query != "" {
		title += "   filter: " + m.query
	}
	return titledBox(title, innerW, b.String())
}

// accountSpark renders a 7-day trend from the account's daily billable tokens.
func accountSpark(a AccountView) string {
	d := a.Daily
	if len(d) > 7 {
		d = d[len(d)-7:]
	}
	vals := make([]int64, len(d))
	for i, b := range d {
		vals[i] = b.Tokens
	}
	return sparkline(vals)
}

// windowCell renders one compact usage cell: a graded meter plus a percent
// (live) or token count (local). Reset countdowns live on the usage tab to keep
// the overview uncluttered.
func (m dashModel) windowCell(a AccountView, five bool, localMax int64) string {
	const barW = 8
	if a.Live != nil {
		lw := a.Live.SevenDay
		if five {
			lw = a.Live.FiveHour
		}
		return gradedMeter(lw.Utilization, barW) + " " + padLeft(pct(lw.Utilization), 4)
	}
	w := a.Seven
	if five {
		w = a.Five
	}
	frac := 0.0
	if localMax > 0 {
		frac = float64(w.Tokens) / float64(localMax)
	}
	return gradedMeter(frac, barW) + " " + dimStyle.Render(padLeft(humanTokens(w.Tokens), 5))
}

func dashPlan(p string) string {
	if p == "" {
		return "-"
	}
	return p
}
