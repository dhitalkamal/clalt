package garden

import (
	"math"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

const v2 = 2

// scene collects one layout while a template builds it, in a local frame that
// finish() recentres on the origin.
type scene struct {
	name, title string
	pavs        []layout.Pavilion
	base        []pt
	land        [][]pt // grass polygons, the base when empty
	river       *layout.River
	ponds       []layout.Pond
	paths       []layout.Path
	bridges     []layout.Bridge
	props       []layout.Prop
	spots       []layout.Spot
	treeBand    float64
	treeSeed    int
	eye         *layout.Eye
	// placed props and spots, so later ones keep their distance
	taken []pt
}

func newScene(name, title string, band float64, seed int) *scene {
	return &scene{name: name, title: title, treeBand: band, treeSeed: seed}
}

func (s *scene) pav(p layout.Pavilion) { s.pavs = append(s.pavs, p) }

func (s *scene) find(id string) layout.Pavilion {
	for _, p := range s.pavs {
		if p.ID == id {
			return p
		}
	}
	return layout.Pavilion{}
}

func (s *scene) ofType(t string) []layout.Pavilion {
	var out []layout.Pavilion
	for _, p := range s.pavs {
		if p.Type == t {
			out = append(out, p)
		}
	}
	return out
}

// path adds a polyline of width w.
func (s *scene) path(w float64, smooth bool, pts ...pt) {
	s.paths = append(s.paths, layout.Path{Pts: pts, W: w, Smooth: smooth})
}

// spur joins a pavilion door straight to a point.
func (s *scene) spur(p layout.Pavilion, to pt) { s.path(0.8, false, door(p), to) }

// free says whether a disc of radius r at (x, z) sits on dry, empty ground.
// pathClear also keeps it off the walkways.
func (s *scene) free(x, z, r float64, pathClear bool) bool {
	if !inPoly(x, z, s.base) || distPoly(x, z, s.base) < 0.8+r {
		return false
	}
	for _, p := range s.pavs {
		if distToPav(p, x, z) < r+0.3 {
			return false
		}
	}
	if s.river != nil && distLine(x, z, s.river.Pts) < s.river.W/2+r+0.2 {
		return false
	}
	for _, p := range s.ponds {
		if inEllipse(x, z, p, r+0.2) {
			return false
		}
	}
	for _, b := range s.bridges {
		if distSeg(x, z, b.A, b.B) < b.W/2+r {
			return false
		}
	}
	if pathClear {
		for _, p := range s.paths {
			if distLine(x, z, p.Pts) < p.W/2+r {
				return false
			}
		}
	}
	for _, t := range s.taken {
		if dist(t, pt{x, z}) < r+0.45 {
			return false
		}
	}
	return true
}

// near finds the free spot closest to an anchor, searching outward in rings.
func (s *scene) near(ax, az, r float64, pathClear bool) (pt, bool) {
	for ring := 0.0; ring <= 30; ring += 0.5 {
		n := max(1, int(ring*5))
		for k := 0; k < n; k++ {
			a := 2 * math.Pi * float64(k) / float64(n)
			x, z := ax+ring*math.Cos(a), az+ring*math.Sin(a)
			if s.free(x, z, r, pathClear) {
				return pt{x, z}, true
			}
		}
	}
	return pt{}, false
}

// prop puts a prop near an anchor when there is room.
func (s *scene) prop(t string, ax, az, rot, r float64) {
	if p, ok := s.near(ax, az, r, true); ok {
		s.props = append(s.props, layout.Prop{T: t, X: p[0], Z: p[1], Rot: rot})
		s.taken = append(s.taken, p)
	}
}

// bed puts a flower bed of w by d near an anchor, checking along its length.
func (s *scene) bed(ax, az, w, d float64) {
	long, short := math.Max(w, d), math.Min(w, d)
	for ring := 0.0; ring <= 30; ring += 0.5 {
		n := max(1, int(ring*5))
		for k := 0; k < n; k++ {
			a := 2 * math.Pi * float64(k) / float64(n)
			x, z := ax+ring*math.Cos(a), az+ring*math.Sin(a)
			ok := true
			for u := -long/2 + short/2; u <= long/2-short/2+1e-9 && ok; u += 0.4 {
				px, pz := x, z
				if w > d {
					px += u
				} else {
					pz += u
				}
				ok = s.free(px, pz, short/2+0.1, true)
			}
			if ok {
				s.props = append(s.props, layout.Prop{T: "bed", X: x, Z: z, W: w, D: d})
				s.taken = append(s.taken, pt{x, z})
				return
			}
		}
	}
}

// spotsAt adds up to n standing spots in front of a pavilion, either side of
// its door, facing it.
func (s *scene) spotsAt(p layout.Pavilion, near string, n int) {
	w, d := size(p)
	added := 0
	for _, off := range []float64{1.5, -1.5, 3.0, -3.0, 4.4, -4.4} {
		if added == n || abs(off) > w/2-0.4 {
			continue
		}
		q := rotPt(p.X, p.Z, p.Rot, off, d/2+0.8)
		if s.free(q[0], q[1], 0.35, false) {
			s.spot(q, normDeg(p.Rot+180), near)
			added++
		}
	}
}

func (s *scene) spot(q pt, rot float64, near string) {
	s.spots = append(s.spots, layout.Spot{X: q[0], Z: q[1], Rot: rot, Near: near})
	s.taken = append(s.taken, q)
}

// topUpSpots makes sure there are at least 8 spots, spreading extras over the
// areas the plan names.
func (s *scene) topUpSpots() {
	order := []string{"workspace", "cafe", "play", "lobby"}
	for i := 0; len(s.spots) < 8 && i < 64; i++ {
		near := order[i%len(order)]
		ps := s.ofType(near)
		if len(ps) == 0 {
			continue
		}
		p := ps[0]
		q, ok := s.near(p.X, p.Z, 0.35, false)
		if ok {
			s.spot(q, normDeg(p.Rot+180), near)
		}
	}
}

// addSpots places the standard overflow spots of a campus.
func (s *scene) addSpots() {
	for _, p := range s.ofType("workspace") {
		s.spotsAt(p, "workspace", 4)
	}
	for _, t := range []string{"cafe", "play", "lobby"} {
		for _, p := range s.ofType(t) {
			s.spotsAt(p, t, 2)
		}
	}
	s.topUpSpots()
}

// eyeAt sets the eye level camera in front of the cafe, looking at the middle
// of the desk pavilions.
func (s *scene) eyeAt() {
	cafe := s.ofType("cafe")
	if len(cafe) == 0 {
		return
	}
	_, d := size(cafe[0])
	from := rotPt(cafe[0].X, cafe[0].Z, cafe[0].Rot, 0, d/2+2.2)
	var sx, sz float64
	n := 0
	for _, p := range s.pavs {
		if p.Type == "workspace" || p.Type == "research" || p.Type == "cabin" {
			sx, sz, n = sx+p.X, sz+p.Z, n+1
		}
	}
	look := pt{cafe[0].X, cafe[0].Z}
	if n > 0 {
		look = pt{sx / float64(n), sz / float64(n)}
	}
	s.eye = &layout.Eye{From: from, Look: look}
}

// shift moves everything by (dx, dz).
func (s *scene) shift(dx, dz float64) {
	mv := func(p pt) pt { return pt{p[0] + dx, p[1] + dz} }
	mvs := func(ps []pt) []pt {
		out := make([]pt, len(ps))
		for i, p := range ps {
			out[i] = mv(p)
		}
		return out
	}
	s.base = mvs(s.base)
	for i := range s.land {
		s.land[i] = mvs(s.land[i])
	}
	for i := range s.pavs {
		s.pavs[i].X, s.pavs[i].Z = s.pavs[i].X+dx, s.pavs[i].Z+dz
	}
	if s.river != nil {
		s.river.Pts = mvs(s.river.Pts)
	}
	for i := range s.ponds {
		s.ponds[i].X, s.ponds[i].Z = s.ponds[i].X+dx, s.ponds[i].Z+dz
	}
	for i := range s.paths {
		s.paths[i].Pts = mvs(s.paths[i].Pts)
	}
	for i := range s.bridges {
		s.bridges[i].A, s.bridges[i].B = mv(s.bridges[i].A), mv(s.bridges[i].B)
	}
	for i := range s.props {
		s.props[i].X, s.props[i].Z = s.props[i].X+dx, s.props[i].Z+dz
	}
	for i := range s.spots {
		s.spots[i].X, s.spots[i].Z = s.spots[i].X+dx, s.spots[i].Z+dz
	}
	if s.eye != nil {
		s.eye.From, s.eye.Look = mv(s.eye.From), mv(s.eye.Look)
	}
}

// centre moves the base outline's bounding box onto the origin.
func (s *scene) centre() {
	lox, loz, hix, hiz := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range s.base {
		lox, hix = math.Min(lox, p[0]), math.Max(hix, p[0])
		loz, hiz = math.Min(loz, p[1]), math.Max(hiz, p[1])
	}
	s.shift(-(lox+hix)/2, -(loz+hiz)/2)
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

func rpt(p pt) pt { return pt{r2(p[0]), r2(p[1])} }

func rpts(ps []pt) []pt {
	out := make([]pt, len(ps))
	for i, p := range ps {
		out[i] = rpt(p)
	}
	return out
}

// finish recentres, rounds every number to two decimals and builds the layout.
func (s *scene) finish(template string) layout.Layout {
	s.centre()
	site := &layout.Site{
		Base:    layout.Poly{Poly: rpts(s.base)},
		Land:    []layout.Poly{},
		Ponds:   []layout.Pond{},
		Paths:   []layout.Path{},
		Bridges: []layout.Bridge{},
		Props:   []layout.Prop{},
		Trees:   layout.Trees{Mode: "edge", Band: s.treeBand, Seed: s.treeSeed},
		Fence:   true,
	}
	if len(s.land) == 0 {
		s.land = [][]pt{s.base}
	}
	for _, l := range s.land {
		site.Land = append(site.Land, layout.Poly{Poly: rpts(l)})
	}
	if s.river != nil {
		site.River = &layout.River{Pts: rpts(s.river.Pts), W: s.river.W}
	}
	for _, p := range s.ponds {
		site.Ponds = append(site.Ponds, layout.Pond{X: r2(p.X), Z: r2(p.Z), RX: p.RX, RZ: p.RZ})
	}
	for _, p := range s.paths {
		site.Paths = append(site.Paths, layout.Path{Pts: rpts(p.Pts), W: p.W, Smooth: p.Smooth})
	}
	for _, b := range s.bridges {
		site.Bridges = append(site.Bridges, layout.Bridge{A: rpt(b.A), B: rpt(b.B), W: b.W})
	}
	for _, p := range s.props {
		p.X, p.Z, p.Rot = r2(p.X), r2(p.Z), r2(p.Rot)
		site.Props = append(site.Props, p)
	}
	l := layout.Layout{V: v2, Name: s.name, Title: s.title, Template: template, Site: site}
	for _, p := range s.pavs {
		p.X, p.Z, p.Rot = r2(p.X), r2(p.Z), r2(normDeg(p.Rot))
		l.Pavilions = append(l.Pavilions, p)
	}
	for _, p := range s.spots {
		l.Overflow = append(l.Overflow, layout.Spot{X: r2(p.X), Z: r2(p.Z), Rot: r2(p.Rot), Near: p.Near})
	}
	if s.eye != nil {
		l.Eye = &layout.Eye{From: rpt(s.eye.From), Look: rpt(s.eye.Look)}
	}
	return l
}

func inEllipse(x, z float64, p layout.Pond, pad float64) bool {
	dx, dz := (x-p.X)/(p.RX+pad), (z-p.Z)/(p.RZ+pad)
	return dx*dx+dz*dz < 1
}
