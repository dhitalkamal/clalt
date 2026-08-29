package tui

import (
	"fmt"
	"strings"
)

// viewFallback renders the rotation chain (order + per-account 5h utilization)
// and the rotation policy, mirroring the order `clalt next` walks.
func (m dashModel) viewFallback() string {
	chain := m.data.Fallback
	if len(chain) == 0 {
		for _, a := range m.data.Accounts {
			chain = append(chain, a.Name)
		}
	}
	byName := map[string]AccountView{}
	for _, a := range m.data.Accounts {
		byName[a.Name] = a
	}

	innerW := m.viewWidth() - 2

	var chainBody strings.Builder
	for i, name := range chain {
		a, ok := byName[name]
		if !ok {
			continue
		}
		frac := liveFiveFrac(&a)
		marker := "  "
		if a.Active {
			marker = activeStyle.Render(" *")
		}
		chainBody.WriteString(fmt.Sprintf("  %d.", i+1) + marker + " " + padRight(name, 12) +
			gradedMeter(frac, 18) + " " + padLeft(pct(frac), 4) + "\n")
	}

	var pol strings.Builder
	pol.WriteString(policyRow("on limit", "rotate to the next healthy account"))
	pol.WriteString(policyRow("threshold", fmt.Sprintf("%.0f%% of the 5h window", m.thresholdPct())))
	pol.WriteString(policyRow("when spent", "stay on the active account, warn"))
	pol.WriteString(dimStyle.Render("  run 'clalt next' to rotate, or press n here."))

	return titledBox(fmt.Sprintf("ROTATION CHAIN  threshold %.0f%%", m.thresholdPct()), innerW, chainBody.String()) +
		"\n" + titledBox("POLICY", innerW, pol.String())
}

func (m dashModel) thresholdPct() float64 {
	if m.data.Threshold <= 0 {
		return 90
	}
	return m.data.Threshold * 100
}

func policyRow(k, v string) string {
	return dimStyle.Render(padRight("  "+k, 14)) + textStyle.Render(v) + "\n"
}
