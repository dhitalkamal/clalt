package layout

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sync"
)

// Item is one placeable thing of the catalog.
type Item struct {
	ID        string   `json:"id"`
	Footprint Size     `json:"footprint"`
	Levels    []int    `json:"levels"`
	Tags      []string `json:"tags"`
	// label and category are for the editor's item panel only
	Label    string `json:"label"`
	Category string `json:"category"`
}

// Catalog maps an item id to its entry.
type Catalog map[string]Item

var idRe = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// ParseCatalog reads catalog.json: a list of items with a footprint in cells.
func ParseCatalog(raw []byte) (Catalog, error) {
	var items []Item
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	cat := Catalog{}
	for _, it := range items {
		if !idRe.MatchString(it.ID) {
			return nil, fmt.Errorf("catalog: bad item id %q", it.ID)
		}
		if _, dup := cat[it.ID]; dup {
			return nil, fmt.Errorf("catalog: duplicate item %q", it.ID)
		}
		if !inSpan(it.Footprint.W, it.Footprint.D) {
			return nil, fmt.Errorf("catalog: item %q has no footprint", it.ID)
		}
		cat[it.ID] = it
	}
	return cat, nil
}

// onLevel says whether the item may stand on a level, no levels meaning both.
func (it Item) onLevel(level int) bool {
	if len(it.Levels) == 0 {
		return level == 0 || level == 1
	}
	for _, l := range it.Levels {
		if l == level {
			return true
		}
	}
	return false
}

func (it Item) hasTag(tag string) bool {
	for _, t := range it.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

var (
	embeddedOnce sync.Once
	embeddedCat  Catalog
	embeddedErr  error
)

// EmbeddedCatalog is the catalog inside the binary, read on first use. when
// the file is missing or bad the error is kept and the catalog is empty, so
// every layout with furniture is refused instead of the binary failing to run.
func EmbeddedCatalog() (Catalog, error) {
	embeddedOnce.Do(func() {
		raw, err := fs.ReadFile(embeddedFS, "data/catalog.json")
		if errors.Is(err, fs.ErrNotExist) {
			embeddedCat, embeddedErr = Catalog{}, errors.New("catalog: not built into this binary")
			return
		}
		if err != nil {
			embeddedCat, embeddedErr = Catalog{}, err
			return
		}
		embeddedCat, embeddedErr = ParseCatalog(raw)
		if embeddedErr != nil {
			embeddedCat = Catalog{}
		}
	})
	return embeddedCat, embeddedErr
}
