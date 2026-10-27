package garden

import "github.com/dhitalkamal/clalt/internal/office/layout"

// size is the footprint (w, d) of a pavilion before rotation, from the
// shared footprint table of the layout package.
func size(p layout.Pavilion) (w, d float64) {
	w, d, _ = layout.Footprint(p.Type, p.Cols, p.Rows)
	return w, d
}

// Desks is how many desks a pavilion holds. a cabin row is one line of desks;
// a research or workspace row is a face to face pair of lines, which is what
// the 3.5 row depth in the footprint table holds.
func Desks(p layout.Pavilion) int {
	switch p.Type {
	case "cabin":
		return p.Cols * p.Rows
	case "research", "workspace", "team":
		return p.Cols * p.Rows * 2
	}
	return 0
}

// door is where a path meets the pavilion: the middle of the front edge, or
// the stair foot at the east end for the treehouse.
func door(p layout.Pavilion) pt {
	w, d := size(p)
	if p.Type == "treehouse" {
		return rotPt(p.X, p.Z, p.Rot, w/2, d/2+0.02)
	}
	return rotPt(p.X, p.Z, p.Rot, 0, d/2+0.02)
}

// corners of the footprint in world space.
func corners(p layout.Pavilion) []pt {
	w, d := size(p)
	return []pt{
		rotPt(p.X, p.Z, p.Rot, -w/2, -d/2), rotPt(p.X, p.Z, p.Rot, w/2, -d/2),
		rotPt(p.X, p.Z, p.Rot, w/2, d/2), rotPt(p.X, p.Z, p.Rot, -w/2, d/2),
	}
}

// distToPav is the distance from a point to the footprint, 0 inside.
func distToPav(p layout.Pavilion, x, z float64) float64 {
	w, d := size(p)
	q := rotPt(0, 0, -p.Rot, x-p.X, z-p.Z)
	dx, dz := max(abs(q[0])-w/2, 0), max(abs(q[1])-d/2, 0)
	return dist(pt{dx, dz}, pt{})
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
