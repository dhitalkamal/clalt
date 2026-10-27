package layout

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"sync"
)

// MaxDesks is the most desk columns or rows one pavilion has.
const MaxDesks = 8

// footprint is one entry of data/footprints.json: either a fixed w and d, or
// a desk cell size (col and row) with padding for desk pavilions.
type footprint struct {
	W   float64 `json:"w"`
	D   float64 `json:"d"`
	Col float64 `json:"col"`
	Row float64 `json:"row"`
	Pad float64 `json:"pad"`
}

var (
	footprintsOnce sync.Once
	footprints     map[string]footprint
)

// loadFootprints reads the embedded table once. a broken table is a build
// bug, so it panics instead of letting every pavilion through.
func loadFootprints() map[string]footprint {
	footprintsOnce.Do(func() {
		raw, err := fs.ReadFile(embeddedFS, "data/footprints.json")
		if err != nil {
			panic("layout: footprints.json missing from the binary: " + err.Error())
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&footprints); err != nil {
			panic("layout: footprints.json is not valid: " + err.Error())
		}
	})
	return footprints
}

// IsPavilionType says whether t is a pavilion type of the table.
func IsPavilionType(t string) bool {
	_, ok := loadFootprints()[t]
	return ok
}

// HasDesks says whether a pavilion type is sized by cols and rows.
func HasDesks(t string) bool { return loadFootprints()[t].Col > 0 }

// Footprint is the width and depth of a pavilion before rotation. cols and
// rows are for desk pavilions (1 to MaxDesks) and must be 0 for the others.
// ok is false for an unknown type or cols and rows that do not fit the type.
func Footprint(t string, cols, rows int) (w, d float64, ok bool) {
	f, found := loadFootprints()[t]
	if !found {
		return 0, 0, false
	}
	if f.Col > 0 {
		if cols < 1 || cols > MaxDesks || rows < 1 || rows > MaxDesks {
			return 0, 0, false
		}
		return f.Col*float64(cols) + f.Pad, f.Row*float64(rows) + f.Pad, true
	}
	if cols != 0 || rows != 0 {
		return 0, 0, false
	}
	return f.W, f.D, true
}
