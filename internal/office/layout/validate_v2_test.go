package layout

import (
	"math"
	"strings"
	"testing"
)

func TestValid2Passes(t *testing.T) {
	if err := Validate(valid2(), nil); err != nil {
		t.Fatal(err)
	}
	l := valid2()
	l.Template = "river"
	if err := Validate(l, nil); err != nil {
		t.Fatalf("a generated layout names its template: %v", err)
	}
}

func TestValidateV2Rules(t *testing.T) {
	long := strings.Repeat("a", 65)
	cases := []struct {
		name string
		mod  func(l *Layout)
		want string
	}{
		{"bad name", func(l *Layout) { l.Name = "Garden" }, "name"},
		{"long title", func(l *Layout) { l.Title = strings.Repeat("a", 61) }, "title"},
		{"url title", func(l *Layout) { l.Title = "see http://x" }, "url"},
		{"template unknown", func(l *Layout) { l.Template = "hall" }, "template"},
		{"no site", func(l *Layout) { l.Site = nil }, "site"},
		{"v1 field size", func(l *Layout) { l.Size = Size{W: 20, D: 20} }, "version 1"},
		{"v1 field rooms", func(l *Layout) { l.Rooms = []Room{{ID: "a"}} }, "version 1"},
		{"v1 field grid", func(l *Layout) { l.Grid = 0.5 }, "version 1"},
		{"base few points", func(l *Layout) { l.Site.Base.Poly = square(20)[:2] }, "base"},
		{"base many points", func(l *Layout) { l.Site.Base.Poly = make([]Pt, 513) }, "base"},
		{"base out of range", func(l *Layout) { l.Site.Base.Poly[0] = Pt{201, 0} }, "range"},
		{"base nan", func(l *Layout) { l.Site.Base.Poly[0] = Pt{math.NaN(), 0} }, "finite"},
		{"base inf", func(l *Layout) { l.Site.Base.Poly[1] = Pt{0, math.Inf(1)} }, "finite"},
		{"no land", func(l *Layout) { l.Site.Land = nil }, "land"},
		{"land many", func(l *Layout) { l.Site.Land = make([]Poly, 17) }, "land"},
		{"land short poly", func(l *Layout) { l.Site.Land[0].Poly = square(5)[:2] }, "land"},
		{"river one point", func(l *Layout) { l.Site.River.Pts = []Pt{{1, 1}} }, "river"},
		{"river thin", func(l *Layout) { l.Site.River.W = 0.4 }, "river"},
		{"river wide", func(l *Layout) { l.Site.River.W = 8.1 }, "river"},
		{"river many points", func(l *Layout) { l.Site.River.Pts = make([]Pt, 257) }, "river"},
		{"pond count", func(l *Layout) { l.Site.Ponds = make([]Pond, 9) }, "pond"},
		{"pond small", func(l *Layout) { l.Site.Ponds[0].RX = 0.4 }, "pond"},
		{"pond big", func(l *Layout) { l.Site.Ponds[0].RZ = 10.1 }, "pond"},
		{"pond far", func(l *Layout) { l.Site.Ponds[0].X = 1e300 }, "range"},
		{"path count", func(l *Layout) { l.Site.Paths = make([]Path, 161) }, "path"},
		{"path one point", func(l *Layout) { l.Site.Paths[0].Pts = []Pt{{0, 0}} }, "path"},
		{"path width", func(l *Layout) { l.Site.Paths[0].W = 4.1 }, "path"},
		{"path thin", func(l *Layout) { l.Site.Paths[0].W = 0.2 }, "path"},
		{"bridge count", func(l *Layout) { l.Site.Bridges = make([]Bridge, 17) }, "bridge"},
		{"bridge short", func(l *Layout) { l.Site.Bridges[0].B = Pt{9.5, 0} }, "bridge"},
		{"bridge long", func(l *Layout) { l.Site.Bridges[0].B = Pt{22, 0} }, "bridge"},
		{"bridge wide", func(l *Layout) { l.Site.Bridges[0].W = 5 }, "bridge"},
		{"prop count", func(l *Layout) { l.Site.Props = make([]Prop, 513) }, "prop"},
		{"prop type", func(l *Layout) { l.Site.Props[0].T = "ufo" }, "prop"},
		{"prop size on bench", func(l *Layout) { l.Site.Props[0].W = 1 }, "prop"},
		{"bed without size", func(l *Layout) { l.Site.Props[1].W = 0 }, "prop"},
		{"bed huge", func(l *Layout) { l.Site.Props[1].D = 50 }, "prop"},
		{"prop rot", func(l *Layout) { l.Site.Props[0].Rot = 400 }, "range"},
		{"trees mode", func(l *Layout) { l.Site.Trees.Mode = "forest" }, "trees"},
		{"trees band", func(l *Layout) { l.Site.Trees.Band = -1 }, "trees"},
		{"trees seed", func(l *Layout) { l.Site.Trees.Seed = -1 }, "trees"},
		{"no lobby", func(l *Layout) { l.Pavilions = l.Pavilions[1:] }, "lobby"},
		{"two lobbies", func(l *Layout) { l.Pavilions[2].Type = "lobby"; l.Pavilions[2].Cols, l.Pavilions[2].Rows = 0, 0 }, "lobby"},
		{"two treehouses", func(l *Layout) {
			l.Pavilions = append(l.Pavilions,
				Pavilion{ID: "t1", Type: "treehouse", X: 0, Z: 10}, Pavilion{ID: "t2", Type: "treehouse", X: 0, Z: 15})
		}, "treehouse"},
		{"too many pavilions", func(l *Layout) { l.Pavilions = append(l.Pavilions, make([]Pavilion, 62)...) }, "pavilions"},
		{"pavilion type", func(l *Layout) { l.Pavilions[1].Type = "castle" }, "type"},
		{"pavilion id", func(l *Layout) { l.Pavilions[1].ID = "P 1" }, "id"},
		{"dup id", func(l *Layout) { l.Pavilions[1].ID = "lobby" }, "duplicate"},
		{"cabin no account", func(l *Layout) { l.Pavilions[1].Account = "" }, "account"},
		{"bad account", func(l *Layout) { l.Pavilions[1].Account = "a/b" }, "account"},
		{"account on workspace", func(l *Layout) { l.Pavilions[2].Account = "work" }, "account"},
		{"long label", func(l *Layout) { l.Pavilions[2].Label = strings.Repeat("a", 41) }, "label"},
		{"url label", func(l *Layout) { l.Pavilions[2].Label = "www.x.com" }, "url"},
		{"cols zero", func(l *Layout) { l.Pavilions[1].Cols = 0 }, "cols"},
		{"rows big", func(l *Layout) { l.Pavilions[1].Rows = 9 }, "rows"},
		{"cols on lobby", func(l *Layout) { l.Pavilions[0].Cols = 1 }, "cols"},
		{"pavilion out of range", func(l *Layout) { l.Pavilions[0].X = 250 }, "range"},
		{"pavilion rot nan", func(l *Layout) { l.Pavilions[0].Rot = math.NaN() }, "finite"},
		{"outside base", func(l *Layout) { l.Pavilions[0].X = 19.5 }, "base"},
		{"corner outside rotated", func(l *Layout) { l.Pavilions[0].X, l.Pavilions[0].Rot = 18.5, 45 }, "base"},
		{"overlap pavilions", func(l *Layout) { l.Pavilions[2].X, l.Pavilions[2].Z = -10, -9 }, "overlap"},
		{"overlap on river", func(l *Layout) { l.Pavilions[0].X, l.Pavilions[0].Z = 11, 0 }, "river"},
		{"overlap on pond", func(l *Layout) { l.Pavilions[0].X, l.Pavilions[0].Z = 0, 9 }, "pond"},
		{"overflow count", func(l *Layout) { l.Overflow = make([]Spot, 65) }, "overflow"},
		{"overflow near", func(l *Layout) { l.Overflow[0].Near = "moon" }, "overflow"},
		{"overflow range", func(l *Layout) { l.Overflow[0].X = -201 }, "range"},
		{"eye nan", func(l *Layout) { l.Eye.From = Pt{math.NaN(), 0} }, "finite"},
		{"eye range", func(l *Layout) { l.Eye.Look = Pt{0, 300} }, "range"},
		{"cabin account long", func(l *Layout) { l.Pavilions[1].Account = long }, "account"},
	}
	for _, c := range cases {
		l := valid2()
		c.mod(&l)
		err := Validate(l, nil)
		if err == nil {
			t.Errorf("%s: layout passed", c.name)
		} else if !strings.Contains(strings.ToLower(err.Error()), c.want) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.want)
		}
	}
}

func TestV1RefusesV2Fields(t *testing.T) {
	l := valid()
	l.Template = "rows"
	if err := Validate(l, testCatalog()); err == nil {
		t.Error("a v1 layout with a template passed")
	}
	l = valid()
	l.Site = &Site{}
	if err := Validate(l, testCatalog()); err == nil {
		t.Error("a v1 layout with a site passed")
	}
}

func TestPavilionOnBoundaryAndRiverEdge(t *testing.T) {
	// a lobby is 4.0 deep: centre -18 puts its edge at exactly -20, which is on the outline
	l := valid2()
	l.Pavilions[0].X, l.Pavilions[0].Z = 0, -18
	if err := Validate(l, nil); err != nil {
		t.Errorf("a pavilion touching the outline should pass: %v", err)
	}
	// river band is 10.5 to 13.5; a lobby 3.8 wide centred at 8.5 ends at 10.4
	l = valid2()
	l.Pavilions[0].X, l.Pavilions[0].Z = 8.5, 0
	if err := Validate(l, nil); err != nil {
		t.Errorf("a pavilion clear of the river band should pass: %v", err)
	}
	l.Pavilions[0].X = 8.7
	if err := Validate(l, nil); err == nil || !strings.Contains(err.Error(), "river") {
		t.Errorf("a pavilion in the river band passed: %v", err)
	}
}

func TestRotatedOverlap(t *testing.T) {
	// two lobbies 3.8 x 4.0: side by side at 4.0 apart is clear, at 3.7 it overlaps
	l := valid2()
	l.Pavilions = append(l.Pavilions, Pavilion{ID: "cafe", Type: "cafe", X: 5, Z: 5})
	l.Pavilions[3].Rot = 45
	// cafe rotated 45 reaches (5.6+4.0)/2*0.707 = 3.39 from its centre, workspace at -10,5 is far
	if err := Validate(l, nil); err != nil {
		t.Fatal(err)
	}
	l.Pavilions[3].X, l.Pavilions[3].Z = -6.3, 5
	if err := Validate(l, nil); err == nil {
		t.Error("overlap of a rotated pavilion was missed")
	}
}

func TestRiverBendBetweenSegments(t *testing.T) {
	l := valid2()
	l.Site.River.Pts = []Pt{{0, -18}, {0, 0}, {10, 0}}
	l.Pavilions = []Pavilion{{ID: "lobby", Type: "lobby", X: -3.5, Z: 3.5}}
	if err := Validate(l, nil); err != nil {
		t.Errorf("clear of the bend: %v", err)
	}
	// corner at (-1.1, 1.0) is 1.49 from the bend, inside the 1.5 half width and only reachable by the round joint
	l.Pavilions[0].X, l.Pavilions[0].Z = -3.0, 3.0
	if err := Validate(l, nil); err == nil {
		t.Error("pavilion in the joint of the river passed")
	}
	l.Pavilions[0].X, l.Pavilions[0].Z = -2, -2
	if err := Validate(l, nil); err == nil {
		t.Error("pavilion across the first leg of the river passed")
	}
}
