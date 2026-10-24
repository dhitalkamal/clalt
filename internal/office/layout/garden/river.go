package garden

import (
	"errors"
	"math"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// river: a stream crosses the island diagonally with wooden bridges. cabins,
// treehouse and lobby stand on the west bank, workspace, research, cafe and
// play on the east bank. the island is a rectangle that grows until
// everything fits, the walkways come from the grid router.
const (
	riverW     = 3.0
	riverHW    = riverW / 2
	islandR    = 2.4
	cabinLeft  = 2.4
	cabinTop   = 3.6
	rowLane    = 2.9 // cabin row bottom to the next row top
	packGap    = 1.8 // clear space kept around a placed pavilion
	edgeMargin = 1.4
	westClear  = riverHW + 2.2
	bridgeOver = 1.0
	bridgeZone = 2.4
)

// riverShape is the viewer's river as offsets from its north end, by fraction
// of the length.
var riverShape = [7][2]float64{{0, 0}, {0.19, -1.8}, {0.35, -5.2}, {0.51, -9.2}, {0.66, -12.6}, {0.82, -16}, {1, -17.8}}

func riverLine(x0, d float64) []pt {
	k := (d + 5) / 29
	ctl := make([]pt, len(riverShape))
	for i, c := range riverShape {
		ctl[i] = pt{x0 + c[1]*k, -2.5 + c[0]*(d+5)}
	}
	return curve(ctl, 8)
}

func genRiver(p plan) (layout.Layout, error) {
	// the island grows in both directions together so it keeps the viewer's
	// shape. search in coarse steps first, then step back to the smallest fit.
	dims := func(k int) (float64, float64) { return 38.0 + 2*float64(k), 24.0 + 2*math.Floor(0.75*float64(k)) }
	const step = 6
	for hi := 0; hi < 90; hi += step {
		w, d := dims(hi)
		s, ok := buildRiver(p, w, d)
		if !ok {
			continue
		}
		for k := max(0, hi-step+1); k < hi; k++ {
			w, d := dims(k)
			if s2, ok := buildRiver(p, w, d); ok {
				s = s2
				break
			}
		}
		return s.finish("river"), nil
	}
	return layout.Layout{}, errors.New("garden: river template does not fit")
}

func buildRiver(p plan, w, d float64) (*scene, bool) {
	s := newScene("river", "River campus", 3.2, 23)
	// west bank: cabin rows from the top left, treehouse under them
	z, right := cabinTop, cabinLeft
	for i := 0; i < len(p.cabins); i += rowsPerLine {
		row := p.cabins[i:min(i+rowsPerLine, len(p.cabins))]
		x, rowD := cabinLeft, 0.0
		for _, c := range row {
			cw, cd := size(c)
			c.X, c.Z = x+cw/2, z+cd/2
			s.pav(c)
			rowD = max(rowD, cd)
			x += cw + cabinGap
		}
		right = max(right, x-cabinGap)
		z += rowD + rowLane
	}
	th := layout.Pavilion{ID: "treehouse", Type: "treehouse"}
	tw, td := size(th)
	thTop := z + 1.1
	if len(p.cabins) == 0 {
		thTop = cabinTop
	}
	th.X, th.Z = 1.7+tw/2, thTop+td/2
	s.pav(th)
	lobby := layout.Pavilion{ID: "lobby", Type: "lobby", Rot: 90, X: 0.9 + 2.0, Z: d - 4.2}
	if thTop+td+1.8 > lobby.Z-1.9 {
		return s, false
	}
	s.pav(lobby)
	westN := len(s.pavs)

	// river: pushed east until the west bank clears it
	var line []pt
	for x0 := 8.0; x0 < w-8; x0 += 0.5 {
		line = riverLine(x0, d)
		clear := true
		for _, pv := range s.pavs {
			if nearBoundary(corners(pv), func(x, z float64) bool { return distLine(x, z, line) < westClear }) {
				clear = false
				break
			}
		}
		if clear {
			break
		}
		line = nil
	}
	if line == nil {
		return s, false
	}
	s.river = &layout.River{Pts: line, W: riverW}
	s.base = rectPoly(w/2, d/2, w, d, islandR, 4)
	s.land = bankPolys(line, w, d)
	if !s.addBridges() {
		return s, false
	}

	// east bank: first fit packing on a grid mask
	pk := newPacker(s, w, d, westN)
	place := func(pv layout.Pavilion, mode string, zFrom float64) (layout.Pavilion, bool) {
		pw, pd := size(pv)
		x, zc, ok := pk.fit(pw, pd, mode, zFrom)
		pv.X, pv.Z = x, zc
		return pv, ok
	}
	for _, pv := range append(append([]layout.Pavilion{}, p.workspace...), p.research...) {
		pv, ok := place(pv, "tr", 0)
		if !ok {
			return s, false
		}
		s.pav(pv)
	}
	pond := layout.Pond{RX: 2.3, RZ: 1.0}
	if x, zc, ok := pk.fit(2*pond.RX, 2*pond.RZ, "tr", d*0.4); ok {
		pond.X, pond.Z = x, zc
		s.ponds = append(s.ponds, pond)
	} else {
		return s, false
	}
	for _, pv := range []layout.Pavilion{{ID: "play", Type: "play"}, {ID: "cafe", Type: "cafe"}} {
		mode := "br"
		if pv.Type == "cafe" {
			mode = "bl"
		}
		pv, ok := place(pv, mode, 0)
		if !ok {
			return s, false
		}
		s.pav(pv)
	}

	r := newRouter(s, w, d)
	if !r.routeAll(s, lobby) {
		return s, false
	}
	br := s.bridges[0]
	s.prop("bench", br.A[0]-2.2, br.A[1]+1.0, 180, 0.5)
	s.prop("bench", br.B[0]+2.2, br.B[1]+1.0, 180, 0.5)
	s.bed(6, thTop-1.4, 1.4, 0.5)
	s.bed(w-5, d*0.55, 0.6, 3.4)
	s.bed(w*0.5, d-3, 1.6, 0.5)
	for _, b := range s.bridges {
		s.prop("lamp", b.A[0]-1.0, b.A[1]-0.9, 0, 0.3)
		s.prop("lamp", b.B[0]+1.0, b.B[1]+0.9, 0, 0.3)
	}
	s.addSpots()
	s.eyeAt()
	return s, true
}

// addBridges lays evenly spread bridges, nudging each along the river until
// both ends stand clear of buildings. one is enough, none is a failure.
func (s *scene) addBridges() bool {
	line := s.river.Pts
	n := max(3, int(math.Round(polyLen(line)/11)))
	n = min(n, 16)
	for i := 0; i < n; i++ {
		t0 := 0.2 + 0.6*float64(i)/float64(n-1)
		for _, dt := range []float64{0, 0.015, -0.015, 0.03, -0.03, 0.05, -0.05, 0.08, -0.08} {
			c, tx, tz := arcAt(line, t0+dt)
			nx, nz := -tz, tx
			a := snap(pt{c[0] + nx*(riverHW+bridgeOver), c[1] + nz*(riverHW+bridgeOver)})
			b := snap(pt{c[0] - nx*(riverHW+bridgeOver), c[1] - nz*(riverHW+bridgeOver)})
			if s.bridgeEndOK(a) && s.bridgeEndOK(b) && s.bridgeApart(a, b) {
				s.bridges = append(s.bridges, layout.Bridge{A: a, B: b, W: 1.3})
				break
			}
		}
	}
	return len(s.bridges) > 0
}

func snap(p pt) pt {
	return pt{math.Round(p[0]/cellSize) * cellSize, math.Round(p[1]/cellSize) * cellSize}
}

func (s *scene) bridgeEndOK(p pt) bool {
	if !inPoly(p[0], p[1], s.base) || distPoly(p[0], p[1], s.base) < 2.0 {
		return false
	}
	for _, pv := range s.pavs {
		if distToPav(pv, p[0], p[1]) < 2.2 {
			return false
		}
	}
	return true
}

// bridgeApart keeps bridges at least 3 units from each other.
func (s *scene) bridgeApart(a, b pt) bool {
	for _, o := range s.bridges {
		if dist(a, o.A) < 3 || dist(b, o.B) < 3 {
			return false
		}
	}
	return true
}
