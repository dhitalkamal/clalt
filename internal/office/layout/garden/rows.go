package garden

import (
	"slices"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// rows: cabins in rows across the top (six to a row), the treehouse beside
// them, desk pavilions, cafe and play in lines below. every pavilion faces
// south onto a lane; a trunk on each side ties the lanes together and the
// lobby sits at the west end.
const (
	rowsPerLine = 6
	cabinGap    = 0.6
	midGap      = 1.5
	laneBelow   = 1.5 // pavilion bottom to its lane
	laneAbove   = 1.4 // lane to the next band top
	treeGap     = 3.2 // last cabin to the treehouse
	startX      = 1.0 // trunk to the first pavilion
)

func genRows(p plan) layout.Layout {
	s := newScene("rows", "Garden rows", 3.2, 41)
	var lanes []float64 // z of every lane that carries spurs
	laneEnd := map[float64]float64{}
	type run struct {
		z, x0, x1 float64
		spurs     []layout.Pavilion
	}
	var runs []run

	// band 0: cabin rows, treehouse beside the first
	z, cabinRight := 0.0, startX
	for i := 0; i < len(p.cabins); i += rowsPerLine {
		row := p.cabins[i:min(i+rowsPerLine, len(p.cabins))]
		rowD, x := 0.0, startX
		for _, c := range row {
			w, d := size(c)
			c.X, c.Z, c.Rot = x+w/2, z+d/2, 0
			s.pav(c)
			rowD = max(rowD, d)
			x += w + cabinGap
		}
		cabinRight = max(cabinRight, x-cabinGap)
		lane := z + rowD + laneBelow
		runs = append(runs, run{z: lane, x0: 0, x1: x - cabinGap + 0.5, spurs: s.pavs[len(s.pavs)-len(row):]})
		z = lane + laneAbove
	}
	th := layout.Pavilion{ID: "treehouse", Type: "treehouse"}
	tw, td := size(th)
	thX := startX
	if len(p.cabins) > 0 {
		thX = cabinRight + treeGap
	}
	th.X, th.Z = thX+tw/2, td/2
	s.pav(th)
	if len(p.cabins) == 0 {
		z = td + 2.8
	}
	topWidth := thX + tw - startX

	// mid band: research, workspace, cafe, play flow left to right in lines
	items := slices.Concat(p.research, p.workspace, p.team,
		[]layout.Pavilion{{ID: "cafe", Type: "cafe"}, {ID: "play", Type: "play"}})
	limit := max(topWidth, 30)
	for i := 0; i < len(items); {
		x, lineD, j := startX, 0.0, i
		for ; j < len(items); j++ {
			w, d := size(items[j])
			if j > i && x+w-startX > limit {
				break
			}
			items[j].X, items[j].Z = x+w/2, z+d/2
			lineD = max(lineD, d)
			x += w + midGap
		}
		line := items[i:j]
		for _, it := range line {
			s.pav(it)
		}
		lane := z + lineD + laneBelow
		runs = append(runs, run{z: lane, x0: 0, x1: -1, spurs: s.pavs[len(s.pavs)-len(line):]})
		z = lane + laneAbove
		i = j
	}
	lastLane := z - laneAbove

	// right trunk sits past the widest pavilion
	xR := startX
	for _, pv := range s.pavs {
		for _, c := range corners(pv) {
			xR = max(xR, c[0]+1.0)
		}
	}
	for _, r := range runs {
		x1 := r.x1
		if x1 < 0 {
			x1 = xR
		}
		for _, sp := range r.spurs {
			d := door(sp)
			s.spur(sp, pt{d[0], r.z})
		}
		s.path(1.0, false, pt{0, r.z}, pt{x1, r.z})
		lanes = append(lanes, r.z)
		laneEnd[r.z] = x1
	}

	// lobby at the west end, its door onto the left trunk
	lobby := layout.Pavilion{ID: "lobby", Type: "lobby", Rot: 90}
	lobbyZ := min(lastLane/2, lastLane-1)
	lobby.X, lobby.Z = -0.7-2.02, lobbyZ
	s.pav(lobby)
	top := min(lanes[0], lobbyZ)
	s.path(1.0, false, pt{0, top}, pt{0, lastLane})
	s.spur(lobby, pt{0, lobbyZ})

	// treehouse stair foot to the right trunk, which starts level with it
	d := door(th)
	s.path(0.9, false, d, pt{xR, d[1]}, pt{xR, lastLane})

	// meadow below the last lane with the pond
	baseBottom := lastLane + 5.0
	s.ponds = append(s.ponds, layout.Pond{X: 8, Z: lastLane + 2.6, RX: 3, RZ: 1.1})
	minX, minZ := -4.8, -2.6
	s.base = chaikin(rectPoly((minX-2.6+xR+2.6)/2, (minZ+baseBottom)/2, xR+2.6-(minX-2.6), baseBottom-minZ, 2.2, 3), 1)

	s.prop("bench", 3.5, lastLane+2.6, 180, 0.5)
	s.prop("bench", 13, lastLane+3.4, 180, 0.5)
	s.bed(xR-3, lastLane+3.2, 1.6, 0.6)
	s.bed(-1.5, lastLane+3.4, 1.4, 0.6)
	s.prop("lamp", -0.9, lanes[0]-0.9, 0, 0.3)
	s.prop("lamp", -0.9, lastLane+0.9, 0, 0.3)
	s.prop("lamp", xR+0.9, lastLane-3, 0, 0.3)
	s.prop("lamp", xR+0.9, lastLane+0.9, 0, 0.3)
	s.addSpots()
	s.eyeAt()
	return s.finish("rows")
}
