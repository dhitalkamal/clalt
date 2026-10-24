package garden

import (
	"container/heap"
	"math"
	"sort"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// router lays walkways on a half unit grid: it grows a network from the
// lobby gate and connects each door to the nearest part of it, around
// buildings and water, over bridges.
const cellSize = 0.5

var moves = [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

// turnCost by how far a move turns from the last one, in 45 degree steps.
var turnCost = [5]float64{0, 0.6, 1.8, 3, 4}

const bridgeCost = 6.0

type router struct {
	nx, nz  int
	blocked []bool
	net     []bool
	tele    map[int]int
	dist    []float64
	parent  []int32
	touched []int
}

func newRouter(s *scene, w, d float64) *router {
	r := &router{nx: int(w/cellSize) + 1, nz: int(d/cellSize) + 1, tele: map[int]int{}}
	n := r.nx * r.nz
	r.blocked, r.net = make([]bool, n), make([]bool, n)
	for j := 0; j < r.nz; j++ {
		for i := 0; i < r.nx; i++ {
			x, z := float64(i)*cellSize, float64(j)*cellSize
			r.blocked[j*r.nx+i] = !s.dryEnough(x, z)
		}
	}
	for _, b := range s.bridges {
		ia, ja := cellOf(b.A)
		ib, jb := cellOf(b.B)
		a, c := r.id(ia, ja), r.id(ib, jb)
		r.tele[a], r.tele[c] = c, a
	}
	return r
}

// dryEnough is the walkable test: inside the island, off water, 0.7 clear of
// every building.
func (s *scene) dryEnough(x, z float64) bool {
	if !inPoly(x, z, s.base) || distPoly(x, z, s.base) < 1.0 {
		return false
	}
	if s.river != nil && distLine(x, z, s.river.Pts) < s.river.W/2+0.7 {
		return false
	}
	for _, p := range s.ponds {
		if inEllipse(x, z, p, 0.7) {
			return false
		}
	}
	for _, p := range s.pavs {
		if distToPav(p, x, z) < 0.7 {
			return false
		}
	}
	return true
}

func cellOf(p pt) (int, int) {
	return int(math.Round(p[0] / cellSize)), int(math.Round(p[1] / cellSize))
}

func (r *router) id(i, j int) int { return j*r.nx + i }

func (r *router) ok(i, j int) bool {
	return i >= 0 && j >= 0 && i < r.nx && j < r.nz && !r.blocked[r.id(i, j)]
}

func centre(i, j int) pt { return pt{float64(i) * cellSize, float64(j) * cellSize} }

// outward is the unit direction a door faces.
func outward(p layout.Pavilion) pt { return rotPt(0, 0, p.Rot, 0, 1) }

// startCell is the grid cell half a unit out from a door.
func startCell(p layout.Pavilion) (int, int) {
	o, d := outward(p), door(p)
	return cellOf(pt{d[0] + o[0]*cellSize, d[1] + o[1]*cellSize})
}

type item struct {
	c, d int
	cost float64
}
type queue []item

func (q queue) Len() int           { return len(q) }
func (q queue) Less(a, b int) bool { return q[a].cost < q[b].cost }
func (q queue) Swap(a, b int)      { q[a], q[b] = q[b], q[a] }
func (q *queue) Push(x any)        { *q = append(*q, x.(item)) }
func (q *queue) Pop() any          { o := *q; v := o[len(o)-1]; *q = o[:len(o)-1]; return v }

// search is a multi source Dijkstra from every network cell outward. it
// stops at the first cell in targets it settles and returns that state.
func (r *router) search(targets map[int]bool) (state int, found bool) {
	n := r.nx * r.nz * 9
	if r.dist == nil {
		r.dist = make([]float64, n)
		r.parent = make([]int32, n)
		for i := range r.dist {
			r.dist[i], r.parent[i] = math.Inf(1), -1
		}
	}
	for _, t := range r.touched {
		r.dist[t], r.parent[t] = math.Inf(1), -1
	}
	r.touched = r.touched[:0]
	q := &queue{}
	for c, on := range r.net {
		if on {
			r.dist[c*9+8] = 0
			r.touched = append(r.touched, c*9+8)
			heap.Push(q, item{c, 8, 0})
		}
	}
	relax := func(from item, to, nd int, cost float64) {
		if cost < r.dist[to*9+nd] {
			if math.IsInf(r.dist[to*9+nd], 1) {
				r.touched = append(r.touched, to*9+nd)
			}
			r.dist[to*9+nd] = cost
			r.parent[to*9+nd] = int32(from.c*9 + from.d)
			heap.Push(q, item{to, nd, cost})
		}
	}
	for q.Len() > 0 {
		cur := heap.Pop(q).(item)
		if cur.cost > r.dist[cur.c*9+cur.d] {
			continue
		}
		if targets[cur.c] && !r.net[cur.c] {
			return cur.c*9 + cur.d, true
		}
		ci, cj := cur.c%r.nx, cur.c/r.nx
		if to, ok := r.tele[cur.c]; ok {
			relax(cur, to, 8, cur.cost+bridgeCost)
		}
		for nd, m := range moves {
			ni, nj := ci+m[0], cj+m[1]
			if !r.ok(ni, nj) {
				continue
			}
			step := 1.0
			if m[0] != 0 && m[1] != 0 {
				if !r.ok(ci+m[0], cj) || !r.ok(ci, cj+m[1]) {
					continue
				}
				step = math.Sqrt2
			}
			if r.net[r.id(ni, nj)] {
				step *= 0.25
			}
			turn := 0.0
			if cur.d != 8 {
				k := abs8(nd - cur.d)
				turn = turnCost[min(k, 8-k)]
			}
			relax(cur, r.id(ni, nj), nd, cur.cost+step+turn)
		}
	}
	return 0, false
}

func abs8(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// chain follows parents from a settled state to the first network cell.
func (r *router) chain(state int) []int {
	out := []int{state / 9}
	s := int32(state)
	for !r.net[out[len(out)-1]] {
		s = r.parent[s]
		if s < 0 {
			break
		}
		out = append(out, int(s)/9)
	}
	return out
}

// routeAll connects every pavilion door to the network, nearest first, and
// adds the paths to the scene. it reports false when a door cannot be reached.
func (r *router) routeAll(s *scene, gate layout.Pavilion) bool {
	gi, gj := startCell(gate)
	r.free(gi, gj)
	r.net[r.id(gi, gj)] = true
	s.path(1.0, false, door(gate), centre(gi, gj))
	left := map[int][]layout.Pavilion{}
	targets := map[int]bool{}
	for _, p := range s.pavs {
		if p.ID == gate.ID {
			continue
		}
		i, j := startCell(p)
		r.free(i, j)
		left[r.id(i, j)] = append(left[r.id(i, j)], p)
		targets[r.id(i, j)] = true
	}
	done := func(cell int, chain []int) {
		for _, p := range left[cell] {
			r.addRoute(s, p, chain)
		}
		delete(left, cell)
		delete(targets, cell)
	}
	for len(left) > 0 {
		// doors whose start cell is already on the network need no search
		hit := false
		for _, cell := range sortedKeys(left) {
			if r.net[cell] {
				done(cell, []int{cell})
				hit = true
				break
			}
		}
		if hit {
			continue
		}
		state, ok := r.search(targets)
		if !ok {
			return false
		}
		done(state/9, r.chain(state))
	}
	return true
}

// free unblocks a cell and the ones along a door's outward line so a path can
// leave the door even when a neighbour crowds it.
func (r *router) free(i, j int) { r.blocked[r.id(i, j)] = false }

// addRoute turns a cell chain into paths, cutting at bridges, and joins its
// cells to the network.
func (r *router) addRoute(s *scene, p layout.Pavilion, cells []int) {
	var piece []pt
	flush := func(first bool) {
		piece = simplify(piece)
		if first {
			if dist(piece[0], door(p)) > 0.01 {
				piece = append([]pt{door(p)}, piece...)
			}
		}
		if len(piece) >= 2 {
			s.path(0.9, false, piece...)
		}
		piece = nil
	}
	first := true
	for k, c := range cells {
		i, j := c%r.nx, c/r.nx
		if k > 0 {
			pi, pj := cells[k-1]%r.nx, cells[k-1]/r.nx
			if abs8(i-pi) > 1 || abs8(j-pj) > 1 {
				flush(first)
				first = false
			}
		}
		piece = append(piece, centre(i, j))
		r.net[c] = true
	}
	flush(first)
}

// simplify drops points that lie on a straight run.
func simplify(pts []pt) []pt {
	if len(pts) < 3 {
		return pts
	}
	out := []pt{pts[0]}
	for k := 1; k+1 < len(pts); k++ {
		a, b, c := out[len(out)-1], pts[k], pts[k+1]
		if (b[0]-a[0])*(c[1]-b[1])-(b[1]-a[1])*(c[0]-b[0]) != 0 {
			out = append(out, b)
		}
	}
	return append(out, pts[len(pts)-1])
}

func sortedKeys(m map[int][]layout.Pavilion) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
