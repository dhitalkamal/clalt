package garden

import (
	"errors"
	"math"
	"slices"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// village: pavilions on an ellipse ring facing a green with a pond and the
// cafe, a loop path between the green and the doors, a short spoke from every
// door. past ten pavilions a second ring stands outside the first, doors on a
// second loop that joins the inner one through the gaps between pavilions.
const (
	ringMax   = 10
	ringGap   = 1.8 // arc allowance between neighbours, on top of width
	ringStart = -150.0
	ringEnd   = 150.0
)

type ring struct {
	items  []layout.Pavilion
	rx, rz float64
	bounds []float64 // param angle (deg) between neighbours
}

// place puts the items on the ellipse by arc length, facing the middle.
func (r *ring) place() {
	const n = 1800
	arc := make([]float64, n+1)
	for i := 1; i <= n; i++ {
		a0 := rad(ringStart + (ringEnd-ringStart)*float64(i-1)/n)
		a1 := rad(ringStart + (ringEnd-ringStart)*float64(i)/n)
		arc[i] = arc[i-1] + math.Hypot(r.rx*(math.Cos(a1)-math.Cos(a0)), r.rz*(math.Sin(a1)-math.Sin(a0)))
	}
	at := func(f float64) float64 {
		want := f * arc[n]
		i := 0
		for i < n && arc[i+1] < want {
			i++
		}
		return ringStart + (ringEnd-ringStart)*float64(i)/n
	}
	total := 0.0
	for _, it := range r.items {
		w, _ := size(it)
		total += w + ringGap
	}
	acc := 0.0
	r.bounds = nil
	for i := range r.items {
		w, _ := size(r.items[i])
		a := rad(at((acc + (w+ringGap)/2) / total))
		r.items[i].X, r.items[i].Z = r.rx*math.Cos(a), r.rz*math.Sin(a)
		r.items[i].Rot = faceDeg(r.items[i].X, r.items[i].Z, 0, 0)
		acc += w + ringGap
		if i+1 < len(r.items) {
			r.bounds = append(r.bounds, at(acc/total))
		}
	}
}

func maxDepth(ps []layout.Pavilion) float64 {
	m := 0.0
	for _, p := range ps {
		_, d := size(p)
		m = max(m, d)
	}
	return m
}

func ellipsePt(rx, rz, deg float64) pt { return pt{rx * math.Cos(rad(deg)), rz * math.Sin(rad(deg))} }

// toEllipse pulls a point along the line to the middle onto an ellipse.
func toEllipse(p pt, rx, rz float64) pt {
	k := math.Hypot(p[0]/rx, p[1]/rz)
	return pt{p[0] / k, p[1] / k}
}

func genVillage(p plan) (layout.Layout, error) {
	items := slices.Concat(p.cabins,
		[]layout.Pavilion{{ID: "treehouse", Type: "treehouse"}, {ID: "play", Type: "play"}},
		p.workspace, p.team, p.research)
	for e := 0.0; e <= 160; e += 0.5 {
		if s, ok := buildVillage(items, e); ok {
			return s.finish("village"), nil
		}
	}
	return layout.Layout{}, errors.New("garden: village template does not fit")
}

// buildVillage lays the village out with the ring grown by e and reports
// whether nothing overlaps.
func buildVillage(items []layout.Pavilion, e float64) (*scene, bool) {
	s := newScene("village", "Village green", 3.4, 17)
	inner := items[:min(ringMax, len(items))]
	outerItems := items[len(inner):]
	in := &ring{items: append([]layout.Pavilion{}, inner...), rx: 13.4 + e, rz: 8.9 + 0.7*e}
	in.place()
	pad := max(0, maxDepth(inner)-4.2) / 2
	pathRX, pathRZ := in.rx-5.0-pad, in.rz-3.7-pad
	// wide pavilions poke past the ring where it bends: pull the loop in
	// until it clears every one of them
	for pathRX >= 8.4 && pathRZ >= 5.2 && !loopClear(in.items, pathRX, pathRZ) {
		pathRX, pathRZ = pathRX-0.5, pathRZ-0.5
	}
	ok := pathRX >= 8.4 && pathRZ >= 5.2

	for _, it := range in.items {
		s.pav(it)
	}
	// the inner loop, then a spoke from every inner door
	loop := loopPts
	s.path(1.0, false, loop(pathRX, pathRZ)...)
	for _, it := range in.items {
		s.path(0.8, false, door(it), toEllipse(door(it), pathRX, pathRZ))
	}
	outRX, outRZ := in.rx, in.rz
	if len(outerItems) > 0 {
		out, loopRX, loopRZ, found := outerRing(in, in.items, outerItems)
		ok = ok && found
		s.path(1.0, false, loop(loopRX, loopRZ)...)
		for _, it := range out.items {
			s.pav(it)
			s.path(0.8, false, door(it), toEllipse(door(it), loopRX, loopRZ))
		}
		// connectors through the gaps between inner pavilions
		for _, k := range connectorIdx(len(in.bounds)) {
			a := in.bounds[k]
			s.path(0.9, false, ellipsePt(pathRX, pathRZ, a), ellipsePt(loopRX, loopRZ, a))
		}
		outRX, outRZ = out.rx, out.rz
	}

	// cafe and pond on the green
	cafe := layout.Pavilion{ID: "cafe", Type: "cafe", X: 2.4, Z: -0.6}
	s.pav(cafe)
	cd := door(cafe)
	s.path(0.8, false, cd, toEllipse(pt{cd[0], pathRZ}, pathRX, pathRZ))
	s.ponds = append(s.ponds, layout.Pond{X: -3.6, Z: 0.9, RX: 2.5, RZ: 1.5})

	// lobby at the west edge, door onto the loops
	lobby := layout.Pavilion{ID: "lobby", Type: "lobby", Rot: 90, X: -(outRX + 3.7)}
	s.pav(lobby)
	s.path(1.0, false, door(lobby), pt{-pathRX, 0})

	// island ellipse grows until every corner sits well inside it
	ax, az := outRX+7.1, outRZ+5.3
	for i := 0; i < 200; i++ {
		fits := true
		s.base = ellipsePoly(0, 0, ax, az, 0, 1, 72)
		for _, p := range s.pavs {
			for _, c := range corners(p) {
				if !inPoly(c[0], c[1], s.base) || distPoly(c[0], c[1], s.base) < 1.2 {
					fits = false
				}
			}
		}
		if fits {
			break
		}
		ax, az = ax*1.02, az*1.02
	}

	s.prop("bench", -3.6, 3.0, 180, 0.5)
	s.prop("bench", 3.4, 3.4, 180, 0.5)
	s.bed(6.2, -3.0, 1.4, 0.5)
	for _, a := range []float64{214, 326, 146, 34} {
		q := ellipsePt(pathRX+0.9, pathRZ+0.9, a)
		s.prop("lamp", q[0], q[1], 0, 0.3)
	}
	s.addSpots()
	s.eyeAt()
	return s, ok && s.apart(1.0) && s.pathsClear()
}

// connectorIdx picks up to three gap indexes spread over the ring.
func connectorIdx(n int) []int {
	switch {
	case n <= 0:
		return nil
	case n < 4:
		return []int{n / 2}
	}
	return []int{n / 4, n / 2, 3 * n / 4}
}

// apart reports whether all pavilions stay at least gap from each other.
func (s *scene) apart(gap float64) bool {
	for i, p := range s.pavs {
		for _, q := range s.pavs[i+1:] {
			if !polysApart(corners(p), corners(q), gap) {
				return false
			}
		}
	}
	return true
}

// pathsClear reports whether no walkway crosses a pavilion. the first and
// last half unit of a path are skipped: that is where it meets a door.
func (s *scene) pathsClear() bool {
	for _, pa := range s.paths {
		total := polyLen(pa.Pts)
		for t := 0.0; t <= total; t += 0.25 {
			if t < 0.5 || total-t < 0.5 {
				continue
			}
			q, _, _ := arcAt(pa.Pts, t/total)
			for _, p := range s.pavs {
				if distToPav(p, q[0], q[1]) < 0.3 {
					return false
				}
			}
		}
	}
	return true
}

// loopClear reports whether the loop path of an ellipse keeps 0.9 from every
// pavilion, measured along the actual polyline.
func loopClear(ps []layout.Pavilion, rx, rz float64) bool {
	l := loopPts(rx, rz)
	for _, p := range ps {
		for i := 0; i+1 < len(l); i++ {
			n := int(dist(l[i], l[i+1])/0.3) + 1
			for k := 0; k <= n; k++ {
				u := float64(k) / float64(n)
				if distToPav(p, l[i][0]+(l[i+1][0]-l[i][0])*u, l[i][1]+(l[i+1][1]-l[i][1])*u) < 0.9 {
					return false
				}
			}
		}
	}
	return true
}

// loopPts is the closed loop polyline of an ellipse path, 44 points.
func loopPts(rx, rz float64) []pt {
	l := make([]pt, 0, 45)
	for i := 0; i < 44; i++ {
		l = append(l, ellipsePt(rx, rz, float64(i)*360/44))
	}
	return append(l, l[0])
}

// outerRing places the second ring far enough out that a loop path fits
// between the rings clear of both, and returns the ring and that loop.
func outerRing(in *ring, inner, outer []layout.Pavilion) (out *ring, lrx, lrz float64, found bool) {
	both := append(append([]layout.Pavilion{}, inner...), outer...)
	base := maxDepth(inner)/2 + maxDepth(outer)/2 + 3.4
	for h := base; h < base+40; h += 0.5 {
		out = &ring{items: append([]layout.Pavilion{}, outer...), rx: in.rx + h, rz: in.rz + h}
		out.place()
		both = append(both[:len(inner)], out.items...)
		for o := 1.0; o < h; o += 0.5 {
			if loopClear(both, in.rx+o, in.rz+o) {
				return out, in.rx + o, in.rz + o, true
			}
		}
	}
	return out, in.rx, in.rz, false
}
