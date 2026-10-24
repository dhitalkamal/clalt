package garden

import (
	"errors"
	"math"
	"sort"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// street: one winding street from the lobby gate to a fountain plaza, houses
// on both sides. the north side holds the cabins (a side street takes those
// past eight), the south side research, workspace and the cafe. treehouse and
// play stand at the plaza.
const (
	streetCabinsMain = 8
	streetStart      = 2.5 // gate to the first house
	streetFront      = 2.3 // street centre line to a house front edge, less half depth
	tiltCap          = 0.3 * 180 / math.Pi
)

// streetZ is the street's centre line z at x, a gentle S.
func streetZ(x float64) float64 { return 1.3 * math.Sin(2*math.Pi*x/26+0.6) }

// streetSlope is dz/dx of streetZ.
func streetSlope(x float64) float64 {
	return 1.3 * 2 * math.Pi / 26 * math.Cos(2*math.Pi*x/26+0.6)
}

func genStreet(p plan) (layout.Layout, error) {
	for gap := 1.0; gap <= 8; gap += 0.3 {
		if s, ok := buildStreet(p, gap); ok {
			return s.finish("street"), nil
		}
	}
	return layout.Layout{}, errors.New("garden: street template does not fit")
}

// beside puts a house next to the street at x. side +1 is north, -1 south.
// it leans a little with the slope, never more than tiltCap.
func beside(pv layout.Pavilion, x, side float64) (layout.Pavilion, pt) {
	_, d := size(pv)
	off := d/2 + streetFront
	pz := streetZ(x)
	ideal := 0.0
	if side < 0 {
		ideal = 180
	}
	tx, tz := 1.0, streetSlope(x)
	toward := math.Atan2(-tz, tx) * 180 / math.Pi
	if side < 0 {
		toward = math.Atan2(tz, -tx) * 180 / math.Pi
	}
	dlt := normDeg(toward-ideal) * 0.6
	pv.X, pv.Z = x, pz-side*off
	pv.Rot = ideal + math.Max(-tiltCap, math.Min(tiltCap, dlt))
	return pv, pt{x, pz}
}

func buildStreet(p plan, gap float64) (*scene, bool) {
	s := newScene("street", "Main street", 3.0, 5)
	anchors := []float64{1.0}
	endN, endS := streetStart, streetStart

	// north side: cabins; the rest go on the side street
	main := p.cabins[:min(streetCabinsMain, len(p.cabins))]
	side := p.cabins[len(main):]
	for _, c := range main {
		w, _ := size(c)
		c, a := beside(c, endN+w/2, 1)
		s.pav(c)
		s.spur(c, a)
		anchors = append(anchors, a[0])
		endN += w + gap
	}
	// south side: research, workspace, cafe
	south := append(append(append([]layout.Pavilion{}, p.research...), p.workspace...), layout.Pavilion{ID: "cafe", Type: "cafe"})
	for _, c := range south {
		w, _ := size(c)
		c, a := beside(c, endS+w/2, -1)
		s.pav(c)
		s.spur(c, a)
		anchors = append(anchors, a[0])
		endS += w + gap
	}
	if len(side) > 0 {
		top := 0.0
		for _, pv := range s.pavs {
			if pv.Type == "cabin" {
				for _, c := range corners(pv) {
					top = math.Min(top, c[1])
				}
			}
		}
		lane := top - 1.5
		x := streetStart
		for _, c := range side {
			w, d := size(c)
			c.X, c.Z, c.Rot = x+w/2, lane-1.5-d/2, 0
			s.pav(c)
			s.spur(c, pt{c.X, lane})
			x += w + gap
		}
		s.path(1.0, false, pt{1.0, streetZ(1.0)}, pt{1.0, lane}, pt{x - gap + 0.5, lane})
	}

	// plaza past the last house, treehouse north of it and play south
	xp := math.Max(endN, endS) + 6.0
	zp := streetZ(xp)
	th := layout.Pavilion{ID: "treehouse", Type: "treehouse", X: xp + 1.0, Z: zp - 6.6}
	pl := layout.Pavilion{ID: "play", Type: "play", X: xp + 0.5, Z: zp + 6.4, Rot: 180}
	s.pav(th)
	s.pav(pl)
	lobby := layout.Pavilion{ID: "lobby", Type: "lobby", Rot: 90, X: -2.02, Z: streetZ(0)}
	s.pav(lobby)

	// the street itself: every anchor is a vertex, so spurs meet it exactly
	xs := []float64{0}
	for x := 3.0; x < xp-1.0; x += 3 {
		xs = append(xs, x)
	}
	xs = append(xs, anchors...)
	sort.Float64s(xs)
	var line []pt
	for _, x := range xs {
		if len(line) > 0 && x-line[len(line)-1][0] < 0.3 && !contains(anchors, x) {
			continue
		}
		line = append(line, pt{x, streetZ(x)})
	}
	line = append(line, pt{xp - 0.8, zp})
	// drop sampled points that crowd an anchor
	s.path(1.3, true, dedupe(line, anchors)...)
	line0 := door(lobby)
	s.path(1.0, false, line0, pt{0, streetZ(0)})
	plaza := make([]pt, 0, 13)
	for k := 0; k < 12; k++ {
		plaza = append(plaza, pt{xp + 0.8*math.Cos(rad(float64(k)*30+180)), zp + 0.8*math.Sin(rad(float64(k)*30+180))})
	}
	s.path(3.6, false, append(plaza, plaza[0])...)
	td := door(th)
	s.path(0.9, false, td, pt{td[0], zp}, pt{xp + 0.8, zp})
	s.path(0.8, false, door(pl), pt{xp, zp + 0.8})
	s.props = append(s.props, layout.Prop{T: "fountain", X: xp, Z: zp})

	// pond north of everything, then the island around the lot
	north := 0.0
	var lox, hix, hiz = math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, pv := range s.pavs {
		for _, c := range corners(pv) {
			north = math.Min(north, c[1])
			lox, hix, hiz = math.Min(lox, c[0]), math.Max(hix, c[0]), math.Max(hiz, c[1])
		}
	}
	pond := layout.Pond{X: 0.45 * xp, Z: north - 3.4, RX: 2.6, RZ: 1.2}
	s.ponds = append(s.ponds, pond)
	cx, cz := (lox+hix)/2, (north-4.6+hiz)/2
	ax, az := (hix-lox)/2+4, (hiz-north+4.6)/2+3
	for i := 0; i < 300; i++ {
		s.base = ellipsePoly(cx, cz, ax, az, 0.04, 2.1, 90)
		if s.allInside(1.2, pt{pond.X - pond.RX, pond.Z}, pt{pond.X + pond.RX, pond.Z}, pt{pond.X, pond.Z - pond.RZ}) {
			break
		}
		ax, az = ax*1.02, az*1.02
	}

	s.prop("bench", pond.X, pond.Z+1.9, 180, 0.5)
	s.prop("bench", xp-3.5, zp+2.4, 180, 0.5)
	s.bed(xp-5, zp-2.8, 3.0, 0.5)
	for x := 4.0; x < xp-3; x += 10 {
		s.prop("lamp", x, streetZ(x)+1.3, 0, 0.3)
		s.prop("lamp", x+5, streetZ(x+5)-1.3, 0, 0.3)
	}
	s.addSpots()
	s.eyeAt()
	return s, s.apart(0.6) && s.pathsClear()
}

func contains(xs []float64, v float64) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// dedupe drops a sampled point that sits within 0.3 in x of a neighbour that
// is an anchor, so the line has no tiny wiggles at the junctions.
func dedupe(line []pt, anchors []float64) []pt {
	out := line[:0:0]
	for i, p := range line {
		keep := true
		if !contains(anchors, p[0]) && i+1 < len(line) && i > 0 {
			for _, a := range anchors {
				if abs(a-p[0]) < 0.9 {
					keep = false
				}
			}
		}
		if keep {
			out = append(out, p)
		}
	}
	return out
}

// allInside reports whether every pavilion corner, the plaza reach and the
// extra points sit inside the base with room to spare.
func (s *scene) allInside(margin float64, extra ...pt) bool {
	check := func(p pt) bool { return inPoly(p[0], p[1], s.base) && distPoly(p[0], p[1], s.base) >= margin }
	for _, pv := range s.pavs {
		for _, c := range corners(pv) {
			if !check(c) {
				return false
			}
		}
	}
	for _, e := range extra {
		if !check(e) {
			return false
		}
	}
	return true
}
