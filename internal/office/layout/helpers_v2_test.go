package layout

import "os"

// square is an axis aligned square outline around the origin.
func square(h float64) []Pt { return []Pt{{-h, -h}, {h, -h}, {h, h}, {-h, h}} }

// valid2 is a garden layout that passes Validate: a lawn with a river down
// the east side, a pond, and three pavilions on the west.
func valid2() Layout {
	return Layout{
		V: Version, Name: "garden", Title: "Garden", Template: "",
		Site: &Site{
			Base:    Poly{Poly: square(20)},
			Land:    []Poly{{Poly: square(19)}},
			River:   &River{Pts: []Pt{{12, -18}, {12, 18}}, W: 3},
			Ponds:   []Pond{{X: 0, Z: 10, RX: 2, RZ: 1}},
			Paths:   []Path{{Pts: []Pt{{-10, 0}, {0, 0}}, W: 1, Smooth: true}},
			Bridges: []Bridge{{A: Pt{9, 0}, B: Pt{15, 0}, W: 1.3}},
			Props:   []Prop{{T: "bench", X: 1, Z: 1}, {T: "bed", X: 2, Z: 2, W: 2, D: 1}},
			Trees:   Trees{Mode: "edge", Band: 3.2, Seed: 23},
			Fence:   true,
		},
		Pavilions: []Pavilion{
			{ID: "lobby", Type: "lobby", X: 0, Z: -10},
			{ID: "work-cabin", Type: "cabin", X: -10, Z: -10, Account: "work", Cols: 2, Rows: 1},
			{ID: "ws", Type: "workspace", X: -10, Z: 5, Rot: 90, Label: "WORK", Cols: 2, Rows: 1},
		},
		Overflow: []Spot{{X: 0, Z: -5, Near: "lobby"}},
		Eye:      &Eye{From: Pt{0, 15}, Look: Pt{0, 0}},
	}
}

// writeUser puts raw bytes in the user folder, skipping Save's checks.
func writeUser(s *Store, name string, raw []byte) error {
	if err := os.MkdirAll(s.dir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path(name), raw, 0o600)
}
