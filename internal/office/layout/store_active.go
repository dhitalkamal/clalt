package layout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// activeFile is the shape of active.json.
type activeFile struct {
	Name string `json:"name"`
}

// ActiveName is the name in active.json, or the default one when the file is
// missing or bad (a bad one is logged).
func (s *Store) ActiveName() string {
	if s.Root == "" {
		return DefaultName
	}
	raw, err := fsutil.ReadFile(filepath.Join(s.Root, "active.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultName
	}
	if err != nil {
		s.logf("active layout unreadable: %v", err)
		return DefaultName
	}
	var a activeFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil || !ValidName(a.Name) {
		s.logf("active.json is not valid, using the default layout")
		return DefaultName
	}
	return a.Name
}

// Active is the layout to show. it never fails: a missing or invalid one is
// logged and the default is used.
func (s *Store) Active() Layout {
	name := s.ActiveName()
	l, err := s.Load(name)
	if err == nil {
		return l
	}
	s.logf("active layout %s is not usable (%v), using the default", name, err)
	if d, err := s.Load(DefaultName); err == nil {
		return d
	}
	return Minimal()
}

// SetActive makes a layout the active one. it must load.
func (s *Store) SetActive(name string) error {
	if _, err := s.Load(name); err != nil {
		return err
	}
	if s.Root == "" {
		return errors.New("no layouts folder")
	}
	raw, err := json.Marshal(activeFile{Name: name})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(s.Root, "active.json"), append(raw, '\n'), 0o600)
}

// Default is the layout shown when nothing is active.
func (s *Store) Default() Layout {
	if d, err := s.Load(DefaultName); err == nil {
		return d
	}
	return Minimal()
}

// Minimal is the layout of last resort: a lobby and nothing else. it needs no
// catalog and never fails validation.
func Minimal() Layout {
	return Layout{
		V: Version, Name: DefaultName, Title: "Default office", Grid: GridSize,
		Size: Size{W: 20, D: 12}, Walls: Walls{Color: "#e9e4dc"}, FloorColor: "#d9b88f",
		Door:      Door{Side: "front", At: 2},
		Rooms:     []Room{{ID: "lobby", Type: "lobby", W: 8, D: 6, Label: "Lobby"}},
		Furniture: []Furn{},
	}
}

// Check validates l against the store's catalog.
func (s *Store) Check(l Layout) error { return Validate(l, s.catalog()) }
