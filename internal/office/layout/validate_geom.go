package layout

import (
	"fmt"
)

// floor is the ground level as a rect.
func (l Layout) floor() Rect { return Rect{W: l.Size.W, D: l.Size.D} }

// bounds is the area a level offers: the floor, the deck, or nothing.
func (l Layout) bounds(level int) (Rect, bool) {
	switch {
	case level == 0:
		return l.floor(), true
	case level == 1 && l.Deck != nil:
		return *l.Deck, true
	}
	return Rect{}, false
}

// inCells and inSpan bound a coordinate pair and a size pair before any sum
// is taken, so nothing can wrap around.
func inCells(x, z int) bool { return x >= 0 && x <= MaxSide && z >= 0 && z <= MaxSide }
func inSpan(w, d int) bool  { return w >= 1 && w <= MaxSide && d >= 1 && d <= MaxSide }

// contains says whether o lies inside r. an empty o never does.
func (r Rect) contains(o Rect) bool {
	return inCells(o.X, o.Z) && inSpan(o.W, o.D) && o.W >= 1 && o.D >= 1 && o.X >= r.X && o.Z >= r.Z && o.X+o.W <= r.X+r.W && o.Z+o.D <= r.Z+r.D
}

// overlaps says whether two half open boxes share a cell.
func (r Rect) overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Z < o.Z+o.D && o.Z < r.Z+r.D
}

// flight and landing cells of stairs. x,z is the top most cell of a flight 2
// wide and 6 long, dir is the way up (n is toward smaller z, e toward larger
// x) and the flight runs the other way from the top. a 2x2 landing sits past
// the top.
const (
	flightWide = 2
	flightLong = 6
)

func (s Stairs) rects() (flight, landing Rect) {
	switch s.Dir {
	case "n":
		return Rect{s.X, s.Z, flightWide, flightLong}, Rect{s.X, s.Z - 2, 2, 2}
	case "s":
		return Rect{s.X, s.Z - flightLong + 1, flightWide, flightLong}, Rect{s.X, s.Z + 1, 2, 2}
	case "e":
		return Rect{s.X - flightLong + 1, s.Z, flightLong, flightWide}, Rect{s.X + 1, s.Z, 2, 2}
	}
	return Rect{s.X, s.Z, flightLong, flightWide}, Rect{s.X - 2, s.Z, 2, 2}
}

func validateRooms(l Layout) error {
	seen := map[string]bool{}
	lobbies := 0
	for _, r := range l.Rooms {
		if !idRe.MatchString(r.ID) {
			return fmt.Errorf("room id %q must be 1 to 32 characters of a-z, 0-9 and -", r.ID)
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate room id %s", r.ID)
		}
		seen[r.ID] = true
		if !roomTypes[r.Type] {
			return fmt.Errorf("room %s has an unknown type %q", r.ID, r.Type)
		}
		if r.Type == "lobby" && r.Level == 0 {
			lobbies++
		}
		if r.Type == "lobby" && r.Level != 0 {
			return fmt.Errorf("the lobby room %s must be on level 0", r.ID)
		}
		if err := validateRoom(l, r); err != nil {
			return err
		}
	}
	if lobbies != 1 {
		return fmt.Errorf("the layout needs exactly one lobby room on level 0, found %d", lobbies)
	}
	for i, a := range l.Rooms {
		for _, b := range l.Rooms[i+1:] {
			if a.Level == b.Level && a.rect().overlaps(b.rect()) {
				return fmt.Errorf("rooms %s and %s overlap", a.ID, b.ID)
			}
		}
	}
	return nil
}

func (r Room) rect() Rect { return Rect{X: r.X, Z: r.Z, W: r.W, D: r.D} }

func validateRoom(l Layout, r Room) error {
	area, ok := l.bounds(r.Level)
	if !ok {
		return fmt.Errorf("room %s is on level %d, which this layout does not have", r.ID, r.Level)
	}
	if !inCells(r.X, r.Z) || !inSpan(r.W, r.D) || !area.contains(r.rect()) {
		return fmt.Errorf("room %s is not inside its level", r.ID)
	}
	if err := text("room "+r.ID+" label", r.Label, MaxLabel); err != nil {
		return err
	}
	if r.Floor != "" && !colourRe.MatchString(r.Floor) {
		return fmt.Errorf("room %s floor is not a #rrggbb colour", r.ID)
	}
	if r.Account != "" {
		if r.Type != "cabins" {
			return fmt.Errorf("room %s sets an account, which only cabins rooms may", r.ID)
		}
		if !ValidAccount(r.Account) {
			return fmt.Errorf("room %s has an account that is not an account id", r.ID)
		}
	}
	return nil
}

func validateFurniture(l Layout, cat Catalog) error {
	rooms := map[string]Room{}
	for _, r := range l.Rooms {
		rooms[r.ID] = r
	}
	seen := map[string]bool{}
	taken := [2]map[[2]int]string{{}, {}}
	for _, f := range l.Furniture {
		if !idRe.MatchString(f.ID) {
			return fmt.Errorf("furniture id %q must be 1 to 32 characters of a-z, 0-9 and -", f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("duplicate furniture id %s", f.ID)
		}
		seen[f.ID] = true
		item, ok := cat[f.Item]
		if !ok {
			return fmt.Errorf("furniture %s: item %q is not in the catalog", f.ID, f.Item)
		}
		if f.Level != 0 && f.Level != 1 || !item.onLevel(f.Level) {
			return fmt.Errorf("furniture %s: item %s cannot stand on level %d", f.ID, f.Item, f.Level)
		}
		if f.Rot != 0 && f.Rot != 90 && f.Rot != 180 && f.Rot != 270 {
			return fmt.Errorf("furniture %s: rot must be 0, 90, 180 or 270", f.ID)
		}
		if !inCells(f.X, f.Z) {
			return fmt.Errorf("furniture %s is not inside its level", f.ID)
		}
		box := f.box(item)
		area, ok := l.bounds(f.Level)
		if !ok || !area.contains(box) {
			return fmt.Errorf("furniture %s is not inside its level", f.ID)
		}
		if f.Room != "" {
			if r, ok := rooms[f.Room]; !ok || r.Level != f.Level {
				return fmt.Errorf("furniture %s: room %q does not exist on level %d", f.ID, f.Room, f.Level)
			}
		}
		if item.hasTag("floor") {
			// rugs and mats lie under other pieces
			continue
		}
		for x := box.X; x < box.X+box.W; x++ {
			for z := box.Z; z < box.Z+box.D; z++ {
				if other, hit := taken[f.Level][[2]int{x, z}]; hit {
					return fmt.Errorf("furniture %s and %s overlap", f.ID, other)
				}
				taken[f.Level][[2]int{x, z}] = f.ID
			}
		}
	}
	return nil
}

// box is the footprint after rotation, anchored at x,z.
func (f Furn) box(it Item) Rect {
	w, d := it.Footprint.W, it.Footprint.D
	if f.Rot == 90 || f.Rot == 270 {
		w, d = d, w
	}
	return Rect{X: f.X, Z: f.Z, W: w, D: d}
}
