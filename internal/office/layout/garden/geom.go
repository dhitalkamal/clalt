package garden

import (
	"math"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// pt is an [x, z] point, same as the format.
type pt = layout.Pt

func rad(deg float64) float64 { return deg * math.Pi / 180 }

func dist(a, b pt) float64 { return math.Hypot(a[0]-b[0], a[1]-b[1]) }

// rotPt turns a local offset (lx, lz) of something at (x, z) turned by rot
// degrees into a world point. 0 faces +z, same as the viewer.
func rotPt(x, z, rot, lx, lz float64) pt {
	c, s := math.Cos(rad(rot)), math.Sin(rad(rot))
	return pt{x + lx*c + lz*s, z - lx*s + lz*c}
}

// faceDeg is the rot that makes a local +z front look at (tx, tz).
func faceDeg(x, z, tx, tz float64) float64 {
	return math.Atan2(tx-x, tz-z) * 180 / math.Pi
}

// normDeg folds an angle into (-180, 180].
func normDeg(a float64) float64 {
	a = math.Mod(a, 360)
	if a > 180 {
		a -= 360
	}
	if a <= -180 {
		a += 360
	}
	return a
}

// rectPoly is a rounded rectangle outline, corners cut with radius r.
func rectPoly(cx, cz, w, d, r float64, seg int) []pt {
	hx, hz := w/2, d/2
	if r <= 0 {
		return []pt{{cx - hx, cz - hz}, {cx + hx, cz - hz}, {cx + hx, cz + hz}, {cx - hx, cz + hz}}
	}
	var out []pt
	corner := func(ox, oz, a0 float64) {
		for i := 0; i <= seg; i++ {
			a := a0 + float64(i)/float64(seg)*math.Pi/2
			out = append(out, pt{ox + math.Cos(a)*r, oz + math.Sin(a)*r})
		}
	}
	corner(cx+hx-r, cz+hz-r, 0)
	corner(cx-hx+r, cz+hz-r, math.Pi/2)
	corner(cx-hx+r, cz-hz+r, math.Pi)
	corner(cx+hx-r, cz-hz+r, math.Pi*1.5)
	return out
}

// ellipsePoly is an ellipse outline with a small seeded wobble so an island
// does not look machined.
func ellipsePoly(cx, cz, rx, rz, wob, seed float64, n int) []pt {
	out := make([]pt, n)
	for i := range out {
		a := float64(i) / float64(n) * 2 * math.Pi
		k := 1 + wob*(math.Sin(a*3+seed)*0.5+math.Sin(a*5+seed*2.3)*0.35+math.Sin(a*2+seed*1.7)*0.4)
		out[i] = pt{cx + math.Cos(a)*rx*k, cz + math.Sin(a)*rz*k}
	}
	return out
}

// chaikin rounds the corners of a closed polygon.
func chaikin(p []pt, iter int) []pt {
	for k := 0; k < iter; k++ {
		q := make([]pt, 0, len(p)*2)
		for i := range p {
			a, b := p[i], p[(i+1)%len(p)]
			q = append(q,
				pt{a[0]*0.75 + b[0]*0.25, a[1]*0.75 + b[1]*0.25},
				pt{a[0]*0.25 + b[0]*0.75, a[1]*0.25 + b[1]*0.75})
		}
		p = q
	}
	return p
}

// curve is a catmull-rom spline through pts, n samples per segment.
func curve(pts []pt, n int) []pt {
	var out []pt
	last := len(pts) - 1
	for i := 0; i < last; i++ {
		p0 := pts[max(0, i-1)]
		p1, p2 := pts[i], pts[i+1]
		p3 := pts[min(last, i+2)]
		for s := 0; s < n; s++ {
			t := float64(s) / float64(n)
			t2, t3 := t*t, t*t*t
			f := func(a, b, c, d float64) float64 {
				return 0.5 * (2*b + (-a+c)*t + (2*a-5*b+4*c-d)*t2 + (-a+3*b-3*c+d)*t3)
			}
			out = append(out, pt{f(p0[0], p1[0], p2[0], p3[0]), f(p0[1], p1[1], p2[1], p3[1])})
		}
	}
	return append(out, pts[last])
}

func inPoly(x, z float64, poly []pt) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		xi, zi, xj, zj := poly[i][0], poly[i][1], poly[j][0], poly[j][1]
		if (zi > z) != (zj > z) && x < (xj-xi)*(z-zi)/(zj-zi)+xi {
			in = !in
		}
	}
	return in
}

func distSeg(x, z float64, a, b pt) float64 {
	dx, dz := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dz*dz
	if l2 == 0 {
		return math.Hypot(x-a[0], z-a[1])
	}
	t := math.Max(0, math.Min(1, ((x-a[0])*dx+(z-a[1])*dz)/l2))
	return math.Hypot(x-(a[0]+dx*t), z-(a[1]+dz*t))
}

// distLine is the distance to an open polyline.
func distLine(x, z float64, pts []pt) float64 {
	m := math.Inf(1)
	for i := 0; i+1 < len(pts); i++ {
		m = math.Min(m, distSeg(x, z, pts[i], pts[i+1]))
	}
	return m
}

// distPoly is the distance to the outline of a closed polygon.
func distPoly(x, z float64, poly []pt) float64 {
	m := math.Inf(1)
	for i := range poly {
		m = math.Min(m, distSeg(x, z, poly[i], poly[(i+1)%len(poly)]))
	}
	return m
}

// polysApart reports whether two convex polygons stay at least gap apart
// (separating axis test).
func polysApart(a, b []pt, gap float64) bool {
	for _, poly := range [][]pt{a, b} {
		for i := range poly {
			p, q := poly[i], poly[(i+1)%len(poly)]
			nx, nz := -(q[1] - p[1]), q[0]-p[0]
			l := math.Hypot(nx, nz)
			if l == 0 {
				continue
			}
			a0, a1 := project(a, nx/l, nz/l)
			b0, b1 := project(b, nx/l, nz/l)
			if a1+gap <= b0 || b1+gap <= a0 {
				return true
			}
		}
	}
	return false
}

func project(poly []pt, nx, nz float64) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range poly {
		d := v[0]*nx + v[1]*nz
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}
	return lo, hi
}

// arcAt is the point at fraction t of a polyline's length.
func arcAt(pts []pt, t float64) (p pt, tx, tz float64) {
	total := 0.0
	for i := 0; i+1 < len(pts); i++ {
		total += dist(pts[i], pts[i+1])
	}
	want := t * total
	for i := 0; i+1 < len(pts); i++ {
		l := dist(pts[i], pts[i+1])
		if want <= l || i+2 == len(pts) {
			u := math.Min(1, want/l)
			a, b := pts[i], pts[i+1]
			return pt{a[0] + (b[0]-a[0])*u, a[1] + (b[1]-a[1])*u}, (b[0] - a[0]) / l, (b[1] - a[1]) / l
		}
		want -= l
	}
	return pts[0], 1, 0
}

// polyLen is the length of an open polyline.
func polyLen(pts []pt) float64 {
	t := 0.0
	for i := 0; i+1 < len(pts); i++ {
		t += dist(pts[i], pts[i+1])
	}
	return t
}

// nearBoundary samples the outline of a convex quad, 0.2 apart, and the
// middle, and reports whether hit fires on any sample.
func nearBoundary(cs []pt, hit func(x, z float64) bool) bool {
	for i := range cs {
		a, b := cs[i], cs[(i+1)%len(cs)]
		n := int(dist(a, b)/0.2) + 1
		for k := 0; k <= n; k++ {
			u := float64(k) / float64(n)
			if hit(a[0]+(b[0]-a[0])*u, a[1]+(b[1]-a[1])*u) {
				return true
			}
		}
	}
	cx, cz := (cs[0][0]+cs[2][0])/2, (cs[0][1]+cs[2][1])/2
	return hit(cx, cz)
}
