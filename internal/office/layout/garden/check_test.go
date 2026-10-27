package garden

import (
	"fmt"
	"math"
	"regexp"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

var (
	idRe   = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
	nameRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
)

// reachTol is how far a path end may be from the thing it joins.
const reachTol = 0.3

// problems is a minimal stand-in for the real validator: it returns every
// rule from the plan that the layout breaks, plus a few the page needs.
func problems(l layout.Layout) []string {
	var out []string
	add := func(f string, a ...any) { out = append(out, fmt.Sprintf(f, a...)) }
	if l.V != 2 || !nameRe.MatchString(l.Name) || len(l.Title) > 60 || l.Title == "" {
		add("bad head v=%d name=%q title=%q", l.V, l.Name, l.Title)
	}
	if l.Site == nil {
		return append(out, "no site")
	}
	s := l.Site
	if n := len(s.Base.Poly); n < 3 || n > 512 {
		add("base has %d points", n)
	}
	if n := len(s.Land); n < 1 || n > 16 {
		add("land has %d polys", n)
	}
	if len(s.Ponds) > 8 || len(s.Paths) > 160 || len(s.Bridges) > 16 || len(s.Props) > 512 || len(l.Pavilions) > 64 || len(l.Overflow) > 64 {
		add("a list is over its cap")
	}
	if s.River != nil && (len(s.River.Pts) < 2 || len(s.River.Pts) > 256 || s.River.W < 0.5 || s.River.W > 8) {
		add("bad river")
	}
	for i, p := range s.Ponds {
		if p.RX < 0.5 || p.RX > 10 || p.RZ < 0.5 || p.RZ > 10 {
			add("pond %d radii", i)
		}
	}
	for i, p := range s.Paths {
		if len(p.Pts) < 2 || len(p.Pts) > 256 || p.W < 0.3 || p.W > 4 {
			add("path %d: %d points, w %v", i, len(p.Pts), p.W)
		}
	}
	for i, b := range s.Bridges {
		if n := dist(b.A, b.B); n < 1 || n > 12 {
			add("bridge %d length %v", i, n)
		}
	}
	out = append(out, finiteProblems(l)...)
	out = append(out, pavilionProblems(l)...)
	out = append(out, placementProblems(l)...)
	out = append(out, reachProblems(l)...)
	return out
}

// finiteProblems checks every number is finite and within +-200.
func finiteProblems(l layout.Layout) []string {
	var bad []string
	chk := func(what string, vs ...float64) {
		for _, v := range vs {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 200 {
				bad = append(bad, fmt.Sprintf("%s has value %v", what, v))
				return
			}
		}
	}
	s := l.Site
	for _, p := range s.Base.Poly {
		chk("base", p[0], p[1])
	}
	for _, poly := range s.Land {
		for _, p := range poly.Poly {
			chk("land", p[0], p[1])
		}
	}
	if s.River != nil {
		for _, p := range s.River.Pts {
			chk("river", p[0], p[1])
		}
	}
	for _, p := range s.Paths {
		for _, q := range p.Pts {
			chk("path", q[0], q[1])
		}
	}
	for _, p := range l.Pavilions {
		chk("pavilion "+p.ID, p.X, p.Z, p.Rot)
	}
	for _, p := range l.Overflow {
		chk("overflow", p.X, p.Z, p.Rot)
	}
	for _, p := range s.Props {
		chk("prop", p.X, p.Z, p.Rot)
	}
	return bad
}

// pavilionProblems covers ids, counts, the lobby and the cabin accounts.
func pavilionProblems(l layout.Layout) []string {
	var out []string
	lobbies, tree := 0, 0
	ids := map[string]bool{}
	for _, p := range l.Pavilions {
		if !idRe.MatchString(p.ID) || ids[p.ID] {
			out = append(out, fmt.Sprintf("bad or duplicate id %q", p.ID))
		}
		ids[p.ID] = true
		switch p.Type {
		case "lobby":
			lobbies++
		case "treehouse":
			tree++
		case "cabin":
			if p.Account == "" {
				out = append(out, p.ID+" has no account")
			}
		}
		if layout.HasDesks(p.Type) && (p.Cols < 1 || p.Cols > 8 || p.Rows < 1 || p.Rows > 8) {
			out = append(out, fmt.Sprintf("%s has %dx%d desks", p.ID, p.Cols, p.Rows))
		}
		if len(p.Label) > 40 {
			out = append(out, p.ID+" label too long")
		}
	}
	if lobbies != 1 {
		out = append(out, fmt.Sprintf("%d lobbies", lobbies))
	}
	if tree > 1 {
		out = append(out, fmt.Sprintf("%d treehouses", tree))
	}
	return out
}

// placementProblems: inside the base, no overlaps, not on water, overflow and
// props on dry ground.
func placementProblems(l layout.Layout) []string {
	var out []string
	s := l.Site
	for i, p := range l.Pavilions {
		cs := corners(p)
		onLand := false
		for _, land := range s.Land {
			onLand = onLand || inPoly(p.X, p.Z, land.Poly)
		}
		if !onLand {
			out = append(out, p.ID+" centre is on no land polygon")
		}
		for _, c := range cs {
			if !inPoly(c[0], c[1], s.Base.Poly) {
				out = append(out, fmt.Sprintf("%s corner %v outside the base", p.ID, c))
				break
			}
		}
		for _, q := range l.Pavilions[i+1:] {
			if !polysApart(cs, corners(q), 0.25) {
				out = append(out, fmt.Sprintf("%s and %s overlap or touch", p.ID, q.ID))
			}
		}
		if s.River != nil && nearBoundary(cs, func(x, z float64) bool { return distLine(x, z, s.River.Pts) < s.River.W/2 }) {
			out = append(out, p.ID+" is on the river")
		}
		for _, pd := range s.Ponds {
			if nearBoundary(cs, func(x, z float64) bool { return inEllipse(x, z, pd, 0) }) || distToPav(p, pd.X, pd.Z) == 0 {
				out = append(out, p.ID+" is on a pond")
			}
		}
	}
	for i, o := range l.Overflow {
		if why := dryProblem(l, o.X, o.Z); why != "" {
			out = append(out, fmt.Sprintf("overflow %d: %s", i, why))
		}
	}
	for i, p := range s.Props {
		if why := dryProblem(l, p.X, p.Z); why != "" {
			out = append(out, fmt.Sprintf("prop %d %s: %s", i, p.T, why))
		}
	}
	for i, pa := range s.Paths {
		total := polyLen(pa.Pts)
		for tt := 0.5; tt < total-0.5; tt += 0.25 {
			q, _, _ := arcAt(pa.Pts, tt/total)
			for _, pv := range l.Pavilions {
				if distToPav(pv, q[0], q[1]) < 0.15 {
					out = append(out, fmt.Sprintf("path %d crosses %s", i, pv.ID))
				}
			}
			if s.River != nil && distLine(q[0], q[1], s.River.Pts) < s.River.W/2-0.2 && !onBridge(s.Bridges, q) {
				out = append(out, fmt.Sprintf("path %d wades in the river", i))
			}
		}
	}
	if len(l.Overflow) < 8 {
		out = append(out, fmt.Sprintf("only %d overflow spots", len(l.Overflow)))
	}
	near := map[string]bool{}
	for _, o := range l.Overflow {
		near[o.Near] = true
	}
	for _, n := range []string{"workspace", "cafe", "play", "lobby"} {
		if !near[n] {
			out = append(out, "no overflow spot near "+n)
		}
	}
	return out
}

// dryProblem says why a point cannot hold a prop or a standing agent.
func dryProblem(l layout.Layout, x, z float64) string {
	s := l.Site
	if !inPoly(x, z, s.Base.Poly) {
		return "outside the base"
	}
	for _, p := range l.Pavilions {
		if distToPav(p, x, z) < 0.1 {
			return "inside " + p.ID
		}
	}
	if s.River != nil && distLine(x, z, s.River.Pts) < s.River.W/2 {
		return "in the river"
	}
	for _, pd := range s.Ponds {
		if inEllipse(x, z, pd, 0) {
			return "in a pond"
		}
	}
	return ""
}

// reachProblems builds a graph from the path polylines and bridges and checks
// every pavilion door joins the one holding the lobby door.
func reachProblems(l layout.Layout) []string {
	g := newGraph()
	for _, p := range l.Site.Paths {
		for i := range p.Pts {
			g.add(p.Pts[i])
			if i > 0 {
				g.union(p.Pts[i-1], p.Pts[i])
			}
		}
	}
	for _, b := range l.Site.Bridges {
		g.add(b.A)
		g.add(b.B)
		g.union(b.A, b.B)
	}
	// join points that sit on another polyline's segment (T junctions)
	for _, p := range l.Site.Paths {
		for _, v := range g.pts {
			for i := 0; i+1 < len(p.Pts); i++ {
				if distSeg(v[0], v[1], p.Pts[i], p.Pts[i+1]) <= reachTol {
					g.union(v, p.Pts[i])
				}
			}
		}
	}
	// bridge ends join paths that stop near them
	for _, b := range l.Site.Bridges {
		for _, v := range g.pts {
			if dist(v, b.A) <= reachTol {
				g.union(v, b.A)
			}
			if dist(v, b.B) <= reachTol {
				g.union(v, b.B)
			}
		}
	}
	var gate pt
	for _, p := range l.Pavilions {
		if p.Type == "lobby" {
			gate = door(p)
		}
	}
	var out []string
	gateRoot := g.root(gate, reachTol)
	if gateRoot < 0 {
		return []string{"lobby door is on no path"}
	}
	for _, p := range l.Pavilions {
		r := g.root(door(p), reachTol)
		if r < 0 {
			out = append(out, p.ID+" door is on no path")
		} else if r != gateRoot {
			out = append(out, p.ID+" is cut off from the lobby")
		}
	}
	return out
}

// graph is a small union-find over points.
type graph struct {
	pts    []pt
	parent []int
	index  map[pt]int
}

func newGraph() *graph { return &graph{index: map[pt]int{}} }

func (g *graph) add(p pt) int {
	if i, ok := g.index[p]; ok {
		return i
	}
	g.index[p] = len(g.pts)
	g.pts = append(g.pts, p)
	g.parent = append(g.parent, len(g.parent))
	return len(g.pts) - 1
}

func (g *graph) find(i int) int {
	for g.parent[i] != i {
		g.parent[i] = g.parent[g.parent[i]]
		i = g.parent[i]
	}
	return i
}

func (g *graph) union(a, b pt) { g.parent[g.find(g.add(a))] = g.find(g.add(b)) }

// root is the component of the graph point nearest to p within tol, or -1.
// the point may lie on a segment between vertices, so every vertex within tol
// counts and segments are not needed: callers add T junction unions first.
func (g *graph) root(p pt, tol float64) int {
	best, bi := tol+1, -1
	for i, v := range g.pts {
		if d := dist(p, v); d < best {
			best, bi = d, i
		}
	}
	if bi < 0 || best > tol {
		return -1
	}
	return g.find(bi)
}

func onBridge(bs []layout.Bridge, q pt) bool {
	for _, b := range bs {
		if distSeg(q[0], q[1], b.A, b.B) < b.W {
			return true
		}
	}
	return false
}
