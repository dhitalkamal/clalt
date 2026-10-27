package garden

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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
	TeamIDs        []string
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
	MaxTeam           = 24
	teamPerPav        = 8
)

// plan is the pavilions to place, sized but not yet positioned.
type plan struct {
	title     string
	cabins    []layout.Pavilion
	research  []layout.Pavilion
	workspace []layout.Pavilion
	team      []layout.Pavilion
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
	team, err := teamPavilions(in.TeamIDs)
	if err != nil {
		return plan{}, err
	}
	p.team = team
	p.workspace = pool("workspace", max(MinWorkspaceDesks, min(in.WorkspaceDesks, maxPoolDesks)))
	p.research = pool("research", max(MinResearchDesks, min(in.ResearchDesks, maxPoolDesks)))
	return p, nil
}

var teamIDRe = regexp.MustCompile(`^[a-z0-9:-]{1,64}$`)

// teamPavilions checks the agent ids and groups them, eight to a pavilion in
// the given order. a pavilion of m members has two columns and enough rows
// for one desk each, so the last one can be smaller.
func teamPavilions(ids []string) ([]layout.Pavilion, error) {
	if len(ids) > MaxTeam {
		return nil, fmt.Errorf("garden: %d team members, at most %d", len(ids), MaxTeam)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !teamIDRe.MatchString(id) {
			return nil, fmt.Errorf("garden: team id %q must be 1 to 64 characters of a-z, 0-9, - and :", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("garden: duplicate team id %q", id)
		}
		seen[id] = true
	}
	var out []layout.Pavilion
	for i := 0; i < len(ids); i += teamPerPav {
		m := slices.Clone(ids[i:min(i+teamPerPav, len(ids))])
		out = append(out, layout.Pavilion{
			ID: fmt.Sprintf("team-%d", len(out)+1), Type: "team",
			Cols: 2, Rows: (len(m) + 3) / 4, Members: m,
		})
	}
	return out, nil
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
