package layout

import (
	"errors"
	"fmt"
	"math"
	"regexp"
)

// limits of the v2 format. every number is finite and inside +-MaxCoord, a
// rotation inside +-MaxRot degrees.
const (
	MaxCoord     = 200.0
	MaxRot       = 360.0
	MaxPavilions = 64
	MaxMembers   = 64
	maxBasePts   = 512
	maxLine      = 256
	maxLand      = 16
	maxPonds     = 8
	maxPaths     = 160
	maxBridges   = 16
	maxProps     = 512
	maxOverflow  = 64
	maxSeed      = 1 << 30
)

// templateIDs of the generators. a v2 layout may name one of them, see Template.
var gardenTemplates = map[string]bool{"rows": true, "village": true, "river": true, "street": true}

var (
	propTypes = map[string]bool{"bench": true, "bed": true, "lamp": true, "fountain": true, "standing-table": true, "sign": true}
	nearTypes = map[string]bool{"workspace": true, "cafe": true, "play": true, "lobby": true}
)

// memberRe is an agent id on a team pavilion: the lowercase type, a plugin
// prefix included.
var memberRe = regexp.MustCompile(`^[a-z0-9:-]{1,64}$`)

// num says whether v is a finite number inside +-lim. the range test runs
// before any arithmetic on the value.
func num(what string, v, lim float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s is not a finite number", what)
	}
	if v < -lim || v > lim {
		return fmt.Errorf("%s is out of range (+-%v)", what, lim)
	}
	return nil
}

// between is num plus a minimum and maximum of its own.
func between(what string, v, lo, hi float64) error {
	if err := num(what, v, MaxCoord); err != nil {
		return err
	}
	if v < lo || v > hi {
		return fmt.Errorf("%s must be %v to %v", what, lo, hi)
	}
	return nil
}

func points(what string, pts []Pt, min, max int) error {
	if len(pts) < min || len(pts) > max {
		return fmt.Errorf("%s needs %d to %d points", what, min, max)
	}
	for _, p := range pts {
		if err := pt(what, p); err != nil {
			return err
		}
	}
	return nil
}

func pt(what string, p Pt) error {
	if err := num(what, p[0], MaxCoord); err != nil {
		return err
	}
	return num(what, p[1], MaxCoord)
}

// validateV2 checks a garden layout. the first problem is returned.
func validateV2(l Layout) error {
	if l.Grid != 0 || l.Size != (Size{}) || l.Walls != (Walls{}) || l.FloorColor != "" || l.Door != (Door{}) ||
		l.Deck != nil || l.Stairs != nil || len(l.Rooms) > 0 || len(l.Furniture) > 0 {
		return errors.New("a version 2 layout cannot hold version 1 fields (grid, size, walls, rooms, furniture and so on)")
	}
	if !ValidName(l.Name) {
		return errors.New("name must be 1 to 40 characters of a-z, 0-9 and -, and not a reserved device name such as con or nul")
	}
	if err := text("title", l.Title, MaxTitle); err != nil {
		return err
	}
	if l.Template != "" && !gardenTemplates[l.Template] {
		return fmt.Errorf("template %q is not a garden template", l.Template)
	}
	if l.Site == nil {
		return errors.New("a version 2 layout needs a site")
	}
	if err := validateSite(*l.Site); err != nil {
		return err
	}
	if err := validatePavilions(l); err != nil {
		return err
	}
	if err := validateOverflow(l.Overflow); err != nil {
		return err
	}
	if l.Eye != nil {
		if err := pt("eye from", l.Eye.From); err != nil {
			return err
		}
		return pt("eye look", l.Eye.Look)
	}
	return nil
}

func validateSite(s Site) error {
	if err := points("base outline", s.Base.Poly, 3, maxBasePts); err != nil {
		return err
	}
	if len(s.Land) < 1 || len(s.Land) > maxLand {
		return fmt.Errorf("land needs 1 to %d areas", maxLand)
	}
	for _, p := range s.Land {
		if err := points("land area", p.Poly, 3, maxBasePts); err != nil {
			return err
		}
	}
	if s.River != nil {
		if err := points("river", s.River.Pts, 2, maxLine); err != nil {
			return err
		}
		if err := between("river width", s.River.W, 0.5, 8); err != nil {
			return err
		}
	}
	if err := validateWater(s); err != nil {
		return err
	}
	if err := validateProps(s.Props); err != nil {
		return err
	}
	if s.Trees.Mode != "edge" && s.Trees.Mode != "scatter" {
		return errors.New("trees mode must be edge or scatter")
	}
	if err := between("trees band", s.Trees.Band, 0, 20); err != nil {
		return err
	}
	if s.Trees.Seed < 0 || s.Trees.Seed > maxSeed {
		return fmt.Errorf("trees seed must be 0 to %d", maxSeed)
	}
	return nil
}

// validateWater is ponds, paths and bridges.
func validateWater(s Site) error {
	if len(s.Ponds) > maxPonds {
		return fmt.Errorf("too many ponds (at most %d)", maxPonds)
	}
	for _, p := range s.Ponds {
		if err := num("pond", p.X, MaxCoord); err != nil {
			return err
		}
		if err := num("pond", p.Z, MaxCoord); err != nil {
			return err
		}
		if err := between("pond radius", p.RX, 0.5, 10); err != nil {
			return err
		}
		if err := between("pond radius", p.RZ, 0.5, 10); err != nil {
			return err
		}
	}
	if len(s.Paths) > maxPaths {
		return fmt.Errorf("too many paths (at most %d)", maxPaths)
	}
	for _, p := range s.Paths {
		if err := points("path", p.Pts, 2, maxLine); err != nil {
			return err
		}
		if err := between("path width", p.W, 0.3, 4); err != nil {
			return err
		}
	}
	if len(s.Bridges) > maxBridges {
		return fmt.Errorf("too many bridges (at most %d)", maxBridges)
	}
	for _, b := range s.Bridges {
		if err := pt("bridge", b.A); err != nil {
			return err
		}
		if err := pt("bridge", b.B); err != nil {
			return err
		}
		if n := math.Hypot(b.B[0]-b.A[0], b.B[1]-b.A[1]); n < 1 || n > 12 {
			return errors.New("a bridge must be 1 to 12 long")
		}
		if err := between("bridge width", b.W, 0.3, 4); err != nil {
			return err
		}
	}
	return nil
}

func validateProps(props []Prop) error {
	if len(props) > maxProps {
		return fmt.Errorf("too many props (at most %d)", maxProps)
	}
	for _, p := range props {
		if !propTypes[p.T] {
			return fmt.Errorf("prop type %q is not known", p.T)
		}
		if err := num("prop", p.X, MaxCoord); err != nil {
			return err
		}
		if err := num("prop", p.Z, MaxCoord); err != nil {
			return err
		}
		if err := num("prop rot", p.Rot, MaxRot); err != nil {
			return err
		}
		if p.T != "bed" {
			if p.W != 0 || p.D != 0 {
				return fmt.Errorf("prop %s has no size, only a bed does", p.T)
			}
			continue
		}
		if err := between("prop bed width", p.W, 0.3, 10); err != nil {
			return err
		}
		if err := between("prop bed depth", p.D, 0.3, 10); err != nil {
			return err
		}
	}
	return nil
}

func validateOverflow(spots []Spot) error {
	if len(spots) > maxOverflow {
		return fmt.Errorf("too many overflow spots (at most %d)", maxOverflow)
	}
	for _, s := range spots {
		if !nearTypes[s.Near] {
			return errors.New("overflow near must be workspace, cafe, play or lobby")
		}
		if err := num("overflow spot", s.X, MaxCoord); err != nil {
			return err
		}
		if err := num("overflow spot", s.Z, MaxCoord); err != nil {
			return err
		}
		if err := num("overflow rot", s.Rot, MaxRot); err != nil {
			return err
		}
	}
	return nil
}

// validatePavilions checks each pavilion alone, then the counts, then the
// placement against the base, each other, the river and the ponds.
func validatePavilions(l Layout) error {
	if len(l.Pavilions) > MaxPavilions {
		return fmt.Errorf("too many pavilions (at most %d)", MaxPavilions)
	}
	seen := map[string]bool{}
	lobbies, treehouses := 0, 0
	boxes := make([]box, len(l.Pavilions))
	for i, p := range l.Pavilions {
		if !idRe.MatchString(p.ID) {
			return fmt.Errorf("pavilion id %q must be 1 to 32 characters of a-z, 0-9 and -", p.ID)
		}
		if seen[p.ID] {
			return fmt.Errorf("duplicate pavilion id %q", p.ID)
		}
		seen[p.ID] = true
		if !IsPavilionType(p.Type) {
			return fmt.Errorf("pavilion %s has an unknown type %q", p.ID, p.Type)
		}
		if err := validatePavilionFields(p); err != nil {
			return err
		}
		switch p.Type {
		case "lobby":
			lobbies++
		case "treehouse":
			treehouses++
		}
		w, d, _ := Footprint(p.Type, p.Cols, p.Rows)
		boxes[i] = newBox(p.X, p.Z, w, d, p.Rot)
	}
	if lobbies != 1 {
		return errors.New("a layout needs exactly one lobby")
	}
	if treehouses > 1 {
		return errors.New("a layout has at most one treehouse")
	}
	return placePavilions(l, boxes)
}

func validatePavilionFields(p Pavilion) error {
	for _, v := range []float64{p.X, p.Z} {
		if err := num("pavilion "+p.ID, v, MaxCoord); err != nil {
			return err
		}
	}
	if err := num("pavilion "+p.ID+" rot", p.Rot, MaxRot); err != nil {
		return err
	}
	if err := text("pavilion label", p.Label, MaxLabel); err != nil {
		return err
	}
	if p.Type == "cabin" {
		if !ValidAccount(p.Account) {
			return fmt.Errorf("cabin %s needs an account id", p.ID)
		}
	} else if p.Account != "" {
		return fmt.Errorf("pavilion %s is not a cabin and cannot name an account", p.ID)
	}
	if err := validateMembers(p); err != nil {
		return err
	}
	if _, _, ok := Footprint(p.Type, p.Cols, p.Rows); !ok {
		if HasDesks(p.Type) {
			return fmt.Errorf("pavilion %s needs cols and rows of 1 to %d", p.ID, MaxDesks)
		}
		return fmt.Errorf("pavilion %s (%s) has no cols or rows", p.ID, p.Type)
	}
	return nil
}

// validateMembers checks the agent ids of a team pavilion: only team
// pavilions have them, one per desk at most, each id seen once.
func validateMembers(p Pavilion) error {
	if len(p.Members) == 0 {
		return nil
	}
	if p.Type != "team" {
		return fmt.Errorf("pavilion %s is not a team and cannot list members", p.ID)
	}
	if len(p.Members) > MaxMembers {
		return fmt.Errorf("pavilion %s has too many members (at most %d)", p.ID, MaxMembers)
	}
	if len(p.Members) > p.Cols*p.Rows*2 {
		return fmt.Errorf("pavilion %s has more members than desks (%d)", p.ID, p.Cols*p.Rows*2)
	}
	seen := map[string]bool{}
	for _, m := range p.Members {
		if !memberRe.MatchString(m) {
			return fmt.Errorf("pavilion %s member %q must be 1 to 64 characters of a-z, 0-9, - and :", p.ID, m)
		}
		if seen[m] {
			return fmt.Errorf("pavilion %s lists member %q twice (duplicate)", p.ID, m)
		}
		seen[m] = true
	}
	return nil
}

func placePavilions(l Layout, boxes []box) error {
	for i, b := range boxes {
		id := l.Pavilions[i].ID
		for _, c := range b.corners() {
			if !inPoly(c, l.Site.Base.Poly) {
				return fmt.Errorf("pavilion %s is not inside the base outline", id)
			}
		}
		for j := 0; j < i; j++ {
			if b.overlaps(boxes[j]) {
				return fmt.Errorf("pavilions %s and %s overlap", l.Pavilions[j].ID, id)
			}
		}
		if r := l.Site.River; r != nil && b.hitsBand(r.Pts, r.W) {
			return fmt.Errorf("pavilion %s stands in the river", id)
		}
		for _, p := range l.Site.Ponds {
			if b.hitsPond(p) {
				return fmt.Errorf("pavilion %s stands in a pond", id)
			}
		}
	}
	return nil
}
