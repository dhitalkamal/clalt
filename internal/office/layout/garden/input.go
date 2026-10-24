package garden

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// Account is one account that gets a cabin. the order is the cabin order.
type Account struct {
	ID    string
	Label string
}

// Input is everything a generator needs: who has cabins and how many desks
// each room type needs.
type Input struct {
	Accounts       []Account
	CabinDesks     map[string]int
	WorkspaceDesks int
	ResearchDesks  int
}

// limits and minimums of the plan.
const (
	MaxAccounts       = 12
	MinCabinDesks     = 2
	MinWorkspaceDesks = 8
	MinResearchDesks  = 4
	maxCabinDesks     = 32
	maxPoolDesks      = 128
	maxLabel          = 40
	maxRowsPerPav     = 4
)

// plan is the pavilions to place, sized but not yet positioned.
type plan struct {
	title     string
	cabins    []layout.Pavilion
	research  []layout.Pavilion
	workspace []layout.Pavilion
}

// fitCabin picks cols and rows for a cabin: two desks wide, up to four rows,
// then wider.
func fitCabin(n int) (cols, rows int) {
	n = max(MinCabinDesks, min(n, maxCabinDesks))
	cols, rows = 2, (n+1)/2
	if rows > maxRowsPerPav {
		rows, cols = maxRowsPerPav, (n+maxRowsPerPav-1)/maxRowsPerPav
	}
	return cols, rows
}

// fitPool splits n desks of a shared room type into pavilions of at most four
// rows each. desks round up to whole rows.
func fitPool(kind string, n int) [][2]int {
	cols := 4
	if kind == "research" && n <= 2*maxRowsPerPav*2 {
		cols = 2
	}
	per := cols * maxRowsPerPav * 2
	k := (n + per - 1) / per
	each := (n + k - 1) / k
	rows := (each + cols*2 - 1) / (cols * 2)
	out := make([][2]int, k)
	for i := range out {
		out[i] = [2]int{cols, rows}
	}
	return out
}

// normalize checks the input and turns it into sized pavilions.
func normalize(in Input) (plan, error) {
	if len(in.Accounts) > MaxAccounts {
		return plan{}, fmt.Errorf("garden: %d accounts, at most %d", len(in.Accounts), MaxAccounts)
	}
	var p plan
	seen := map[string]bool{}
	for i, a := range in.Accounts {
		if a.ID == "" || len(a.ID) > 64 {
			return plan{}, errors.New("garden: account id must be 1 to 64 characters")
		}
		if seen[a.ID] {
			return plan{}, fmt.Errorf("garden: duplicate account id %q", a.ID)
		}
		seen[a.ID] = true
		cols, rows := fitCabin(in.CabinDesks[a.ID])
		p.cabins = append(p.cabins, layout.Pavilion{
			ID: fmt.Sprintf("cabin-%d", i+1), Type: "cabin", Account: a.ID,
			Label: cleanLabel(a.Label, a.ID, i+1), Cols: cols, Rows: rows,
		})
	}
	p.workspace = pool("workspace", max(MinWorkspaceDesks, min(in.WorkspaceDesks, maxPoolDesks)))
	p.research = pool("research", max(MinResearchDesks, min(in.ResearchDesks, maxPoolDesks)))
	return p, nil
}

func pool(kind string, n int) []layout.Pavilion {
	var out []layout.Pavilion
	for i, cr := range fitPool(kind, n) {
		out = append(out, layout.Pavilion{ID: fmt.Sprintf("%s-%d", kind, i+1), Type: kind, Cols: cr[0], Rows: cr[1]})
	}
	return out
}

// cleanLabel cleans a label the way layouts do and cuts it to the label
// limit. when the result still breaks the text rule (a url look) it falls
// back to the account id, then to "cabin N".
func cleanLabel(label, id string, n int) string {
	for _, c := range []string{label, id} {
		c = strings.TrimSpace(layout.CleanText(c, maxLabel))
		if c != "" && layout.CheckLabel(c) == nil {
			return c
		}
	}
	return fmt.Sprintf("cabin %d", n)
}
