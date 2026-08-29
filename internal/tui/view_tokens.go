package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewTokens renders token analytics for the selected account: a compact summary
// and top models on the left, daily/activity charts on the right. Stacks to one
// column on narrow terminals.
func (m dashModel) viewTokens() string {
	a := m.active()
	if a == nil {
		return ""
	}
	if m.viewWidth() < 100 {
		w := m.viewWidth() - 2
		return m.tokenPanels(a, w) + "\n" + m.tokenCharts(a, w)
	}
	leftInner := 40
	rightInner := m.viewWidth() - (leftInner + 2) - 3
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.tokenPanels(a, leftInner), "  ", m.tokenCharts(a, rightInner))
}

// tokenPanels builds the boxed summary + models column.
func (m dashModel) tokenPanels(a *AccountView, iw int) string {
	s := a.Stats
	comp := s.Composition()

	var sum strings.Builder
	sum.WriteString(kv("today", barStyle.Render(humanTokens(s.Today.Billable))+"  "+
		costStyle.Render(money(s.Today.Cost))+dimStyle.Render("  "+humanCount(s.Today.Messages)+" msgs")) + "\n")
	sum.WriteString(kv("total", barStyle.Render(humanTokens(s.Billable))+"  "+
		costStyle.Render(money(s.Cost))+dimStyle.Render("  "+humanCount(s.Sessions)+" sessions")) + "\n")
	sum.WriteString(kv("cache", infoStyle.Render(fmt.Sprintf("%.0f%% hit", s.CacheHitRate*100))+
		dimStyle.Render(fmt.Sprintf("   cr %.0f / cw %.0f / io %.0f", comp.CacheRead*100, comp.CacheWrite*100, (comp.Input+comp.Output)*100))))

	var models strings.Builder
	var maxModel int64 = 1
	for _, mu := range s.Models {
		if mu.Billable > maxModel {
			maxModel = mu.Billable
		}
	}
	for i, mu := range s.Models {
		if i >= 3 {
			break
		}
		models.WriteString(padRight(shortModel(mu.Model), 10) +
			barStyle.Render(horizBar(float64(mu.Billable)/float64(maxModel), 12)) + " " +
			padLeft(humanTokens(mu.Billable), 7) + " " + costStyle.Render(padLeft(money(mu.Cost), 9)))
		if i < 2 && i < len(s.Models)-1 {
			models.WriteString("\n")
		}
	}

	return titledBox("SUMMARY", iw, sum.String()) + "\n" +
		titledBox("TOP MODELS", iw, models.String())
}

// tokenCharts builds the boxed DAILY and ACTIVITY charts at the given inner width.
func (m dashModel) tokenCharts(a *AccountView, iw int) string {
	if iw < 20 {
		iw = 20
	}
	tokens := make([]int64, len(a.Daily))
	msgs := make([]int64, len(a.Daily))
	var peakTok int64
	var peakDay string
	for i, d := range a.Daily {
		tokens[i] = d.Tokens
		msgs[i] = int64(d.Messages)
		if d.Tokens > peakTok {
			peakTok, peakDay = d.Tokens, lcDate(d.Day, "Jan 2")
		}
	}
	daily := strings.Join(renderChart(downsample(tokens, iw), 9), "\n") + "\n" + m.axis(a, iw)
	act := strings.Join(renderChart(downsample(msgs, iw), 9), "\n") + "\n" + m.axis(a, iw)

	dTitle := "DAILY  tokens/day"
	if peakTok > 0 {
		dTitle += fmt.Sprintf("  peak %s %s", humanTokens(peakTok), peakDay)
	}
	return titledBox(dTitle, iw, daily) + "\n" + titledBox("ACTIVITY  msgs/day", iw, act)
}

func renderChart(vals []int64, rows int) []string {
	out := vBarChart(vals, rows)
	for i := range out {
		out[i] = barStyle.Render(out[i])
	}
	return out
}

func (m dashModel) axis(a *AccountView, width int) string {
	if len(a.Daily) == 0 {
		return ""
	}
	first := lcDate(a.Daily[0].Day, "Jan")
	last := lcDate(a.Daily[len(a.Daily)-1].Day, "Jan")
	return dimStyle.Render(padRight(first, width-len(last)) + last)
}

func kv(label, value string) string {
	return dimStyle.Render(padRight(label, 8)) + value
}
