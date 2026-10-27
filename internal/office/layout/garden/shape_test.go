package garden

import (
	"math"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

func countType(l layout.Layout, typ string) int {
	n := 0
	for _, p := range l.Pavilions {
		if p.Type == typ {
			n++
		}
	}
	return n
}

func baseArea(l layout.Layout) float64 {
	a, p := 0.0, l.Site.Base.Poly
	for i := range p {
		q := p[(i+1)%len(p)]
		a += p[i][0]*q[1] - q[0]*p[i][1]
	}
	return math.Abs(a) / 2
}

func TestThreeAccountCampus(t *testing.T) {
	for _, tpl := range templates {
		l, err := Generate(tpl, three())
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"lobby": 1, "treehouse": 1, "cafe": 1, "play": 1, "research": 1, "workspace": 1, "cabin": 3}
		for typ, n := range want {
			if got := countType(l, typ); got != n {
				t.Errorf("%s: %d %s pavilions, want %d", tpl, got, typ, n)
			}
		}
		var labels []string
		for _, p := range l.Pavilions {
			if p.Type == "cabin" {
				labels = append(labels, p.Account+":"+p.Label)
			}
		}
		if len(labels) != 3 || labels[0] != "work:WORK" || labels[1] != "personal:PERSONAL" || labels[2] != "client:CLIENT" {
			t.Errorf("%s: cabins %v", tpl, labels)
		}
		if (tpl == "river") != (l.Site.River != nil) {
			t.Errorf("%s: river presence wrong", tpl)
		}
		if tpl == "river" && len(l.Site.Bridges) < 3 {
			t.Errorf("river: %d bridges, want at least 3", len(l.Site.Bridges))
		}
		if tpl == "village" && len(l.Site.Ponds) != 1 {
			t.Errorf("village: %d ponds", len(l.Site.Ponds))
		}
		fountains, benches, lamps := 0, 0, 0
		for _, p := range l.Site.Props {
			switch p.T {
			case "fountain":
				fountains++
			case "bench":
				benches++
			case "lamp":
				lamps++
			}
		}
		if (tpl == "street") != (fountains == 1) || benches == 0 || lamps == 0 {
			t.Errorf("%s: fountains %d benches %d lamps %d", tpl, fountains, benches, lamps)
		}
	}
}

func TestCampusGrowsWithAccounts(t *testing.T) {
	for _, tpl := range templates {
		area := map[int]float64{}
		for _, n := range []int{0, 3, 8, 12} {
			l, err := Generate(tpl, inputFor(n, [3]int{2, 8, 4}))
			if err != nil {
				t.Fatal(err)
			}
			area[n] = baseArea(l)
		}
		// not strictly monotone: a ring or row wrap can reshuffle in between
		if !(area[3] <= area[8] && area[3] < area[12]) {
			t.Errorf("%s: island areas %v do not grow", tpl, area)
		}
	}
}

func TestBigNeedsAddWorkspacePavilions(t *testing.T) {
	for _, tpl := range templates {
		l, err := Generate(tpl, inputFor(2, [3]int{2, 40, 4}))
		if err != nil {
			t.Fatal(err)
		}
		if got := countType(l, "workspace"); got != 2 {
			t.Errorf("%s: %d workspace pavilions for 40 desks, want 2", tpl, got)
		}
	}
}

func TestOutputIsPlainJSON(t *testing.T) {
	for _, tpl := range templates {
		l, err := Generate(tpl, inputFor(12, [3]int{8, 40, 40}))
		if err != nil {
			t.Fatal(err)
		}
		b, err := layout.Encode(l)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) > layout.MaxBytes {
			t.Errorf("%s: %d bytes, cap %d", tpl, len(b), layout.MaxBytes)
		}
		for _, c := range b {
			if c > 126 {
				t.Fatalf("%s: non ascii byte in output", tpl)
			}
		}
		if _, err := layout.Decode(b); err != nil {
			t.Errorf("%s: %v", tpl, err)
		}
	}
}
