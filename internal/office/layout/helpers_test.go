package layout

import (
	"encoding/json"
	"testing"
)

// testCatalog is a small catalog of our own, so the tests do not depend on the
// real catalog.json the page builder writes.
func testCatalog() Catalog {
	cat, err := ParseCatalog([]byte(`[
	  {"id":"desk","footprint":{"w":2,"d":1},"levels":[0,1],"tags":["seat"]},
	  {"id":"plant","footprint":{"w":1,"d":1},"levels":[0]},
	  {"id":"rug","footprint":{"w":4,"d":3},"levels":[0,1],"tags":["floor"]},
	  {"id":"sofa","footprint":{"w":3,"d":2},"levels":[0,1]}
	]`))
	if err != nil {
		panic(err)
	}
	return cat
}

// valid returns a layout that passes Validate against testCatalog.
func valid() Layout {
	return Layout{
		V: 1, Name: "mine", Title: "Mine", Grid: 0.5,
		Size:       Size{W: 20, D: 12},
		Walls:      Walls{Color: "#e9e4dc"},
		FloorColor: "#d9b88f",
		Door:       Door{Side: "front", At: 2},
		Deck:       &Rect{X: 10, Z: 0, W: 10, D: 6},
		Stairs:     &Stairs{X: 17, Z: 6, Dir: "s"},
		Rooms: []Room{
			{ID: "r1", Type: "lobby", X: 0, Z: 0, W: 5, D: 5, Label: "Lobby"},
			{ID: "r2", Type: "workspace", X: 6, Z: 0, W: 4, D: 5, Label: "Open", Floor: "#aabbcc"},
			{ID: "r3", Type: "cabins", Level: 1, X: 10, Z: 0, W: 5, D: 5, Label: "Cabins", Account: "work"},
		},
		Furniture: []Furn{
			{ID: "f1", Item: "desk", X: 7, Z: 1, Rot: 0, Room: "r2"},
			{ID: "f2", Item: "desk", Level: 1, X: 11, Z: 1, Rot: 90},
		},
	}
}

func mustJSON(t *testing.T, l Layout) []byte {
	t.Helper()
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
