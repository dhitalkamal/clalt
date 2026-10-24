package garden

import (
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

func TestFitCabin(t *testing.T) {
	cases := []struct{ n, cols, rows int }{
		{0, 2, 1}, {2, 2, 1}, {3, 2, 2}, {8, 2, 4}, {9, 3, 4}, {32, 8, 4}, {99, 8, 4},
	}
	for _, c := range cases {
		cols, rows := fitCabin(c.n)
		if cols != c.cols || rows != c.rows {
			t.Errorf("fitCabin(%d) = %dx%d, want %dx%d", c.n, cols, rows, c.cols, c.rows)
		}
	}
}

func TestFitPool(t *testing.T) {
	cases := []struct {
		kind string
		n    int
		want [][2]int
	}{
		{"workspace", 8, [][2]int{{4, 1}}},
		{"workspace", 9, [][2]int{{4, 2}}},
		{"workspace", 32, [][2]int{{4, 4}}},
		{"workspace", 40, [][2]int{{4, 3}, {4, 3}}},
		{"research", 4, [][2]int{{2, 1}}},
		{"research", 16, [][2]int{{2, 4}}},
		{"research", 40, [][2]int{{4, 3}, {4, 3}}},
	}
	for _, c := range cases {
		got := fitPool(c.kind, c.n)
		if len(got) != len(c.want) {
			t.Fatalf("fitPool(%s,%d) = %v, want %v", c.kind, c.n, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("fitPool(%s,%d)[%d] = %v, want %v", c.kind, c.n, i, got[i], c.want[i])
			}
		}
	}
}

func TestSizeAndDesks(t *testing.T) {
	cabin := pav("cabin", 2, 1)
	if w, d := size(cabin); w != 4.2 || d != 2.8 {
		t.Errorf("cabin 2x1 = %v x %v", w, d)
	}
	ws := pav("workspace", 4, 1)
	if w, d := size(ws); w < 8.59 || w > 8.61 || d < 3.69 || d > 3.71 {
		t.Errorf("workspace 4x1 = %v x %v", w, d)
	}
	if Desks(cabin) != 2 || Desks(ws) != 8 {
		t.Errorf("desks cabin %d ws %d", Desks(cabin), Desks(ws))
	}
	if w, d := size(pav("treehouse", 0, 0)); w != 9.6 || d != 4.2 {
		t.Errorf("treehouse = %v x %v", w, d)
	}
}

func TestNormalizeRejects(t *testing.T) {
	many := make([]Account, 13)
	for i := range many {
		many[i] = Account{ID: string(rune('a' + i)), Label: "x"}
	}
	bad := []Input{
		{Accounts: many},
		{Accounts: []Account{{ID: "a"}, {ID: "a"}}},
		{Accounts: []Account{{ID: ""}}},
	}
	for i, in := range bad {
		if _, err := normalize(in); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestNormalizeClampsNeeds(t *testing.T) {
	p, err := normalize(Input{Accounts: []Account{{ID: "a", Label: "A"}}, CabinDesks: map[string]int{"a": 0}, WorkspaceDesks: 1, ResearchDesks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if Desks(p.cabins[0]) != 2 || Desks(p.workspace[0]) != 8 || Desks(p.research[0]) != 4 {
		t.Errorf("minimums not applied: %d %d %d", Desks(p.cabins[0]), Desks(p.workspace[0]), Desks(p.research[0]))
	}
}

func pav(typ string, cols, rows int) layout.Pavilion {
	return layout.Pavilion{Type: typ, Cols: cols, Rows: rows}
}
