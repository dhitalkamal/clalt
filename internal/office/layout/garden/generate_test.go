package garden

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

var templates = []string{"rows", "village", "river", "street"}

// needs sets the desk needs of one table row: cabin, workspace, research.
var needSets = [][3]int{{2, 8, 4}, {4, 16, 8}, {8, 24, 12}, {40, 40, 40}}

func inputFor(n int, n3 [3]int) Input {
	in := Input{WorkspaceDesks: n3[1], ResearchDesks: n3[2], CabinDesks: map[string]int{}}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("acct%d", i)
		in.Accounts = append(in.Accounts, Account{ID: id, Label: fmt.Sprintf("ACCOUNT %d", i)})
		in.CabinDesks[id] = n3[0]
	}
	return in
}

func three() Input {
	return Input{
		Accounts:       []Account{{"work", "WORK"}, {"personal", "PERSONAL"}, {"client", "CLIENT"}},
		CabinDesks:     map[string]int{"work": 2, "personal": 2, "client": 2},
		WorkspaceDesks: 8,
		ResearchDesks:  4,
	}
}

func TestGenerateTable(t *testing.T) {
	for _, tpl := range templates {
		for n := 0; n <= 12; n++ {
			for _, ns := range needSets {
				// the 40 desk extreme is slow on big islands: sample it
				if ns[0] == 40 && n != 0 && n != 1 && n != 5 && n != 12 {
					continue
				}
				t.Run(fmt.Sprintf("%s/%d/%v", tpl, n, ns), func(t *testing.T) {
					t.Parallel()
					in := inputFor(n, ns)
					l, err := Generate(tpl, in)
					if err != nil {
						t.Fatal(err)
					}
					if l.Name != tpl || l.Template != tpl {
						t.Errorf("name %q template %q", l.Name, l.Template)
					}
					for _, p := range problems(l) {
						t.Error(p)
					}
					if err := layout.Validate(l, nil); err != nil {
						t.Errorf("Validate: %v", err)
					}
					cabins := 0
					desks := map[string]int{}
					for _, p := range l.Pavilions {
						if p.Type == "cabin" {
							cabins++
						}
						desks[p.Type] += Desks(p)
					}
					if cabins != n {
						t.Errorf("%d cabins, want %d", cabins, n)
					}
					if desks["workspace"] < max(8, ns[1]) || desks["research"] < max(4, ns[2]) {
						t.Errorf("desks short: %v", desks)
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

func TestGenerateErrors(t *testing.T) {
	if _, err := Generate("hall", three()); err == nil {
		t.Error("unknown template accepted")
	}
	if _, err := Generate("rows", inputFor(13, needSets[0])); err == nil {
		t.Error("13 accounts accepted")
	}
}

func TestEyeAndTitle(t *testing.T) {
	for _, tpl := range templates {
		l, err := Generate(tpl, three())
		if err != nil {
			t.Fatal(err)
		}
		if l.Eye == nil || l.Title == "" || l.Site.Trees.Mode != "edge" || !l.Site.Fence {
			t.Errorf("%s: eye/title/trees/fence missing", tpl)
		}
	}
}

// TestGenerateMixedNeeds covers uneven needs: each account asks for a
// different cabin size and the pools are not round numbers.
func TestGenerateMixedNeeds(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, tpl := range templates {
		for i := 0; i < 12; i++ {
			n := rng.Intn(13)
			in := Input{WorkspaceDesks: 1 + rng.Intn(40), ResearchDesks: rng.Intn(41), CabinDesks: map[string]int{}}
			for a := 0; a < n; a++ {
				id := fmt.Sprintf("a%d", a)
				in.Accounts = append(in.Accounts, Account{ID: id, Label: id})
				in.CabinDesks[id] = rng.Intn(41)
			}
			t.Run(fmt.Sprintf("%s/%d", tpl, i), func(t *testing.T) {
				t.Parallel()
				l, err := Generate(tpl, in)
				if err != nil {
					t.Fatalf("%v with %+v", err, in)
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
