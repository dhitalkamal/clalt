package layout

import "math"

// eps is the slack of the geometry checks: shapes that only touch do not
// overlap. every input was range checked before it gets here.
const eps = 1e-9

// box is a rotated rectangle: centre, half sizes, rot in degrees counter
// clockwise seen from above (x east, z south).
type box struct {
	cx, cz, hw, hd float64
	sin, cos       float64
}

func newBox(x, z, w, d, rotDeg float64) box {
	r := rotDeg * math.Pi / 180
	return box{cx: x, cz: z, hw: w / 2, hd: d / 2, sin: math.Sin(r), cos: math.Cos(r)}
}

// toWorld maps a point in the box frame (x along its width) to the world.
func (b box) toWorld(lx, lz float64) Pt {
	return Pt{b.cx + lx*b.cos + lz*b.sin, b.cz - lx*b.sin + lz*b.cos}
}

// toLocal is the inverse of toWorld.
func (b box) toLocal(p Pt) Pt {
	dx, dz := p[0]-b.cx, p[1]-b.cz
	return Pt{dx*b.cos - dz*b.sin, dx*b.sin + dz*b.cos}
}

func (b box) corners() [4]Pt {
	return [4]Pt{b.toWorld(-b.hw, -b.hd), b.toWorld(b.hw, -b.hd), b.toWorld(b.hw, b.hd), b.toWorld(-b.hw, b.hd)}
}

// axes are the unit directions of the box sides in the world.
func (b box) axes() [2]Pt { return [2]Pt{{b.cos, -b.sin}, {b.sin, b.cos}} }

// overlaps is a separating axis test of two rotated rectangles.
func (b box) overlaps(o box) bool {
	for _, ax := range [][2]Pt{{b.axes()[0], b.axes()[1]}, {o.axes()[0], o.axes()[1]}} {
		for _, a := range ax {
			lo1, hi1 := project(b.corners(), a)
			lo2, hi2 := project(o.corners(), a)
			if hi1 <= lo2+eps || hi2 <= lo1+eps {
				return false
			}
		}
	}
	return true
}

func project(c [4]Pt, axis Pt) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, p := range c {
		v := p[0]*axis[0] + p[1]*axis[1]
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return lo, hi
}

// inPoly says whether p is inside or on the outline of poly.
func inPoly(p Pt, poly []Pt) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if distToSegment(p, a, b) <= eps {
			return true
		}
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

func distToSegment(p, a, b Pt) float64 {
	dx, dz := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dz*dz
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dz)/l2))
	}
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dz))
}

// distToBox is the distance of a point to an axis aligned box centred at 0.
func distToBox(p Pt, hw, hd float64) float64 {
	return math.Hypot(math.Max(math.Abs(p[0])-hw, 0), math.Max(math.Abs(p[1])-hd, 0))
}

// segmentHitsBox says whether the segment ab crosses the axis aligned box
// centred at 0 (Liang-Barsky clipping).
func segmentHitsBox(a, b Pt, hw, hd float64) bool {
	t0, t1 := 0.0, 1.0
	d := Pt{b[0] - a[0], b[1] - a[1]}
	lim := Pt{hw, hd}
	for i := 0; i < 2; i++ {
		if d[i] == 0 {
			if math.Abs(a[i]) > lim[i] {
				return false
			}
			continue
		}
		ta, tb := (-lim[i]-a[i])/d[i], (lim[i]-a[i])/d[i]
		if ta > tb {
			ta, tb = tb, ta
		}
		t0, t1 = math.Max(t0, ta), math.Min(t1, tb)
		if t0 > t1 {
			return false
		}
	}
	return true
}

// hitsBand says whether the box comes within half of a stream's width of the
// centre line, joints included (the band has round joints).
func (b box) hitsBand(pts []Pt, w float64) bool {
	r := w / 2
	for i := 1; i < len(pts); i++ {
		a, c := b.toLocal(pts[i-1]), b.toLocal(pts[i])
		if segmentHitsBox(a, c, b.hw, b.hd) {
			return true
		}
		d := math.Min(distToBox(a, b.hw, b.hd), distToBox(c, b.hw, b.hd))
		for _, k := range [][2]float64{{-b.hw, -b.hd}, {b.hw, -b.hd}, {b.hw, b.hd}, {-b.hw, b.hd}} {
			d = math.Min(d, distToSegment(Pt{k[0], k[1]}, a, c))
		}
		if d < r-eps {
			return true
		}
	}
	return false
}

// hitsPond says whether the box overlaps an oval pond. the plane is scaled so
// the pond is a unit circle, which turns the box into a parallelogram.
func (b box) hitsPond(p Pond) bool {
	var q [4]Pt
	for i, c := range b.corners() {
		q[i] = Pt{(c[0] - p.X) / p.RX, (c[1] - p.Z) / p.RZ}
	}
	if inPoly(Pt{}, q[:]) {
		return true
	}
	for i := range q {
		if distToSegment(Pt{}, q[i], q[(i+1)%4]) < 1-eps {
			return true
		}
	}
	return false
}
