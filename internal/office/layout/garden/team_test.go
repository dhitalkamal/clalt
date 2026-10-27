package garden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// teamIDs makes n distinct agent ids.
func teamIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("agent-%d", i+1)
	}
	return ids
}

func TestGenerateTeamTable(t *testing.T) {
	for _, tpl := range templates {
		for n := 0; n <= 12; n++ {
			for _, m := range []int{0, 5, 9, 24} {
				t.Run(fmt.Sprintf("%s/%d/%d", tpl, n, m), func(t *testing.T) {
					t.Parallel()
					in := inputFor(n, needSets[1])
					in.TeamIDs = teamIDs(m)
					l, err := Generate(tpl, in)
					if err != nil {
						t.Fatal(err)
					}
					for _, p := range problems(l) {
						t.Error(p)
					}
					if err := layout.Validate(l, nil); err != nil {
						t.Errorf("Validate: %v", err)
					}
					var got []string
					pavs := 0
					for _, p := range l.Pavilions {
						if p.Type != "team" {
							continue
						}
						pavs++
						if len(p.Members) > Desks(p) || len(p.Members) == 0 {
							t.Errorf("%s: %d members on %d desks", p.ID, len(p.Members), Desks(p))
						}
						got = append(got, p.Members...)
					}
					if want := (m + 7) / 8; pavs != want {
						t.Errorf("%d team pavilions, want %d", pavs, want)
					}
					if !slices.Equal(got, in.TeamIDs) && !(m == 0 && len(got) == 0) {
						t.Errorf("members %v, want %v in order", got, in.TeamIDs)
					}
					l2, _ := Generate(tpl, in)
					a, _ := json.Marshal(l)
					b, _ := json.Marshal(l2)
					if string(a) != string(b) {
						t.Error("not deterministic")
					}
				})
			}
		}
	}
}

// TestGenerateTeamSizes covers the pavilion edges (1, 8, 14, 16 members) for
// three accounts: the full account sweep above only samples member counts.
func TestGenerateTeamSizes(t *testing.T) {
	for _, tpl := range templates {
		for _, m := range []int{1, 7, 8, 14, 16, 17} {
			t.Run(fmt.Sprintf("%s/%d", tpl, m), func(t *testing.T) {
				t.Parallel()
				in := three()
				in.TeamIDs = teamIDs(m)
				l, err := Generate(tpl, in)
				if err != nil {
					t.Fatal(err)
				}
				for _, p := range problems(l) {
					t.Error(p)
				}
				if err := layout.Validate(l, nil); err != nil {
					t.Errorf("Validate: %v", err)
				}
			})
		}
	}
}

func TestGenerateTeamInputErrors(t *testing.T) {
	cases := map[string][]string{
		"upper case": {"Explore"},
		"empty":      {""},
		"space":      {"a b"},
		"too long":   {strings.Repeat("a", 65)},
		"duplicate":  {"a", "a"},
		"too many":   teamIDs(MaxTeam + 1),
	}
	for name, ids := range cases {
		in := three()
		in.TeamIDs = ids
		if _, err := Generate("rows", in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestTrackerTeamChangeAppliesAtOnce(t *testing.T) {
	tr := NewTracker()
	now := t0
	acc := []Account{{ID: "a", Label: "A"}}
	tr.Observe(now, acc, Needs{TeamIDs: []string{"explore"}})
	_, rev1, err := tr.Layout("rows")
	if err != nil {
		t.Fatal(err)
	}
	// same list again: nothing moves
	tr.Observe(now, acc, Needs{TeamIDs: []string{"explore"}})
	if _, rev, _ := tr.Layout("rows"); rev != rev1 {
		t.Error("rev moved without a team change")
	}
	// one more member, same instant, no hold
	tr.Observe(now, acc, Needs{TeamIDs: []string{"explore", "plan"}})
	l, rev2, _ := tr.Layout("rows")
	if rev2 == rev1 {
		t.Fatal("team change did not change the layout")
	}
	found := false
	for _, p := range l.Pavilions {
		if p.Type == "team" && slices.Equal(p.Members, []string{"explore", "plan"}) {
			found = true
		}
	}
	if !found {
		t.Error("new members not in the team pavilion")
	}
}

// TestWriteTeamFixtures writes the generated team layouts the web tests use
// into the directory named by GARDEN_TEAM_FIXTURES_DIR. it does nothing
// otherwise.
func TestWriteTeamFixtures(t *testing.T) {
	dir := os.Getenv("GARDEN_TEAM_FIXTURES_DIR")
	if dir == "" {
		t.Skip("GARDEN_TEAM_FIXTURES_DIR not set")
	}
	names := []string{
		"general-purpose", "explore", "plan", "pr-review-toolkit:code-reviewer", "silent-failure-hunter",
		"backend-architect", "frontend-developer", "go-helper", "test-runner", "doc-writer",
		"security-auditor", "db-design", "release-notes", "perf-engineer",
	}
	for _, tpl := range templates {
		for _, m := range []int{5, 14} {
			in := three()
			in.TeamIDs = names[:m]
			l, err := Generate(tpl, in)
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(l)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("gen-team-%s-%d.json", tpl, m)), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
