package layout

import (
	"math"
	"testing"
)

func TestFootprintFixedAndDesks(t *testing.T) {
	cases := []struct {
		typ        string
		cols, rows int
		w, d       float64
	}{
		{"lobby", 0, 0, 3.8, 4.0},
		{"treehouse", 0, 0, 9.6, 4.2},
		{"cabin", 2, 1, 4.2, 2.8},
		{"research", 3, 2, 3*2.1 + 0.2, 2*3.5 + 0.2},
		{"workspace", 1, 1, 2.1 + 0.2, 3.5 + 0.2},
	}
	for _, c := range cases {
		w, d, ok := Footprint(c.typ, c.cols, c.rows)
		if !ok || math.Abs(w-c.w) > 1e-9 || math.Abs(d-c.d) > 1e-9 {
			t.Errorf("%s %dx%d = %v x %v ok=%v, want %v x %v", c.typ, c.cols, c.rows, w, d, ok, c.w, c.d)
		}
	}
}

func TestFootprintRefusals(t *testing.T) {
	for _, c := range []struct {
		typ        string
		cols, rows int
	}{
		{"castle", 0, 0}, {"lobby", 1, 1}, {"cabin", 0, 1}, {"cabin", 1, 0}, {"cabin", 9, 1}, {"cabin", 1, -1},
	} {
		if _, _, ok := Footprint(c.typ, c.cols, c.rows); ok {
			t.Errorf("%s %dx%d should have no footprint", c.typ, c.cols, c.rows)
		}
	}
}
