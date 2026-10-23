package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// the percent next to an account is local tokens over a fixed 2,000,000, not a
// share of the plan's limit, so it is shown as the estimate it is
func TestTheFiveHourPercentIsMarkedAsAnEstimate(t *testing.T) {
	m := dashModel{data: sampleData(), width: 120, height: 40, focus: focusAccounts}
	for name, want := range map[string]string{"work": "~17%", "casey": "~35%", "jordan": "~95%"} {
		var row string
		for _, a := range m.data.Accounts {
			if a.Name == name {
				row = m.accountRow(a, false, false, 30)
			}
		}
		if !strings.Contains(row, want) {
			t.Errorf("%s: want %s in the row, got %q", name, want, row)
		}
	}
}

// a rate past the reference is shown as 100 and still fits the row, whatever
// the width of the number
func TestAnAccountOverTheReferenceStillFitsItsRow(t *testing.T) {
	m := dashModel{data: sampleData(), width: 120, height: 40, focus: focusAccounts}
	for _, tokens := range []int64{0, 70_000, 340_000, 1_900_000, 2_000_000, 9_000_000} {
		a := AccountView{Name: "a-long-name-that-has-to-be-cut", Five: usage.WindowStat{Tokens: tokens}}
		for _, innerW := range []int{20, 30, 44} {
			for _, selected := range []bool{false, true} {
				row := m.accountRow(a, selected, true, innerW)
				if got := lipgloss.Width(row); got != innerW {
					t.Errorf("%d tokens, innerW %d, selected %v: the row is %d wide: %q", tokens, innerW, selected, got, row)
				}
			}
		}
	}
	over := AccountView{Name: "x", Five: usage.WindowStat{Tokens: 9_000_000}}
	if row := m.accountRow(over, false, false, 30); !strings.Contains(row, "~100%") {
		t.Errorf("an account over the reference is shown as 100: %q", row)
	}
}
