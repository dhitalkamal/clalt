package layout

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	colourRe  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	accountRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// reserved are Windows device names, which cannot be file names there.
var reserved = regexp.MustCompile(`^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

// ValidName says whether s can name a layout. a name is never a path and never
// a Windows device name.
func ValidName(s string) bool { return nameRe.MatchString(s) && !reserved.MatchString(s) }

// Validate checks every rule of the format against the catalog. it returns
// the first problem, written for the person who made the layout.
func Validate(l Layout, cat Catalog) error {
	switch l.V {
	case Version:
		return validateV2(l)
	case V1:
	default:
		return fmt.Errorf("unsupported layout version %d, this clalt reads versions %d and %d", l.V, V1, Version)
	}
	if l.Template != "" || l.Site != nil || len(l.Pavilions) > 0 || len(l.Overflow) > 0 || l.Eye != nil {
		return errors.New("a version 1 layout cannot hold version 2 fields (template, site, pavilions, overflow, eye)")
	}
	if err := validateHead(l); err != nil {
		return err
	}
	if len(l.Rooms) > MaxRooms {
		return fmt.Errorf("too many rooms (at most %d)", MaxRooms)
	}
	if len(l.Furniture) > MaxFurniture {
		return fmt.Errorf("too many furniture pieces (at most %d)", MaxFurniture)
	}
	if err := validateRooms(l); err != nil {
		return err
	}
	return validateFurniture(l, cat)
}

// validateHead is everything above the room list.
func validateHead(l Layout) error {
	switch {
	case !ValidName(l.Name):
		return errors.New("name must be 1 to 40 characters of a-z, 0-9 and -, and not a reserved device name such as con or nul")
	case l.Grid != GridSize:
		return fmt.Errorf("grid must be %v", GridSize)
	case l.Size.W < MinSide || l.Size.W > MaxSide || l.Size.D < MinSide || l.Size.D > MaxSide:
		return fmt.Errorf("size must be %d to %d cells each way", MinSide, MaxSide)
	}
	if err := text("title", l.Title, MaxTitle); err != nil {
		return err
	}
	for what, c := range map[string]string{"walls.color": l.Walls.Color, "floorColor": l.FloorColor} {
		if !colourRe.MatchString(c) {
			return fmt.Errorf("%s is not a #rrggbb colour", what)
		}
	}
	if err := validateDoor(l); err != nil {
		return err
	}
	return validateDeck(l)
}

func validateDoor(l Layout) error {
	var along int
	switch l.Door.Side {
	case "left", "right":
		along = l.Size.D
	case "front", "back":
		along = l.Size.W
	default:
		return errors.New("door side must be left, right, back or front")
	}
	if l.Door.At < 0 || l.Door.At >= along {
		return fmt.Errorf("door is not on its side (at must be 0 to %d)", along-1)
	}
	return nil
}

func validateDeck(l Layout) error {
	if l.Deck == nil {
		if l.Stairs != nil {
			return errors.New("stairs are set but there is no deck")
		}
		return nil
	}
	if !inCells(l.Deck.X, l.Deck.Z) || !inSpan(l.Deck.W, l.Deck.D) || !l.floor().contains(*l.Deck) {
		return errors.New("deck is not inside the floor")
	}
	if l.Stairs == nil {
		return errors.New("a deck needs stairs")
	}
	s := l.Stairs
	if s.Dir != "n" && s.Dir != "s" && s.Dir != "e" && s.Dir != "w" {
		return errors.New("stairs dir must be n, s, e or w")
	}
	if !inCells(s.X, s.Z) {
		return errors.New("stairs are not inside the floor")
	}
	flight, landing := s.rects()
	if !l.floor().contains(flight) || !l.floor().contains(landing) {
		return errors.New("stairs (flight and landing) are not inside the floor")
	}
	return nil
}

// text checks a free text string: already clean, no url, short enough.
func text(what, s string, max int) error {
	if clean(s, 4*max) != s {
		return fmt.Errorf("%s has control characters or odd spacing", what)
	}
	low := strings.ToLower(s)
	for _, bad := range []string{"://", "www.", "javascript:", "data:", "vbscript:"} {
		if strings.Contains(low, bad) {
			return fmt.Errorf("%s looks like a url, which a layout never holds", what)
		}
	}
	if strings.HasPrefix(s, "//") {
		return fmt.Errorf("%s looks like a url, which a layout never holds", what)
	}
	if utf8.RuneCountInString(s) > max {
		return fmt.Errorf("%s is longer than %d characters", what, max)
	}
	return nil
}

// CheckLabel reports whether s is a label Validate accepts as is.
func CheckLabel(s string) error { return text("label", s, MaxLabel) }
