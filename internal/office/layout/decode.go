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
func clean(s string, max int) string {
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
