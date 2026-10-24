package garden

import (
	"math"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// packer is the free-space mask of the island for first fit placement, with a
// summed area table so a rectangle test costs four lookups.
type packer struct {
	w, d    float64
	nx, nz  int
	blocked []bool
	sat     []int32
}

func newPacker(s *scene, w, d float64, westN int) *packer {
	pk := &packer{w: w, d: d, nx: int(w/cellSize) + 1, nz: int(d/cellSize) + 1}
	pk.blocked = make([]bool, pk.nx*pk.nz)
	west := sideOf(s.river.Pts, -1000, 0)
	for j := 0; j < pk.nz; j++ {
		for i := 0; i < pk.nx; i++ {
			x, z := float64(i)*cellSize, float64(j)*cellSize
			bad := x < edgeMargin || z < edgeMargin || x > w-edgeMargin || z > d-edgeMargin ||
				distLine(x, z, s.river.Pts) < riverHW+edgeMargin || sideOf(s.river.Pts, x, z) == west
			for _, b := range s.bridges {
				if dist(pt{x, z}, b.A) < bridgeZone || dist(pt{x, z}, b.B) < bridgeZone {
					bad = true
				}
			}
			pk.blocked[j*pk.nx+i] = bad
		}
	}
	for _, pv := range s.pavs[:westN] {
		pk.mark(pv)
	}
	pk.build()
	return pk
}

func (pk *packer) build() {
	pk.sat = make([]int32, (pk.nx+1)*(pk.nz+1))
	for j := 0; j < pk.nz; j++ {
		for i := 0; i < pk.nx; i++ {
			v := int32(0)
			if pk.blocked[j*pk.nx+i] {
				v = 1
			}
			pk.sat[(j+1)*(pk.nx+1)+i+1] = v + pk.sat[j*(pk.nx+1)+i+1] + pk.sat[(j+1)*(pk.nx+1)+i] - pk.sat[j*(pk.nx+1)+i]
		}
	}
}

// span is the cell range a coordinate interval touches.
func (pk *packer) span(lo, hi float64, n int) (int, int) {
	a, b := int(math.Floor(lo/cellSize+1e-9)), int(math.Ceil(hi/cellSize-1e-9))
	return max(a, 0), min(b, n-1)
}

func (pk *packer) mark(pv layout.Pavilion) {
	w, d := size(pv)
	if pv.Rot == 90 {
		w, d = d, w
	}
	pk.markRect(pv.X-w/2-packGap, pv.X+w/2+packGap, pv.Z-d/2-packGap, pv.Z+d/2+packGap)
}

func (pk *packer) markRect(x0, x1, z0, z1 float64) {
	i0, i1 := pk.span(x0, x1, pk.nx)
	j0, j1 := pk.span(z0, z1, pk.nz)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			pk.blocked[j*pk.nx+i] = true
		}
	}
}

func (pk *packer) clear(x0, x1, z0, z1 float64) bool {
	i0, i1 := pk.span(x0, x1, pk.nx)
	j0, j1 := pk.span(z0, z1, pk.nz)
	if x0 < 0 || z0 < 0 || x1 > pk.w || z1 > pk.d {
		return false
	}
	st := pk.nx + 1
	n := pk.sat[(j1+1)*st+i1+1] - pk.sat[j0*st+i1+1] - pk.sat[(j1+1)*st+i0] + pk.sat[j0*st+i0]
	return n == 0
}

// fit finds the first free spot for a w by d rectangle. tr scans from the top
// right going down, br from the bottom right going up, bl from the left going
// up. it marks the spot taken and returns the centre.
func (pk *packer) fit(w, d float64, mode string, zFrom float64) (float64, float64, bool) {
	try := func(x0, z0 float64) bool { return pk.clear(x0, x0+w, z0, z0+d) }
	take := func(x0, z0 float64) (float64, float64, bool) {
		pk.markRect(x0-packGap, x0+w+packGap, z0-packGap, z0+d+packGap)
		pk.build()
		return x0 + w/2, z0 + d/2, true
	}
	// doors face south, so the bottom row keeps its front clear too
	lo, hi := edgeMargin, pk.d-edgeMargin-packGap-d
	switch mode {
	case "tr", "br":
		for xr := pk.w - edgeMargin; xr-w >= edgeMargin; xr -= cellSize {
			if mode == "tr" {
				for z := math.Max(lo, math.Ceil(zFrom/cellSize)*cellSize); z <= hi; z += cellSize {
					if try(xr-w, z) {
						return take(xr-w, z)
					}
				}
			} else {
				for z := math.Floor(hi/cellSize) * cellSize; z >= lo; z -= cellSize {
					if try(xr-w, z) {
						return take(xr-w, z)
					}
				}
			}
		}
	case "bl":
		for xl := edgeMargin; xl+w <= pk.w-edgeMargin; xl += cellSize {
			for z := math.Floor(hi/cellSize) * cellSize; z >= lo; z -= cellSize {
				if try(xl, z) {
					return take(xl, z)
				}
			}
		}
	}
	return 0, 0, false
}
