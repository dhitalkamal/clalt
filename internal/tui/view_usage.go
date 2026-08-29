package tui

import (
	"fmt"
	"strings"
)

// viewUsage renders per-account window detail: the 5h and 7d windows (live when
// available, else local), token/cost totals, and top projects.
func (m dashModel) viewUsage() string {
	a := m.active()
	if a == nil {
		return ""
	}
	innerW := m.viewWidth() - 2
	s := a.Stats

	var w strings.Builder
	w.WriteString(dimStyle.Render("plan ") + dashPlan(a.Plan) + "   " +
		dimStyle.Render("token ") + humanUntil(a.ExpiresAt) + "\n")
	if a.Live != nil {
		w.WriteString(windowLine("5h live", a.Live.FiveHour.Utilization, resetLabel(a.Live.FiveHour.ResetsAt)))
		w.WriteString(windowLine("7d live", a.Live.SevenDay.Utilization, resetLabel(a.Live.SevenDay.ResetsAt)))
	}
	w.WriteString(localWindowLine("5h local", a.Five.Tokens, a.Five.Messages))
	w.WriteString(localWindowLine("7d local", a.Seven.Tokens, a.Seven.Messages))
	w.WriteString(kv("total", barStyle.Render(humanTokens(s.Billable))+" tokens  "+
		costStyle.Render(money(s.Cost))+" est") + "\n")
	w.WriteString(kv("sessions", humanCount(s.Sessions)))
	if a.DailyBudget > 0 {
		frac := s.Today.Cost / a.DailyBudget
		note := ""
		if frac >= 1 {
			note = " " + lipglossFg(colErr).Render("OVER")
		} else if frac >= 0.8 {
			note = " " + lipglossFg(colWarn).Render("near")
		}
		w.WriteString("\n" + kv("budget", gradedMeter(frac, 14)+" "+
			money(s.Today.Cost)+" / "+money(a.DailyBudget)+" today"+note))
	}

	title := "USAGE  " + a.Name
	if a.Active {
		title += " (active)"
	}
	out := titledBox(title, innerW, w.String())

	if len(s.Projects) > 0 {
		var p strings.Builder
		var maxProj int64 = 1
		for _, pr := range s.Projects {
			if pr.Billable > maxProj {
				maxProj = pr.Billable
			}
		}
		for i, pr := range s.Projects {
			if i >= 6 {
				break
			}
			frac := float64(pr.Billable) / float64(maxProj)
			p.WriteString(padRight(trunc(pr.Project, 26), 26) + " " +
				barStyle.Render(horizBar(frac, 14)) + " " +
				padLeft(humanTokens(pr.Billable), 8) + "\n")
		}
		out += "\n" + titledBox("TOP PROJECTS", innerW, p.String())
	}
	return out
}

func windowLine(label string, util float64, reset string) string {
	return dimStyle.Render(padRight(label, 10)) +
		barStyle.Render(meter(util, 20)) + " " +
		padLeft(pct(util), 4) + " " + dimStyle.Render(reset) + "\n"
}

func localWindowLine(label string, tokens int64, msgs int) string {
	return dimStyle.Render(padRight(label, 10)) +
		barStyle.Render(humanTokens(tokens)) + dimStyle.Render(" tokens over ") +
		fmt.Sprintf("%d", msgs) + dimStyle.Render(" msgs") + "\n"
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "~"
}
