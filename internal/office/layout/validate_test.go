package layout

import (
	"math"
	"strings"
	"testing"
)

func TestValidLayoutPasses(t *testing.T) {
	if err := Validate(valid(), testCatalog()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name string
		mod  func(l *Layout)
		want string
	}{
		{"version", func(l *Layout) { l.V = 3 }, "version"},
		{"bad name", func(l *Layout) { l.Name = "../x" }, "name"},
		{"upper name", func(l *Layout) { l.Name = "Mine" }, "name"},
		{"long name", func(l *Layout) { l.Name = strings.Repeat("a", 41) }, "name"},
		{"long title", func(l *Layout) { l.Title = strings.Repeat("a", 61) }, "title"},
		{"grid", func(l *Layout) { l.Grid = 1 }, "grid"},
		{"size small", func(l *Layout) { l.Size.W = 7 }, "size"},
		{"size big", func(l *Layout) { l.Size.D = 121 }, "size"},
		{"wall colour", func(l *Layout) { l.Walls.Color = "red" }, "colour"},
		{"wall colour short", func(l *Layout) { l.Walls.Color = "#fff" }, "colour"},
		{"floor colour", func(l *Layout) { l.FloorColor = "url(http://x)" }, "colour"},
		{"room floor colour", func(l *Layout) { l.Rooms[1].Floor = "#GGGGGG" }, "colour"},
		{"door side", func(l *Layout) { l.Door.Side = "top" }, "door"},
		{"door past front", func(l *Layout) { l.Door.At = 20 }, "door"},
		{"door past left", func(l *Layout) { l.Door = Door{Side: "left", At: 12} }, "door"},
		{"deck outside", func(l *Layout) { l.Deck = &Rect{X: 15, Z: 0, W: 10, D: 6} }, "deck"},
		{"deck empty", func(l *Layout) { l.Deck = &Rect{X: 15, Z: 0, W: 0, D: 6} }, "deck"},
		{"stairs missing", func(l *Layout) { l.Stairs = nil }, "stairs"},
		{"stairs without deck", func(l *Layout) {
			l.Deck = nil
			l.Rooms = l.Rooms[:2]
			l.Furniture = l.Furniture[:1]
		}, "stairs"},
		{"stairs dir", func(l *Layout) { l.Stairs.Dir = "up" }, "stairs"},
		{"stairs outside", func(l *Layout) { l.Stairs.X = 20 }, "stairs"},
		{"stairs flight outside", func(l *Layout) { l.Stairs = &Stairs{X: 5, Z: 3, Dir: "s"} }, "stairs"},
		{"stairs landing outside", func(l *Layout) { l.Stairs = &Stairs{X: 5, Z: 1, Dir: "n"} }, "stairs"},
		{"no lobby", func(l *Layout) { l.Rooms[0].Type = "play" }, "lobby"},
		{"two lobbies", func(l *Layout) { l.Rooms[1].Type = "lobby" }, "lobby"},
		{"lobby on deck", func(l *Layout) { l.Rooms[0].Type = "play"; l.Rooms[2].Type = "lobby"; l.Rooms[2].Account = "" }, "lobby"},
		{"room type", func(l *Layout) { l.Rooms[1].Type = "dungeon" }, "type"},
		{"room id", func(l *Layout) { l.Rooms[1].ID = "R 2" }, "id"},
		{"dup room id", func(l *Layout) { l.Rooms[1].ID = "r1" }, "duplicate"},
		{"room level", func(l *Layout) { l.Rooms[1].Level = 2 }, "level"},
		{"room outside", func(l *Layout) { l.Rooms[1].X = 18 }, "inside"},
		{"room size zero", func(l *Layout) { l.Rooms[1].W = 0 }, "inside"},
		{"room negative", func(l *Layout) { l.Rooms[1].Z = -1 }, "inside"},
		{"room outside deck", func(l *Layout) { l.Rooms[2].X = 8 }, "inside"},
		{"rooms overlap", func(l *Layout) { l.Rooms[1].X = 4 }, "overlap"},
		{"long label", func(l *Layout) { l.Rooms[1].Label = strings.Repeat("a", 41) }, "label"},
		{"account on workspace", func(l *Layout) { l.Rooms[1].Account = "work" }, "account"},
		{"account bad", func(l *Layout) { l.Rooms[2].Account = "a b" }, "account"},
		{"account email", func(l *Layout) { l.Rooms[2].Account = "me@example.com" }, "account"},
		{"url label", func(l *Layout) { l.Rooms[1].Label = "see http://evil.example" }, "url"},
		{"url title", func(l *Layout) { l.Title = "//evil.example/x" }, "url"},
		{"url www", func(l *Layout) { l.Rooms[1].Label = "www.evil.example" }, "url"},
		{"url javascript", func(l *Layout) { l.Title = "javascript:alert(1)" }, "url"},
		{"url data", func(l *Layout) { l.Title = "data:text/html,x" }, "url"},
		{"unknown item", func(l *Layout) { l.Furniture[0].Item = "dragon" }, "item"},
		{"item wrong level", func(l *Layout) { l.Furniture[1].Item = "plant" }, "level"},
		{"furniture id", func(l *Layout) { l.Furniture[0].ID = "" }, "id"},
		{"dup furniture id", func(l *Layout) { l.Furniture[1].ID = "f1" }, "duplicate"},
		{"furniture rot", func(l *Layout) { l.Furniture[0].Rot = 45 }, "rot"},
		{"furniture outside", func(l *Layout) { l.Furniture[0].X = 19 }, "inside"},
		{"furniture outside deck", func(l *Layout) { l.Furniture[1].Z = 5 }, "inside"},
		{"furniture room unknown", func(l *Layout) { l.Furniture[0].Room = "nope" }, "room"},
		{"furniture room level", func(l *Layout) { l.Furniture[0].Room = "r3" }, "room"},
		{"furniture overlap", func(l *Layout) { l.Furniture = append(l.Furniture, Furn{ID: "f3", Item: "desk", X: 8, Z: 1}) }, "overlap"},
		{"too many rooms", func(l *Layout) {
			for i := 0; i < 64; i++ {
				l.Rooms = append(l.Rooms, Room{ID: "x" + string(rune('a'+i/26)) + string(rune('a'+i%26)), Type: "play", W: 1, D: 1})
			}
		}, "rooms"},
		{"too many furniture", func(l *Layout) {
			l.Size = Size{W: 120, D: 120}
			l.Furniture = nil
			for i := 0; i < 1501; i++ {
				l.Furniture = append(l.Furniture, Furn{ID: "f" + itoa(i), Item: "plant", X: i % 100, Z: i / 100})
			}
		}, "furniture"},
	}
	for _, c := range cases {
		l := valid()
		c.mod(&l)
		err := Validate(l, testCatalog())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", c.name, err, c.want)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for ; i > 0; i /= 10 {
		s = string(rune('0'+i%10)) + s
	}
	return s
}

// a 2x1 desk turned 90 degrees is a 1x2 box, so a neighbour that clears the
// flat desk can still collide with the turned one.
func TestFurnitureOverlapUsesTheRotatedFootprint(t *testing.T) {
	l := valid()
	l.Furniture = []Furn{
		{ID: "a", Item: "desk", X: 7, Z: 1, Rot: 90},
		{ID: "b", Item: "plant", X: 7, Z: 2},
	}
	if err := Validate(l, testCatalog()); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Errorf("rotated overlap not caught: %v", err)
	}
	l.Furniture[0].Rot = 0
	if err := Validate(l, testCatalog()); err != nil {
		t.Errorf("flat desk and plant do not overlap: %v", err)
	}
	// rotation can push the box out of the room too
	l.Furniture = []Furn{{ID: "a", Item: "desk", X: 7, Z: 11, Rot: 90}, {ID: "b", Item: "plant", X: 1, Z: 11}}
	if err := Validate(l, testCatalog()); err == nil || !strings.Contains(err.Error(), "inside") {
		t.Errorf("rotated box outside the deck level not caught: %v", err)
	}
}

func TestFurnitureOnDifferentLevelsMayShareACell(t *testing.T) {
	l := valid()
	l.Furniture = []Furn{{ID: "a", Item: "desk", X: 11, Z: 1}, {ID: "b", Item: "desk", Level: 1, X: 11, Z: 1}}
	if err := Validate(l, testCatalog()); err != nil {
		t.Error(err)
	}
}

func TestNoDeckMeansNoUpperLevel(t *testing.T) {
	l := valid()
	l.Deck, l.Stairs = nil, nil
	l.Rooms = l.Rooms[:2]
	l.Furniture = l.Furniture[:1]
	if err := Validate(l, testCatalog()); err != nil {
		t.Fatal(err)
	}
	l.Furniture = append(l.Furniture, Furn{ID: "up", Item: "desk", Level: 1, X: 11, Z: 1})
	if err := Validate(l, testCatalog()); err == nil {
		t.Error("furniture on level 1 without a deck passed")
	}
}

func TestEmptyCatalogRefusesFurniture(t *testing.T) {
	if err := Validate(valid(), Catalog{}); err == nil || !strings.Contains(err.Error(), "item") {
		t.Errorf("err = %v", err)
	}
}

func TestParseCatalogRefusals(t *testing.T) {
	for _, raw := range []string{`nope`, `[{"id":"a b","footprint":{"w":1,"d":1}}]`, `[{"id":"a","footprint":{"w":0,"d":1}}]`,
		`[{"id":"a","footprint":{"w":1,"d":1}},{"id":"a","footprint":{"w":1,"d":1}}]`, `[{"id":"a","footprint":{"w":1,"d":1},"x":1}]`} {
		if _, err := ParseCatalog([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestStairsInEveryDirection(t *testing.T) {
	for _, st := range []Stairs{{X: 10, Z: 3, Dir: "n"}, {X: 10, Z: 6, Dir: "s"}, {X: 8, Z: 6, Dir: "e"}, {X: 4, Z: 6, Dir: "w"}} {
		l := valid()
		l.Stairs = &st
		if err := Validate(l, testCatalog()); err != nil {
			t.Errorf("%+v: %v", st, err)
		}
	}
}

func TestFloorItemsMayLieUnderOthers(t *testing.T) {
	l := valid()
	l.Furniture = []Furn{{ID: "a", Item: "rug", X: 6, Z: 0}, {ID: "b", Item: "desk", X: 7, Z: 1}, {ID: "c", Item: "rug", X: 6, Z: 1}}
	if err := Validate(l, testCatalog()); err != nil {
		t.Error(err)
	}
}

// huge and negative numbers must be refused before any arithmetic can wrap.
func TestValidateRefusesOutOfRangeNumbers(t *testing.T) {
	vals := []int{math.MaxInt64, math.MinInt64, -1, MaxSide + 1, math.MaxInt64 - 1}
	fields := map[string]func(l *Layout, v int){
		"deck.x":   func(l *Layout, v int) { l.Deck.X = v },
		"deck.z":   func(l *Layout, v int) { l.Deck.Z = v },
		"deck.w":   func(l *Layout, v int) { l.Deck.W = v },
		"deck.d":   func(l *Layout, v int) { l.Deck.D = v },
		"stairs.x": func(l *Layout, v int) { l.Stairs.X = v },
		"stairs.z": func(l *Layout, v int) { l.Stairs.Z = v },
		"room.x":   func(l *Layout, v int) { l.Rooms[1].X = v },
		"room.z":   func(l *Layout, v int) { l.Rooms[1].Z = v },
		"room.w":   func(l *Layout, v int) { l.Rooms[1].W = v },
		"room.d":   func(l *Layout, v int) { l.Rooms[1].D = v },
		"lobby.w":  func(l *Layout, v int) { l.Rooms[0].W = v },
		"lobby.x":  func(l *Layout, v int) { l.Rooms[0].X = v },
		"furn.x":   func(l *Layout, v int) { l.Furniture[0].X = v },
		"furn.z":   func(l *Layout, v int) { l.Furniture[0].Z = v },
		"size.w":   func(l *Layout, v int) { l.Size.W = v },
		"size.d":   func(l *Layout, v int) { l.Size.D = v },
		"door.at":  func(l *Layout, v int) { l.Door.At = v },
	}
	for name, set := range fields {
		for _, v := range vals {
			l := valid()
			set(&l, v)
			if err := Validate(l, testCatalog()); err == nil {
				t.Errorf("%s = %d passed", name, v)
			}
		}
	}
}

func TestReservedWindowsNamesAreNotLayoutNames(t *testing.T) {
	for _, n := range []string{"con", "prn", "aux", "nul", "com1", "com9", "lpt1", "lpt9"} {
		if ValidName(n) {
			t.Errorf("%s is a valid name", n)
		}
	}
	for _, n := range []string{"com", "com10", "console", "lpt0", "my-con"} {
		if !ValidName(n) {
			t.Errorf("%s should be valid", n)
		}
	}
}
