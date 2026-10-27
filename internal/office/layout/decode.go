package layout

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Decode reads a layout from json: size capped, unknown keys refused, one
// value only. strings come back cleaned and nil lists as empty ones. it does
// not validate, see Validate.
func Decode(raw []byte) (Layout, error) {
	var l Layout
	if len(raw) > MaxBytes {
		return l, fmt.Errorf("layout is larger than %d KB", MaxBytes/1024)
	}
	if err := checkTopKeys(raw); err != nil {
		return Layout{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return Layout{}, fmt.Errorf("layout is not valid: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Layout{}, errors.New("layout is not valid: more than one json value")
	}
	return normalize(l), nil
}

var (
	v1Keys = map[string]bool{"v": true, "name": true, "title": true, "grid": true, "size": true, "walls": true,
		"floorColor": true, "door": true, "deck": true, "stairs": true, "rooms": true, "furniture": true}
	v2Keys = map[string]bool{"v": true, "name": true, "title": true, "template": true, "site": true,
		"pavilions": true, "overflow": true, "eye": true}
)

// checkTopKeys refuses a top level key of the other version: the rules follow
// "v", a v1 file has no v2 keys and the other way round. anything that is not
// an object, or has no known version, is left to the full decode and Validate.
func checkTopKeys(raw []byte) error {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return nil
	}
	var v int
	if json.Unmarshal(top["v"], &v) != nil {
		return nil
	}
	allowed := v1Keys
	if v == Version {
		allowed = v2Keys
	}
	for k := range top {
		if !allowed[k] {
			return fmt.Errorf("layout is not valid: key %q does not belong in a version %d layout", k, v)
		}
	}
	return nil
}

// MarshalJSON writes a v2 layout with the v2 keys only, a v1 layout as it
// always was.
func (l Layout) MarshalJSON() ([]byte, error) {
	if l.V != Version {
		type plain Layout
		return json.Marshal(plain(l))
	}
	return json.Marshal(struct {
		V         int        `json:"v"`
		Name      string     `json:"name"`
		Title     string     `json:"title"`
		Template  string     `json:"template,omitempty"`
		Site      *Site      `json:"site"`
		Pavilions []Pavilion `json:"pavilions"`
		Overflow  []Spot     `json:"overflow"`
		Eye       *Eye       `json:"eye,omitempty"`
	}{l.V, l.Name, l.Title, l.Template, l.Site, l.Pavilions, l.Overflow, l.Eye})
}

// Encode is the json of a layout as stored: indented, one trailing newline.
func Encode(l Layout) ([]byte, error) {
	l = normalize(l)
	raw, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// normalize cleans every free text string and makes the lists non-nil. ids,
// colours and enums are not touched: Validate refuses them instead.
func normalize(l Layout) Layout {
	l.Title = clean(l.Title, 4*MaxTitle)
	if l.V == Version {
		return normalizeV2(l)
	}
	rooms := make([]Room, len(l.Rooms))
	for i, r := range l.Rooms {
		r.Label = clean(r.Label, 4*MaxLabel)
		rooms[i] = r
	}
	l.Rooms = rooms
	if l.Furniture == nil {
		l.Furniture = []Furn{}
	}
	return l
}

// clean removes escape sequences, turns control, format and line separator
// characters into spaces, collapses spaces and cuts to max characters. the
// length rules of Validate run on the cleaned text, so max is only a bound
// against huge strings.
func clean(s string, max int) string { return CleanText(s, max) }

// CleanText is the cleaning every layout text goes through, exported so a
// generator can make text that passes the layout text rule.
func CleanText(s string, max int) string {
	s = strings.ToValidUTF8(ansi.Strip(s), "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return strings.TrimSpace(s)
}

// normalizeV2 cleans the labels and makes every list of a v2 layout non-nil,
// so the page never sees null where it expects an array. the river stays null
// when there is none.
func normalizeV2(l Layout) Layout {
	pav := make([]Pavilion, len(l.Pavilions))
	for i, p := range l.Pavilions {
		p.Label = clean(p.Label, 4*MaxLabel)
		pav[i] = p
	}
	l.Pavilions = pav
	if l.Overflow == nil {
		l.Overflow = []Spot{}
	}
	if l.Site == nil {
		return l
	}
	s := *l.Site
	s.Base.Poly = orEmpty(s.Base.Poly)
	land := make([]Poly, len(s.Land))
	for i, p := range s.Land {
		land[i] = Poly{Poly: orEmpty(p.Poly)}
	}
	s.Land = land
	if s.River != nil {
		r := *s.River
		r.Pts = orEmpty(r.Pts)
		s.River = &r
	}
	paths := make([]Path, len(s.Paths))
	for i, p := range s.Paths {
		p.Pts = orEmpty(p.Pts)
		paths[i] = p
	}
	s.Paths = paths
	if s.Ponds == nil {
		s.Ponds = []Pond{}
	}
	if s.Bridges == nil {
		s.Bridges = []Bridge{}
	}
	if s.Props == nil {
		s.Props = []Prop{}
	}
	l.Site = &s
	return l
}

func orEmpty(p []Pt) []Pt {
	if p == nil {
		return []Pt{}
	}
	return p
}
