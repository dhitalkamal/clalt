package garden

import "math"

// bankPolys splits the island rectangle along the river into its two banks.
func bankPolys(line []pt, w, d float64) [][]pt {
	r := rectPoly(w/2, d/2, w, d, islandR, 4)
	edgeW, edgeE := offsetLine(line, riverHW), offsetLine(line, -riverHW)
	clamp := func(ps []pt) []pt {
		out := make([]pt, len(ps))
		for i, p := range ps {
			out[i] = pt{p[0], math.Max(0, math.Min(d, p[1]))}
		}
		return out
	}
	sides := make([]float64, len(r))
	for i, p := range r {
		sides[i] = sideOf(line, p[0], p[1])
	}
	run := func(want float64) []pt {
		n, i0 := len(r), 0
		for i := 0; i < n; i++ {
			if sides[i] == want && sides[(i+n-1)%n] != want {
				i0 = i
			}
		}
		var out []pt
		for k := 0; k < n; k++ {
			i := (i0 + k) % n
			if sides[i] != want {
				break
			}
			out = append(out, r[i])
		}
		return out
	}
	west := sideOf(line, -1000, 0)
	rev := make([]pt, len(edgeE))
	for i, p := range edgeE {
		rev[len(edgeE)-1-i] = p
	}
	return [][]pt{
		append(run(west), clamp(edgeW)...),
		append(run(-west), clamp(rev)...),
	}
}

// sideOf says which side of a polyline a point is on, by the nearest segment.
func sideOf(line []pt, x, z float64) float64 {
	best, side := math.Inf(1), 1.0
	for i := 0; i+1 < len(line); i++ {
		a, b := line[i], line[i+1]
		dx, dz := b[0]-a[0], b[1]-a[1]
		t := math.Max(0, math.Min(1, ((x-a[0])*dx+(z-a[1])*dz)/(dx*dx+dz*dz)))
		px, pz := a[0]+dx*t, a[1]+dz*t
		if dd := math.Hypot(x-px, z-pz); dd < best {
			best = dd
			side = -1
			if (x-px)*-dz+(z-pz)*dx > 0 {
				side = 1
			}
		}
	}
	return side
}

// offsetLine moves a polyline sideways by d, positive to the right when
// walking along it with z pointing south.
func offsetLine(pts []pt, d float64) []pt {
	out := make([]pt, len(pts))
	for i, p := range pts {
		a, b := pts[max(0, i-1)], pts[min(len(pts)-1, i+1)]
		l := math.Hypot(b[0]-a[0], b[1]-a[1])
		tx, tz := (b[0]-a[0])/l, (b[1]-a[1])/l
		out[i] = pt{p[0] - tz*d, p[1] + tx*d}
	}
	return out
}
